package tunnel

import (
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Route describes the path to the origin
type Route struct {
	ServiceName      string
	ServicePort      intstr.IntOrString // maps either to service.Name (string) or service.Port (int32)
	IngressName      string
	Namespace        string
	ExternalHostname string
	OriginCert       []byte
	Version          string
}
