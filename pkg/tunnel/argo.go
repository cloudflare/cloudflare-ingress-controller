package tunnel

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/cloudflare/cloudflare-ingress-controller/pkg/cloudflare"
	"github.com/cloudflare/cloudflare-ingress-controller/pkg/version"
	"github.com/cloudflare/cloudflared/origin"
	tunnelpogs "github.com/cloudflare/cloudflared/tunnelrpc/pogs"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	log "github.com/sirupsen/logrus"
)

// ArgoTunnelManager manages a single tunnel in a goroutine
type ArgoTunnelManager struct {
	id           string
	config       *Config
	tunnelConfig *origin.TunnelConfig
	errCh        chan error
	stopCh       chan struct{}
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

// NewArgoTunnelManager is a wrapper around a argo tunnel running in a goroutine
func NewArgoTunnelManager(config *Config, metricsSetup *MetricsConfig) (Tunnel, error) {

	haConnections := 1

	protocolLogger := log.New()

	tunnelLogger := log.WithFields(log.Fields{
		"service": config.ServiceName,
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
		ReportedVersion:   version.VERSION,
		LBPool:            config.LBPool,
		Tags:              []tunnelpogs.Tag{},
		HAConnections:     haConnections,
		HTTPTransport:     httpTransport,
		Metrics:           metricsSetup.Metrics,
		MetricsUpdateFreq: metricsSetup.UpdateFrequency,
		ProtocolLogger:    protocolLogger,
		Logger:            tunnelLogger,
		IsAutoupdated:     false,
		GracePeriod:       0,     //time.Duration
		RunFromTerminal:   false, // bool

	}

	mgr := ArgoTunnelManager{
		id:           utilrand.String(8),
		config:       config,
		tunnelConfig: &tunnelConfig,
		errCh:        make(chan error),
		stopCh:       nil,
	}
	return &mgr, nil
}

func (mgr *ArgoTunnelManager) Config() Config {
	return *mgr.config
}

func (mgr *ArgoTunnelManager) Active() bool {
	return mgr.stopCh != nil
}

func (mgr *ArgoTunnelManager) Start(serviceURL string) error {

	if serviceURL == "" {
		return fmt.Errorf("Cannot start tunnel for %s with empty url", mgr.Config().ServiceName)
	}
	mgr.tunnelConfig.OriginUrl = serviceURL

	placeHolderOnlyConnectedSignal := make(chan struct{})
	mgr.stopCh = make(chan struct{})

	go func() {
		mgr.errCh <- origin.StartTunnelDaemon(mgr.tunnelConfig, mgr.stopCh, placeHolderOnlyConnectedSignal)
	}()
	return nil
}

func (mgr *ArgoTunnelManager) Stop() error {
	if mgr.stopCh == nil {
		return fmt.Errorf("tunnel %s already stopped", mgr.id)
	}
	close(mgr.stopCh)
	mgr.tunnelConfig.OriginUrl = ""
	mgr.stopCh = nil
	<-mgr.errCh // muxerShutdownError is not an error
	return nil
}

func (mgr *ArgoTunnelManager) TearDown() error {
	return mgr.Stop()
}

func (mgr *ArgoTunnelManager) CheckStatus() error {
	return fmt.Errorf("Not implemented")
}
