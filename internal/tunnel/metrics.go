package tunnel

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/cloudflare/cloudflare-ingress-controller/pkg/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	// "github.com/cloudflare/cloudflared/metrics"
	"github.com/cloudflare/cloudflared/origin"
	"github.com/sirupsen/logrus"
)

const (
	AppKey      = "application"
	ServiceKey  = "origin_service"
	HostnameKey = "hostname"
)

// MetricsConfig wraps the argo tunnel metrics in a struct
type MetricsConfig struct {
	Metrics         *origin.TunnelMetrics
	UpdateFrequency time.Duration
}

// NewMetrics created a set of TunnelMetrics,
// with a fixed set of additional prometheus label keys
func NewMetrics(orderedLabelKeys []string) *MetricsConfig {

	return &MetricsConfig{
		Metrics:         origin.InitializeTunnelMetrics(orderedLabelKeys),
		UpdateFrequency: 5 * time.Second,
	}
}

// NewDummyMetrics creates a sample TunnelMetrics object
// full of default zero values
// does not initializs prometheus and is acceptable for tests
func NewDummyMetrics() *MetricsConfig {

	return &MetricsConfig{
		Metrics:         &origin.TunnelMetrics{},
		UpdateFrequency: 10000 * time.Hour,
	}
}

func ServeMetrics(port int, shutdownC <-chan struct{}, logger *logrus.Logger) (err error) {

	metricsListener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}

	var wg sync.WaitGroup

	server := &http.Server{
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	metricsPath := "/metrics"
	http.Handle(metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(`<html>
             <head><title>Argot Metrics</title></head>
             <body>
             <h1>Argo Ingress Controller</h1>
             <p><a href='` + metricsPath + `'>Metrics</a></p>
             <h2>Build</h2>
             <pre>` + version.VERSION + `</pre>
             </body>
             </html>`))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	wg.Add(1)
	go func() {
		defer wg.Done()
		err = server.Serve(metricsListener)
	}()
	logger.WithField("addr", metricsListener.Addr()).Info("Starting metrics server")

	startupDelayTime := time.Millisecond * 500
	time.Sleep(startupDelayTime)

	<-shutdownC
	shutdownTimeout := time.Second * 15
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	server.Shutdown(ctx)
	cancel()

	wg.Wait()
	if err == http.ErrServerClosed {
		logger.Info("Metrics server stopped")
		return nil
	}
	logger.WithError(err).Error("Metrics server quit with error")
	return err
}
