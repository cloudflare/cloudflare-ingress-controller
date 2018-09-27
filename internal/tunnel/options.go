package tunnel

import (
	"time"
)

const (
	// HaConnectionsDefault defines the default high-availability connections
	HaConnectionsDefault = 4
	// HeartbeatCountDefault defines the default heartbeat count
	// Minimum number of unacknowledged heartbeats to send before closing the connection.
	HeartbeatCountDefault = uint64(5)
	// HeartbeatIntervalDefault defines the default interval between heartbeats
	// Minimum number of unacked heartbeats to send before closing the connection.
	HeartbeatIntervalDefault = time.Second * 5
	// RetriesDefault defines the default number of attempts to repair on failure
	// Maximum number of retries for connection/protocol errors.
	RetriesDefault = uint(5)
)

// Options optional tunnel behavior configurations
// todo: change to not-exported with resource handling refactor
type Options struct {
	CompressionQuality uint64
	GracePeriod        time.Duration
	HaConnections      int
	HeartbeatCount     uint64
	HeartbeatInterval  time.Duration
	LbPool             string
	NoChunkedEncoding  bool
	Retries            uint
}

// Option provides behavior overrides
type Option func(*Options)

// CompressionQuality sets tunnel compressions quality
func CompressionQuality(i uint64) Option {
	return func(o *Options) {
		o.CompressionQuality = i
	}
}

// DisableChunkedEncoding disable chunked encoding
func DisableChunkedEncoding(b bool) Option {
	return func(o *Options) {
		o.NoChunkedEncoding = b
	}
}

// GracePeriod sets tunnel grace period
func GracePeriod(d time.Duration) Option {
	return func(o *Options) {
		o.GracePeriod = d
	}
}

// HaConnections sets tunnel ha connections
func HaConnections(i int) Option {
	return func(o *Options) {
		o.HaConnections = i
	}
}

// HeartbeatCount sets tunnel heartbeat count
func HeartbeatCount(i uint64) Option {
	return func(o *Options) {
		o.HeartbeatCount = i
	}
}

// HeartbeatInterval sets tunnel heartbeat interval
func HeartbeatInterval(d time.Duration) Option {
	return func(o *Options) {
		o.HeartbeatInterval = d
	}
}

// LbPool sets tunnel load balancer pool
func LbPool(s string) Option {
	return func(o *Options) {
		o.LbPool = s
	}
}

// Retries sets tunnel retries
func Retries(i uint) Option {
	return func(o *Options) {
		o.Retries = i
	}
}

// CollectOptions flattens an array of Option into an Options struct
func CollectOptions(opts []Option) Options {
	// set option defaults
	o := Options{
		HaConnections:     HaConnectionsDefault,
		HeartbeatCount:    HeartbeatCountDefault,
		HeartbeatInterval: HeartbeatIntervalDefault,
		Retries:           RetriesDefault,
	}
	// overlay option values
	for _, opt := range opts {
		opt(&o)
	}
	return o
}
