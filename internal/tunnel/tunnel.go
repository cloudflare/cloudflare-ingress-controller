package tunnel

// Tunnel is the interface for different implementation of
// the argo tunnel, matching an external hostname
// to a kubernetes service
type Tunnel interface {
	// Origin returns the tunnel origin
	Origin() string

	// Route returns the tunnel configuration
	Route() Route

	// Options returns the tunnel options
	Options() Options

	// Start the tunnel to connect to a particular service url, making it active
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
