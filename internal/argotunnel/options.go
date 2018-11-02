package argotunnel

import (
	"time"
)

const (
	// IngressClassDefault defines the default class of ingresses managed by the controller
	IngressClassDefault = "argo-tunnel"

	// ResyncPeriodDefault defines the default duration prior to synchronization
	ResyncPeriodDefault = 5 * time.Minute

	// RequeueLimitDefault defines the default processing attempts before dropping the item
	RequeueLimitDefault = 2

	// WorkersDefault defines the default number of workers processing items from the queue
	WorkersDefault = 2
)

type options struct {
	ingressClass string
	resyncPeriod time.Duration
	requeueLimit int
	secret       *resource
	workers      int
}

// Option provides behavior overrides
type Option func(*options)

// IngressClass defines the ingress class for the controller
func IngressClass(s string) Option {
	return func(o *options) {
		o.ingressClass = s
	}
}

// ResyncPeriod defines the duration prior to synchronization
func ResyncPeriod(d time.Duration) Option {
	return func(o *options) {
		o.resyncPeriod = d
	}
}

// RequeueLimit defines the processing attempts before dropping the item
func RequeueLimit(i int) Option {
	return func(o *options) {
		o.requeueLimit = i
	}
}

// Secret defines the default secret used by tunnels
func Secret(name, namespace string) Option {
	return func(o *options) {
		if len(name) > 0 && len(namespace) > 0 {
			o.secret = &resource{
				name:      name,
				namespace: namespace,
			}
		}
	}
}

// Workers defines the number of queue consumers
func Workers(i int) Option {
	return func(o *options) {
		o.workers = i
	}
}

func collectOptions(opts []Option) options {
	// set defaults
	o := options{
		ingressClass: IngressClassDefault,
		resyncPeriod: ResyncPeriodDefault,
		requeueLimit: RequeueLimitDefault,
		workers:      WorkersDefault,
	}
	// overlay values
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

const (
	// haConnectionsDefault defines the default high-availability connections
	haConnectionsDefault = 4
	// heartbeatCountDefault defines the default heartbeat count
	// Minimum number of unacknowledged heartbeats to send before closing the connection.
	heartbeatCountDefault = uint64(5)
	// heartbeatIntervalDefault defines the default interval between heartbeats
	// Minimum number of unacked heartbeats to send before closing the connection.
	heartbeatIntervalDefault = time.Second * 5
	// retriesDefault defines the default number of attempts to repair on failure
	// Maximum number of retries for connection/protocol errors.
	retriesDefault = uint(3)
)

type tunnelOptions struct {
	compressionQuality uint64
	gracePeriod        time.Duration
	haConnections      int
	heartbeatCount     uint64
	heartbeatInterval  time.Duration
	lbPool             string
	noChunkedEncoding  bool
	retries            uint
}

type tunnelOption func(*tunnelOptions)

func compressionQuality(i uint64) tunnelOption {
	return func(o *tunnelOptions) {
		o.compressionQuality = i
	}
}

func disableChunkedEncoding(b bool) tunnelOption {
	return func(o *tunnelOptions) {
		o.noChunkedEncoding = b
	}
}

func gracePeriod(d time.Duration) tunnelOption {
	return func(o *tunnelOptions) {
		o.gracePeriod = d
	}
}

func haConnections(i int) tunnelOption {
	return func(o *tunnelOptions) {
		o.haConnections = i
	}
}

func heartbeatCount(i uint64) tunnelOption {
	return func(o *tunnelOptions) {
		o.heartbeatCount = i
	}
}

func heartbeatInterval(d time.Duration) tunnelOption {
	return func(o *tunnelOptions) {
		o.heartbeatInterval = d
	}
}

func lbPool(s string) tunnelOption {
	return func(o *tunnelOptions) {
		o.lbPool = s
	}
}

func retries(i uint) tunnelOption {
	return func(o *tunnelOptions) {
		o.retries = i
	}
}

func collectTunnelOptions(opts []tunnelOption) tunnelOptions {
	// set defaults
	o := tunnelOptions{
		haConnections:     haConnectionsDefault,
		heartbeatCount:    heartbeatCountDefault,
		heartbeatInterval: heartbeatIntervalDefault,
		retries:           retriesDefault,
	}
	// overlay values
	for _, opt := range opts {
		opt(&o)
	}
	return o
}
