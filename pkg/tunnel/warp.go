package tunnel

import (
	"crypto/tls"
	"fmt"
	"time"

	"github.com/cloudflare/cloudflare-warp-ingress/pkg/cloudflare"
	"github.com/cloudflare/cloudflare-warp/origin"
	tunnelpogs "github.com/cloudflare/cloudflare-warp/tunnelrpc/pogs"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	log "github.com/sirupsen/logrus"
)

const ()

// WarpManager manages a single tunnel in a goroutine
type WarpManager struct {
	id           string
	config       *Config
	tunnelConfig *origin.TunnelConfig
	errCh        chan error
	stopCh       chan struct{}
}

// tunnelConfig := &origin.TunnelConfig{
// 	EdgeAddr:          c.String("edge"),
// 	OriginUrl:         url,
// 	Hostname:          hostname,
// 	OriginCert:        originCert,
// 	TlsConfig:         &tls.Config{},
// 	Retries:           c.Uint("retries"),
// 	HeartbeatInterval: c.Duration("heartbeat-interval"),
// 	MaxHeartbeats:     c.Uint64("heartbeat-count"),
// 	ClientID:          clientID,
// 	ReportedVersion:   Version,
// 	LBPool:            c.String("lb-pool"),
// 	Tags:              tags,
// 	ConnectedSignal:   h2mux.NewSignal(),
// }

// NewWarpManager is a wrapper around a warp tunnel running in a goroutine
func NewWarpManager(config *Config, metricsSetup *MetricsConfig) (Tunnel, error) {

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
		Retries:           5,
		HeartbeatInterval: 5 * time.Second,
		MaxHeartbeats:     5,
		ClientID:          utilrand.String(16),
		ReportedVersion:   "DEV",
		LBPool:            "",
		Tags:              []tunnelpogs.Tag{},
		HAConnections:     haConnections,
		Metrics:           metricsSetup.Metrics,
		MetricsUpdateFreq: metricsSetup.UpdateFrequency,
		ProtocolLogger:    protocolLogger,
		Logger:            tunnelLogger,
		IsAutoupdated:     false,
	}

	tunnelConfig.TlsConfig.RootCAs = cloudflare.GetCloudflareRootCA()
	tunnelConfig.TlsConfig.ServerName = "cftunnel.com"

	mgr := WarpManager{
		id:           utilrand.String(8),
		config:       config,
		tunnelConfig: &tunnelConfig,
		errCh:        make(chan error),
		stopCh:       nil,
	}
	return &mgr, nil
}

func (mgr *WarpManager) Config() Config {
	return *mgr.config
}

func (mgr *WarpManager) Active() bool {
	return mgr.stopCh != nil
}

func (mgr *WarpManager) Start(serviceURL string) error {

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

func (mgr *WarpManager) Stop() error {
	if mgr.stopCh == nil {
		return fmt.Errorf("tunnel %s already stopped", mgr.id)
	}
	close(mgr.stopCh)
	mgr.tunnelConfig.OriginUrl = ""
	mgr.stopCh = nil
	return <-mgr.errCh
}

func (mgr *WarpManager) TearDown() error {
	return mgr.Stop()
}

func (mgr *WarpManager) CheckStatus() error {
	return fmt.Errorf("Not implemented")
}
