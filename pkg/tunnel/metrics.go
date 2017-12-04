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

func NewMetrics() *MetricsConfig {
	return &MetricsConfig{
		Metrics:         origin.NewTunnelMetrics(),
		UpdateFrequency: 5 * time.Second,
	}
}
