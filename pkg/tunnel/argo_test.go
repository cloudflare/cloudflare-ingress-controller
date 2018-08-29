package tunnel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestArgoTunnelConfig(t *testing.T) {

	config := &Config{
		ServiceName:      "service",
		ServiceNamespace: "default",
		ServicePort:      intstr.FromInt(6000),
		ExternalHostname: "acme.example.com",
		LBPool:           "abc123",
		OriginCert:       []byte("this is not a cert"),
	}

	metricsConfig := NewMetrics(ArgoMetricsLabelKeys())

	mgr, err := NewArgoTunnelManager(config, metricsConfig)
	assert.Nil(t, err)

	argot := mgr.(*ArgoTunnelManager)
	assert.NotNil(t, argot)

	assert.Equal(t, argot.tunnelConfig.Hostname, config.ExternalHostname)
	assert.Equal(t, argot.tunnelConfig.LBPool, config.LBPool)
	assert.Equal(t, argot.tunnelConfig.OriginCert, config.OriginCert)

	assert.Equal(t, argot.tunnelConfig.HAConnections, 1)
	assert.NotNil(t, argot.tunnelConfig.ProtocolLogger)

	assert.Equal(t, argot.config.ServiceName, config.ServiceName)
	assert.Equal(t, argot.config.ServicePort, config.ServicePort)

	// TODO write a test for the post-start condition where the origin url and port have been determined
	//assert.Equal(t, fmt.Sprintf("%s.%s:%d", config.ServiceName, config.ServiceNamespace, config.ServicePort.IntValue()), argot.tunnelConfig.OriginUrl)
	assert.Equal(t, "", argot.tunnelConfig.OriginUrl)

	assert.False(t, argot.Active())
}
