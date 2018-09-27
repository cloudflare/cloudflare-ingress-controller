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

	mgr := ArgoTunnel{
		id:           utilrand.String(8),
		origin:       source,
		config:       config,
		tunnelConfig: &tunnelConfig,
		errCh:        make(chan error),
		stopCh:       nil,
		quitCh:       nil,
	}
	return &mgr, nil
}

func (mgr *ArgoTunnel) Config() Config {
	return *mgr.config
}

func (mgr *ArgoTunnel) Active() bool {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	return mgr.stopCh != nil
}

func (mgr *ArgoTunnel) Start(serviceURL string) error {
	if serviceURL == "" {
		return fmt.Errorf("Cannot start tunnel for %s with empty url", mgr.Config().ServiceName)
	} else if mgr.stopCh != nil {
		return nil
	}

	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	if mgr.stopCh != nil {
		return nil
	}

	mgr.tunnelConfig.OriginUrl = serviceURL
	mgr.stopCh = make(chan struct{})
	mgr.quitCh = make(chan struct{})
	go repairFunc(mgr)()
	go launchFunc(mgr)()
	return nil
}

func (mgr *ArgoTunnel) Stop() error {
	if mgr.stopCh == nil {
		return fmt.Errorf("tunnel %s (%s) already stopped", mgr.origin, mgr.id)
	}

	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	if mgr.stopCh == nil {
		return fmt.Errorf("tunnel %s (%s) already stopped", mgr.origin, mgr.id)
	}

	close(mgr.quitCh)
	close(mgr.stopCh)
	mgr.tunnelConfig.OriginUrl = ""
	mgr.quitCh = nil
	mgr.stopCh = nil
	return nil
}

func (mgr *ArgoTunnel) TearDown() error {
	return mgr.Stop()
}

func (mgr *ArgoTunnel) CheckStatus() error {
	return fmt.Errorf("Not implemented")
}

func launchFunc(atm *ArgoTunnelManager) func() {
	errCh := atm.errCh
	stopCh := atm.stopCh
	config := atm.tunnelConfig
	return func() {
		errCh <- origin.StartTunnelDaemon(config, stopCh, make(chan struct{}))
	}
}

func repairFunc(atm *ArgoTunnelManager) func() {
	mgr := atm
	errCh := mgr.errCh
	quitCh := mgr.quitCh
	origin := mgr.origin
	config := mgr.config
	logger := mgr.tunnelConfig.Logger
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

						if mgr.stopCh == nil {
							logger.WithFields(log.Fields{
								"origin":   origin,
								"hostname": config.ExternalHostname,
							}).Infof("tunnel repair canceled, stop detected.")
							return
						}

						mgr.mu.Lock()
						defer mgr.mu.Unlock()
						if mgr.stopCh == nil {
							logger.WithFields(log.Fields{
								"origin":   origin,
								"hostname": config.ExternalHostname,
							}).Infof("tunnel repair canceled, stop detected.")
							return
						}

						close(mgr.stopCh)
						mgr.stopCh = make(chan struct{})
						go launchFunc(mgr)()
					}()
				}
			}
		}
	}
}
