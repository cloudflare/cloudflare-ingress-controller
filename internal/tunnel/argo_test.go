package tunnel

import (
	"testing"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestArgoTunnelConfig(t *testing.T) {
	logger, hook := test.NewNullLogger()
	route := Route{
		ServiceName:      "acme",
		ServicePort:      intstr.FromInt(6000),
		IngressName:      "acme",
		Namespace:        "default",
		ExternalHostname: "acme.example.com",
		OriginCert:       []byte("this is not a cert"),
		Version:          "test",
	}

	tunnel, err := NewArgoTunnel(route, logger)
	assert.Nil(t, err)

	argot := tunnel.(*ArgoTunnel)
	assert.NotNil(t, argot)

	assert.Equal(t, argot.tunnelConfig.Hostname, route.ExternalHostname)
	assert.Equal(t, argot.tunnelConfig.OriginCert, route.OriginCert)

	assert.Equal(t, argot.tunnelConfig.HAConnections, HaConnectionsDefault)
	assert.Equal(t, argot.tunnelConfig.Retries, RetriesDefault)
	assert.NotNil(t, argot.tunnelConfig.ProtocolLogger)

	assert.Equal(t, argot.route.ServiceName, route.ServiceName)
	assert.Equal(t, argot.route.ServicePort, route.ServicePort)

	// TODO write a test for the post-start condition where the origin url and port have been determined
	//assert.Equal(t, fmt.Sprintf("%s.%s:%d", config.ServiceName, config.Namespace, config.ServicePort.IntValue()), argot.tunnelConfig.OriginUrl)
	assert.Equal(t, "", argot.tunnelConfig.OriginUrl)
	assert.False(t, argot.Active())

	hook.Reset()
	assert.Nil(t, hook.LastEntry())
}
