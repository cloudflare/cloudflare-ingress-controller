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
	haConnectionsDefault = 4
	repairDelay          = 50 * time.Millisecond
	repairJitter         = 1.0
)

// ArgoTunnel manages a single tunnel in a goroutine
type ArgoTunnel struct {
	id           string
	origin       string
	config       *Config
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
func NewArgoTunnel(config *Config, metricsSetup *MetricsConfig) (Tunnel, error) {
	protocolLogger := log.New()

	source := config.ServiceNamespace + "/" + config.ServiceName + ":" + config.ServicePort.String()
	tunnelLogger := log.WithFields(log.Fields{
		"origin":   source,
		"hostname": config.ExternalHostname,
	}).Logger

	httpTransport := newHttpTransport()
	tlsConfig := &tls.Config{
		RootCAs:    cloudflare.GetCloudflareRootCA(),
		ServerName: "cftunnel.com",
	}

	tunnelConfig := origin.TunnelConfig{
		EdgeAddrs:         []string{}, // load default values later, see github.com/cloudflare/cloudflared/blob/master/origin/discovery.go#
		OriginUrl:         "",
		Hostname:          config.ExternalHostname,
		OriginCert:        config.OriginCert, // []byte{}
		TlsConfig:         tlsConfig,
		ClientTlsConfig:   httpTransport.TLSClientConfig, // *tls.Config
		Retries:           5,
		HeartbeatInterval: 5 * time.Second,
		MaxHeartbeats:     5,
		ClientID:          utilrand.String(16),
		BuildInfo:         origin.GetBuildInfo(),
		ReportedVersion:   config.Version,
		LBPool:            config.LBPool,
		Tags:              []tunnelpogs.Tag{},
		HAConnections:     haConnectionsDefault,
		HTTPTransport:     httpTransport,
		Metrics:           metricsSetup.Metrics,
		MetricsUpdateFreq: metricsSetup.UpdateFrequency,
		ProtocolLogger:    protocolLogger,
		Logger:            tunnelLogger,
		IsAutoupdated:     false,
		GracePeriod:       0,     //time.Duration
		RunFromTerminal:   false, // bool

	}

	t := ArgoTunnel{
		id:           utilrand.String(8),
		origin:       source,
		config:       config,
		tunnelConfig: &tunnelConfig,
		errCh:        make(chan error),
		stopCh:       nil,
		quitCh:       nil,
	}
	return &t, nil
}

func (t *ArgoTunnel) Config() Config {
	return *t.config
}

func (t *ArgoTunnel) Active() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.stopCh != nil
}

func (t *ArgoTunnel) Start(serviceURL string) error {
	if serviceURL == "" {
		return fmt.Errorf("Cannot start tunnel for %s with empty url", t.config.ServiceName)
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

func (t *ArgoTunnel) TearDown() error {
	return t.Stop()
}

func (t *ArgoTunnel) CheckStatus() error {
	return fmt.Errorf("Not implemented")
}

func launchFunc(a *ArgoTunnel) func() {
	errCh := a.errCh
	stopCh := a.stopCh
	config := a.tunnelConfig
	return func() {
		errCh <- origin.StartTunnelDaemon(config, stopCh, make(chan struct{}))
	}
}

func repairFunc(a *ArgoTunnel) func() {
	t := a
	errCh := a.errCh
	quitCh := a.quitCh
	origin := a.origin
	config := a.config
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
							"hostname": config.ExternalHostname,
						}).Errorf("tunnel exited with error (%s) '%v', repairing ...", reflect.TypeOf(err), err)

						// linear back-off on runtime error
						delay := wait.Jitter(repairDelay, repairJitter)
						logger.WithFields(log.Fields{
							"origin":   origin,
							"hostname": config.ExternalHostname,
						}).Infof("tunnel repair starts in %v", delay)

						select {
						case <-quitCh:
							logger.WithFields(log.Fields{
								"origin":   origin,
								"hostname": config.ExternalHostname,
							}).Infof("tunnel repair canceled, stop detected.")
							return
						case <-time.After(delay):
						}

						if t.stopCh == nil {
							logger.WithFields(log.Fields{
								"origin":   origin,
								"hostname": config.ExternalHostname,
							}).Infof("tunnel repair canceled, stop detected.")
							return
						}

						t.mu.Lock()
						defer t.mu.Unlock()
						if t.stopCh == nil {
							logger.WithFields(log.Fields{
								"origin":   origin,
								"hostname": config.ExternalHostname,
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
