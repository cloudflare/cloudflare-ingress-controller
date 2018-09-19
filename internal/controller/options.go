package controller

const (
	// IngressClassDefault defines the default class of ingresses managed by the controller
	IngressClassDefault = "argo-tunnel"

	// SecretNamespaceDefault defines the default namespace use to house tunnel secrets
	// TODO: secrets should not be centally housed
	// In multi-tentant environments, all tenants should be able to add secrets and define
	// tunnels without access to a shared namespace.
	SecretNamespaceDefault = "default"

	// SecretNameDefault defines the default name for the default tunnel secret
	// TODO: the concept of a hardcoded default secret name should be deprecated
	// with the secret namespace, preferring a simple option to define a default
	// tunnel secret by "namespace/name"
	SecretNameDefault = "cloudflared-cert"
)

type options struct {
	enableMetrics   bool
	ingressClass    string
	secretNamespace string
	secretName      string
	version         string
}

// Option provides behavior overrides
type Option func(*options)

// EnableMetrics enables tunnel metrics
func EnableMetrics(b bool) Option {
	return func(o *options) {
		o.enableMetrics = b
	}
}

// IngressClass defines the ingress class for the controller
func IngressClass(s string) Option {
	return func(o *options) {
		o.ingressClass = s
	}
}

// SecretName defines the namespace used to house tunnel secrets
func SecretName(s string) Option {
	return func(o *options) {
		o.secretName = s
	}
}

// SecretNamespace defines the namespace used to house tunnel secrets
func SecretNamespace(s string) Option {
	return func(o *options) {
		o.secretNamespace = s
	}
}

// Version defines the tunnel version
func Version(s string) Option {
	return func(o *options) {
		o.version = s
	}
}

func collectOptions(opts []Option) options {
	// set option defaults
	o := options{
		ingressClass:    IngressClassDefault,
		secretNamespace: SecretNamespaceDefault,
		secretName:      SecretNameDefault,
	}
	// overlay option values
	for _, opt := range opts {
		opt(&o)
	}
	return o
}
