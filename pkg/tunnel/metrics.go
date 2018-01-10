package tunnel

import (
	"time"

	"github.com/cloudflare/cloudflare-warp/origin"
)

// MetricsConfig wraps the cloudflare-warp tunnel metrics in a struct
type MetricsConfig struct {
	Metrics         *origin.TunnelMetrics
	UpdateFrequency time.Duration
}

// NewMetrics created a set of TunnelMetrics,
// initializes global prometheus objects which breaks tests
func NewMetrics() *MetricsConfig {
	return &MetricsConfig{
		Metrics:         origin.NewTunnelMetrics(),
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
