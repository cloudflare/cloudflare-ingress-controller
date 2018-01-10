package tunnel

type FakeTunnel struct {
	config Config
	active bool
}

func (f *FakeTunnel) Config() Config {
	return f.config
}

// Start the tunnel to connect ot a particular service url, making it active
func (f *FakeTunnel) Start(serviceURL string) error {
	f.active = true
	return nil
}

// Stop the tunnel, making it inactive
func (f *FakeTunnel) Stop() error {
	f.active = false
	return nil
}

// Active tells whether the tunnel is active or not
func (f *FakeTunnel) Active() bool {
	return f.active
}

// TearDown cleans up all external resources
func (f *FakeTunnel) TearDown() error {
	return f.Stop()
}

// CheckStatus validates the current state of the tunnel
func (f *FakeTunnel) CheckStatus() error {
	return nil
}

func NewFakeManager(config *Config, metricsSetup *MetricsConfig) (Tunnel, error) {
	return &FakeTunnel{}, nil
}
