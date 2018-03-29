package tunnel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestWarpConfig(t *testing.T) {

	config := &Config{
		ServiceName:      "service",
		ServiceNamespace: "default",
		ServicePort:      intstr.FromInt(6000),
		ExternalHostname: "acme.example.com",
		LBPool:           "abc123",
		OriginCert:       []byte("this is not a cert"),
	}
	metricsConfig := NewDummyMetrics()

	mgr, err := NewWarpManager(config, metricsConfig)
	assert.Nil(t, err)

	warp := mgr.(*WarpManager)
	assert.NotNil(t, warp)

	assert.Equal(t, warp.tunnelConfig.Hostname, config.ExternalHostname)
	assert.Equal(t, warp.tunnelConfig.LBPool, config.LBPool)
	assert.Equal(t, warp.tunnelConfig.OriginCert, config.OriginCert)

	assert.Equal(t, warp.tunnelConfig.HAConnections, 1)
	assert.NotNil(t, warp.tunnelConfig.ProtocolLogger)

	assert.Equal(t, warp.config.ServiceName, config.ServiceName)
	assert.Equal(t, warp.config.ServicePort, config.ServicePort)

	assert.False(t, warp.Active())
}
