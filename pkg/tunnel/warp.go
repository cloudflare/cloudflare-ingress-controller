package tunnel

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/cloudflare/cloudflare-warp-ingress/pkg/cloudflare"
	"github.com/cloudflare/cloudflare-warp/h2mux"
	"github.com/cloudflare/cloudflare-warp/origin"
	tunnelpogs "github.com/cloudflare/cloudflare-warp/tunnelrpc/pogs"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
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
func NewWarpManager(config *Config) (Tunnel, error) {

	// path to pem file should be passed in,
	// or here, assume that it is mounted at a particular location
	originCertPath := "/etc/cloudflare-warp/cert.pem"
	originCert, err := ioutil.ReadFile(originCertPath)
	if err != nil {
		return nil, fmt.Errorf("Cannot read %s to load origin certificate, %v", originCertPath, err)
	}

	tunnelConfig := origin.TunnelConfig{
		EdgeAddr:          "cftunnel.com:7844",
		OriginUrl:         config.ServiceName,
		Hostname:          config.ExternalHostname,
		OriginCert:        originCert,    // []byte{}
		TlsConfig:         &tls.Config{}, // need to load the cloudflare cert
		Retries:           5,
		HeartbeatInterval: 5 * time.Second,
		MaxHeartbeats:     5,
		ClientID:          utilrand.String(16),
		ReportedVersion:   "DEV",
		LBPool:            "",
		Tags:              []tunnelpogs.Tag{},
		ConnectedSignal:   h2mux.NewSignal(),
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

func (mgr *WarpManager) Start() error {

	mgr.stopCh = make(chan struct{})
	go func() {
		mgr.errCh <- origin.StartTunnelDaemon(mgr.tunnelConfig, mgr.stopCh)
	}()
	return nil
}

func (mgr *WarpManager) Stop() error {
	close(mgr.stopCh)
	mgr.stopCh = nil
	return <-mgr.errCh
}

func (mgr *WarpManager) TearDown() error {
	return mgr.Stop()
}

func (mgr *WarpManager) CheckStatus() error {
	return fmt.Errorf("Not implemented")
}
