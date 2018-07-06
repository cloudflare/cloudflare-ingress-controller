package tunnel

import (
	"fmt"
	"net"
	"time"

	"github.com/cloudflare/cloudflared/metrics"
	"github.com/cloudflare/cloudflared/origin"
	"github.com/sirupsen/logrus"
)

const (
	MetricsAppKey      = "application"
	MetricsServiceKey  = "origin_service"
	MetricsHostnameKey = "hostname"
)

// MetricsConfig wraps the argo tunnel metrics in a struct
type MetricsConfig struct {
	Metrics          *origin.TunnelMetrics
	UpdateFrequency  time.Duration
	MetricsLabelKeys []string
}

// NewMetrics created a set of TunnelMetrics,
// allows global prometheus objects, which breaks tests
func NewMetrics(metricsLabelKeys []string) *MetricsConfig {

	return &MetricsConfig{
		Metrics:          origin.NewTunnelMetrics(metricsLabelKeys),
		UpdateFrequency:  5 * time.Second,
		MetricsLabelKeys: metricsLabelKeys,
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
	return metrics.ServeMetrics(metricsListener, shutdownC, logger)
}
