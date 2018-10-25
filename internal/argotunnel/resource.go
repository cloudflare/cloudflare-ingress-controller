package argotunnel

const (
	endpointKind = "endpoint"
	ingressKind  = "ingress"
	secretKind   = "secret"
	serviceKind  = "service"
)

type resource struct {
	name      string
	namespace string
}
