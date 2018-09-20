package tunnel

import (
	"github.com/stretchr/testify/mock"
)

type mockTunnel struct {
	mock.Mock
}

func (t *mockTunnel) Config() Config {
	args := t.Called()
	return args.Get(0).(Config)
}

func (t *mockTunnel) Start(serviceURL string) error {
	args := t.Called(serviceURL)
	return args.Error(0)
}

func (t *mockTunnel) Stop() error {
	args := t.Called()
	return args.Error(0)
}

func (t *mockTunnel) Active() bool {
	args := t.Called()
	return args.Get(0).(bool)
}

func (t *mockTunnel) TearDown() error {
	args := t.Called()
	return args.Error(0)
}

func (t *mockTunnel) CheckStatus() error {
	args := t.Called()
	return args.Error(0)
}
