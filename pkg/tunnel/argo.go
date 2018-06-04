package tunnel

import (
	"crypto/tls"
	"fmt"
	"time"

	"github.com/cloudflare/cloudflare-ingress-controller/pkg/cloudflare"
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

// NewArgoTunnelManager is a wrapper around a argo tunnel running in a goroutine
func NewArgoTunnelManager(config *Config, metricsSetup *MetricsConfig) (Tunnel, error) {

	haConnections := 1

	protocolLogger := log.New()

	tunnelLogger := log.WithFields(log.Fields{
		"service": config.ServiceName,
	}).Logger

	tunnelConfig := origin.TunnelConfig{
		EdgeAddrs:         []string{"cftunnel.com:7844"},
		OriginUrl:         "",
		Hostname:          config.ExternalHostname,
		OriginCert:        config.OriginCert, // []byte{}
		TlsConfig:         &tls.Config{},     // need to load the cloudflare cert
		ClientTlsConfig:   nil,               // *tls.Config
		Retries:           5,
		HeartbeatInterval: 5 * time.Second,
		MaxHeartbeats:     5,
		ClientID:          utilrand.String(16),
		BuildInfo:         origin.GetBuildInfo(),
		ReportedVersion:   "DEV",
		LBPool:            config.LBPool,
		Tags:              []tunnelpogs.Tag{},
		HAConnections:     haConnections,
		HTTPTransport:     nil, // http.RoundTripper
		Metrics:           metricsSetup.Metrics,
		MetricsUpdateFreq: metricsSetup.UpdateFrequency,
		ProtocolLogger:    protocolLogger,
		Logger:            tunnelLogger,
		IsAutoupdated:     false,
		GracePeriod:       0,     //time.Duration
		RunFromTerminal:   false, // bool

	}

	tunnelConfig.TlsConfig.RootCAs = cloudflare.GetCloudflareRootCA()
	tunnelConfig.TlsConfig.ServerName = "cftunnel.com"

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

	go func() {
		err := <-mgr.errCh
		if err != nil {
			mgr.tunnelConfig.Logger.Errorf("error in starting tunnel, %v", err)
		}
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
	return nil
	// return <-mgr.errCh
}

func (mgr *ArgoTunnelManager) TearDown() error {
	return mgr.Stop()
}

func (mgr *ArgoTunnelManager) CheckStatus() error {
	return fmt.Errorf("Not implemented")
}
