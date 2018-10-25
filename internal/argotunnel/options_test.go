package argotunnel

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOptions(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		in  []Option
		out options
	}{
		"default-options": {
			in: []Option{},
			out: options{
				ingressClass: IngressClassDefault,
				resyncPeriod: ResyncPeriodDefault,
				requeueLimit: RequeueLimitDefault,
				workers:      WorkersDefault,
			},
		},
		"set-one-option": {
			in: []Option{
				IngressClass("test-class"),
			},
			out: options{
				ingressClass: "test-class",
				resyncPeriod: ResyncPeriodDefault,
				requeueLimit: RequeueLimitDefault,
				workers:      WorkersDefault,
			},
		},
		"set-all-options": {
			in: []Option{
				IngressClass("test-class"),
				ResyncPeriod(1 * time.Minute),
				RequeueLimit(-1),
				Secret("test-secret-name", "test-secret-namespace"),
				Workers(2),
			},
			out: options{
				ingressClass: "test-class",
				resyncPeriod: 1 * time.Minute,
				requeueLimit: -1,
				secret:       &resource{"test-secret-name", "test-secret-namespace"},
				workers:      2,
			},
		},
	} {
		out := collectOptions(test.in)
		assert.Equalf(t, test.out, out, "test '%s' options mismatch", name)
	}
}

func TestTunnelOptions(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		in  []tunnelOption
		out tunnelOptions
	}{
		"default-options": {
			in: []tunnelOption{},
			out: tunnelOptions{
				haConnections:     haConnectionsDefault,
				heartbeatCount:    heartbeatCountDefault,
				heartbeatInterval: heartbeatIntervalDefault,
				retries:           retriesDefault,
			},
		},
		"set-one-option": {
			in: []tunnelOption{
				retries(100),
			},
			out: tunnelOptions{
				haConnections:     haConnectionsDefault,
				heartbeatCount:    heartbeatCountDefault,
				heartbeatInterval: heartbeatIntervalDefault,
				retries:           100,
			},
		},
		"set-all-options": {
			in: []tunnelOption{
				compressionQuality(8),
				disableChunkedEncoding(true),
				gracePeriod(100 * time.Millisecond),
				haConnections(8),
				heartbeatCount(100),
				heartbeatInterval(100 * time.Millisecond),
				lbPool("test-lb"),
				retries(100),
			},
			out: tunnelOptions{
				compressionQuality: 8,
				noChunkedEncoding:  true,
				gracePeriod:        100 * time.Millisecond,
				haConnections:      8,
				heartbeatCount:     100,
				heartbeatInterval:  100 * time.Millisecond,
				lbPool:             "test-lb",
				retries:            100,
			},
		},
	} {
		out := collectTunnelOptions(test.in)
		assert.Equalf(t, test.out, out, "test '%s' options mismatch", name)
	}
}
