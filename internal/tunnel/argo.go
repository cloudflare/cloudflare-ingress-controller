package tunnel

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/cloudflare/cloudflare-ingress-controller/internal/cloudflare"
	"github.com/cloudflare/cloudflared/origin"
	tunnelpogs "github.com/cloudflare/cloudflared/tunnelrpc/pogs"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"

	log "github.com/sirupsen/logrus"
)

const (
	repairDelay  = 50 * time.Millisecond
	repairJitter = 1.0
)

// ArgoTunnel manages a single tunnel in a goroutine
type ArgoTunnel struct {
	id           string
	origin       string
	route        Route
	options      Options
	tunnelConfig *origin.TunnelConfig
	errCh        chan error
	stopCh       chan struct{}
	quitCh       chan struct{}
	mu           sync.RWMutex
}

func newHttpTransport() *http.Transport {
	tlsConfig := &tls.Config{}

	httpTransport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   time.Second * 30,
			KeepAlive: time.Second * 30,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       time.Second * 90,
		TLSHandshakeTimeout:   time.Second * 10,
		ExpectContinueTimeout: time.Second * 1,
		TLSClientConfig:       tlsConfig,
	}

	return httpTransport
}

// NewArgoTunnel is a wrapper around a argo tunnel running in a goroutine
func NewArgoTunnel(route Route, options ...Option) (Tunnel, error) {
	opts := CollectOptions(options)
	protocolLogger := log.New()

	source := route.Namespace + "/" + route.ServiceName + "/" + route.ServicePort.String()
	tunnelLogger := log.WithFields(log.Fields{
		"origin":   source,
		"hostname": route.ExternalHostname,
	}).Logger

	httpTransport := newHttpTransport()
	tlsConfig := &tls.Config{
		RootCAs:    cloudflare.GetCloudflareRootCA(),
		ServerName: "cftunnel.com",
	}

	tunnelConfig := origin.TunnelConfig{
		EdgeAddrs:          []string{}, // load default values later, see github.com/cloudflare/cloudflared/blob/master/origin/discovery.go#
		OriginUrl:          "",
		Hostname:           route.ExternalHostname,
		OriginCert:         route.OriginCert, // []byte{}
		TlsConfig:          tlsConfig,
		ClientTlsConfig:    httpTransport.TLSClientConfig, // *tls.Config
		Retries:            opts.Retries,
		HeartbeatInterval:  opts.HeartbeatInterval,
		MaxHeartbeats:      opts.HeartbeatCount,
		ClientID:           utilrand.String(16),
		BuildInfo:          origin.GetBuildInfo(),
		ReportedVersion:    route.Version,
		LBPool:             opts.LbPool,
		Tags:               []tunnelpogs.Tag{},
		HAConnections:      opts.HaConnections,
		HTTPTransport:      httpTransport,
		Metrics:            metricsConfig.metrics,
		MetricsUpdateFreq:  metricsConfig.updateFrequency,
		ProtocolLogger:     protocolLogger,
		Logger:             tunnelLogger,
		IsAutoupdated:      false,
		GracePeriod:        opts.GracePeriod,
		RunFromTerminal:    false, // bool
		NoChunkedEncoding:  opts.NoChunkedEncoding,
		CompressionQuality: opts.CompressionQuality,
	}

	t := ArgoTunnel{
		id:           utilrand.String(8),
		origin:       source,
		route:        route,
		options:      opts,
		tunnelConfig: &tunnelConfig,
		errCh:        make(chan error),
		stopCh:       nil,
		quitCh:       nil,
	}
	return &t, nil
}

// Route returns the tunnel configuration
func (t *ArgoTunnel) Route() Route {
	return t.route
}

// Options returns the tunnel options
func (t *ArgoTunnel) Options() Options {
	return t.options
}

// Active tells whether the tunnel is active or not
func (t *ArgoTunnel) Active() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.stopCh != nil
}

// Start the tunnel to connect to a particular service url, making it active
func (t *ArgoTunnel) Start(serviceURL string) error {
	if serviceURL == "" {
		return fmt.Errorf("Cannot start tunnel for %s with empty url", t.route.ServiceName)
	} else if t.stopCh != nil {
		return nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if t.stopCh != nil {
		return nil
	}

	t.tunnelConfig.OriginUrl = serviceURL
	t.stopCh = make(chan struct{})
	t.quitCh = make(chan struct{})
	go repairFunc(t)()
	go launchFunc(t)()
	return nil
}

// Stop the tunnel, making it inactive
func (t *ArgoTunnel) Stop() error {
	if t.stopCh == nil {
		return fmt.Errorf("tunnel %s (%s) already stopped", t.origin, t.id)
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if t.stopCh == nil {
		return fmt.Errorf("tunnel %s (%s) already stopped", t.origin, t.id)
	}

	close(t.quitCh)
	close(t.stopCh)
	t.tunnelConfig.OriginUrl = ""
	t.quitCh = nil
	t.stopCh = nil
	return nil
}

// TearDown cleans up all external resources
func (t *ArgoTunnel) TearDown() error {
	return t.Stop()
}

// CheckStatus validates the current state of the tunnel
func (t *ArgoTunnel) CheckStatus() error {
	return fmt.Errorf("Not implemented")
}

func launchFunc(a *ArgoTunnel) func() {
	errCh := a.errCh
	stopCh := a.stopCh
	route := a.tunnelConfig
	return func() {
		errCh <- origin.StartTunnelDaemon(route, stopCh, make(chan struct{}))
	}
}

func repairFunc(a *ArgoTunnel) func() {
	t := a
	errCh := a.errCh
	quitCh := a.quitCh
	origin := a.origin
	route := a.route
	logger := a.tunnelConfig.Logger
	return func() {
		for {
			select {
			case <-quitCh:
				return
			case err, open := <-errCh:
				if !open {
					return
				}
				if err != nil {
					func() {
						logger.WithFields(log.Fields{
							"origin":   origin,
							"hostname": route.ExternalHostname,
						}).Errorf("tunnel exited with error (%s) '%v', repairing ...", reflect.TypeOf(err), err)

						// linear back-off on runtime error
						delay := wait.Jitter(repairDelay, repairJitter)
						logger.WithFields(log.Fields{
							"origin":   origin,
							"hostname": route.ExternalHostname,
						}).Infof("tunnel repair starts in %v", delay)

						select {
						case <-quitCh:
							logger.WithFields(log.Fields{
								"origin":   origin,
								"hostname": route.ExternalHostname,
							}).Infof("tunnel repair canceled, stop detected.")
							return
						case <-time.After(delay):
						}

						if t.stopCh == nil {
							logger.WithFields(log.Fields{
								"origin":   origin,
								"hostname": route.ExternalHostname,
							}).Infof("tunnel repair canceled, stop detected.")
							return
						}

						t.mu.Lock()
						defer t.mu.Unlock()
						if t.stopCh == nil {
							logger.WithFields(log.Fields{
								"origin":   origin,
								"hostname": route.ExternalHostname,
							}).Infof("tunnel repair canceled, stop detected.")
							return
						}

						close(t.stopCh)
						t.stopCh = make(chan struct{})
						go launchFunc(t)()
					}()
				}
			}
		}
	}
}
