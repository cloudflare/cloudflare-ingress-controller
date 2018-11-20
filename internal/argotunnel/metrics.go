package argotunnel

import (
	"sync"
	"time"

	"github.com/cloudflare/cloudflared/origin"
)

// TODO: Review the metrics pattern used by cloudflared and
// migrate towards go-kit metrics with configurable providers.
var metricsConfig = struct {
	metrics         *origin.TunnelMetrics
	updateFrequency time.Duration
	setMetrics      sync.Once
}{
	metrics:         &origin.TunnelMetrics{},
	updateFrequency: 10000 * time.Hour,
}

// EnableMetrics configures the metrics used by all tunnels
func EnableMetrics(updateFrequency time.Duration) {
	metricsConfig.setMetrics.Do(func() {
		metricsConfig.metrics = origin.NewTunnelMetrics()
		metricsConfig.updateFrequency = updateFrequency
	})
}
