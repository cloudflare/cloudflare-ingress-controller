package tunnel

import "k8s.io/apimachinery/pkg/util/intstr"

// Tunnel is the interface for different implementation of
// the cloudflare-warp tunnel, matching an external hostname
// to a kubernetes service
type Tunnel interface {

	// Config returns the tunnel configuration
	Config() Config

	// Start the tunnel to connect ot a particular service url, making it active
	Start(serviceURL string) error

	// Stop the tunnel, making it inactive
	Stop() error

	// Active tells whether the tunnel is active or not
	Active() bool

	// TearDown cleans up all external resources
	TearDown() error

	// CheckStatus validates the current state of the tunnel
	CheckStatus() error
}

// Config contains the smallest set of elements to define
// a warp tunnel
type Config struct {
	ServiceName      string
	ServicePort      intstr.IntOrString // maps either to service.Name (string) or service.Port (int32)
	ExternalHostname string
	OriginCert       []byte
}
