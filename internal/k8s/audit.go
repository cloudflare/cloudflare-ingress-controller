package k8s

import (
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	// CertPem is the string constant used to locate a secrets cert
	CertPem = "cert.pem"
)

// EndpointsHaveSubsets verify that subsets exist
func EndpointsHaveSubsets(ep *v1.Endpoints) bool {
	if ep != nil {
		for _, subset := range ep.Subsets {
			if len(subset.Addresses) > 0 {
				return true
			}
		}
	}
	return false
}

// GetSecretCert extracts the 'cert.pem' from a secret
func GetSecretCert(sec *v1.Secret) (cert []byte, exists bool) {
	if sec != nil {
		cert, exists = sec.Data[CertPem]
	}
	return
}

// GetServicePort extracts the port defined by a service
func GetServicePort(svc *v1.Service, port intstr.IntOrString) (val int32, exists bool) {
	if svc != nil {
		for _, servicePort := range svc.Spec.Ports {
			switch port.Type {
			case intstr.Int:
				if servicePort.Port == port.IntVal {
					return servicePort.Port, true
				}
			case intstr.String:
				if servicePort.Name == port.StrVal {
					return servicePort.Port, true
				}
			}
		}
	}
	return
}
