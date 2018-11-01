package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestGetEndpointsPort(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		obj  *v1.Endpoints
		port intstr.IntOrString
		out  v1.EndpointPort
		ok   bool
	}{
		"endpoints-nil": {
			obj:  nil,
			port: intstr.FromInt(80),
			out:  v1.EndpointPort{},
			ok:   false,
		},
		"endpoints-empty": {
			obj:  nil,
			port: intstr.FromInt(80),
			out:  v1.EndpointPort{},
			ok:   false,
		},
		"endpoints-no-ports": {
			obj: &v1.Endpoints{
				Subsets: []v1.EndpointSubset{
					{
						Ports: []v1.EndpointPort{},
					},
					{
						Ports: []v1.EndpointPort{},
					},
				},
			},
			port: intstr.FromInt(80),
			out:  v1.EndpointPort{},
			ok:   false,
		},
		"endpoints-no-str-port": {
			obj: &v1.Endpoints{
				Subsets: []v1.EndpointSubset{
					{
						Ports: []v1.EndpointPort{
							{
								Name: "unit-a",
								Port: 8080,
							},
							{
								Name: "unit-b",
								Port: 9090,
							},
						},
					},
					{
						Ports: []v1.EndpointPort{
							{
								Name: "unit-c",
								Port: 8081,
							},
							{
								Name: "unit-d",
								Port: 9091,
							},
						},
					},
				},
			},
			port: intstr.FromString("http"),
			out:  v1.EndpointPort{},
			ok:   false,
		},
		"endpoints-no-int-port": {
			obj: &v1.Endpoints{
				Subsets: []v1.EndpointSubset{
					{
						Ports: []v1.EndpointPort{
							{
								Name: "http-a",
								Port: 8080,
							},
							{
								Name: "unit-b",
								Port: 9090,
							},
						},
					},
					{
						Ports: []v1.EndpointPort{
							{
								Name: "unit-c",
								Port: 8081,
							},
							{
								Name: "unit-d",
								Port: 9091,
							},
						},
					},
				},
			},
			port: intstr.FromInt(80),
			out:  v1.EndpointPort{},
			ok:   false,
		},
		"endpoints-has-str-port": {
			obj: &v1.Endpoints{
				Subsets: []v1.EndpointSubset{
					{
						Ports: []v1.EndpointPort{
							{
								Name: "http",
								Port: 8080,
							},
							{
								Name: "grpc",
								Port: 9090,
							},
						},
					},
					{
						Ports: []v1.EndpointPort{
							{
								Name: "metrics",
								Port: 8081,
							},
							{
								Name: "pprof",
								Port: 9091,
							},
						},
					},
				},
			},
			port: intstr.FromString("pprof"),
			out: v1.EndpointPort{
				Name: "pprof",
				Port: 9091,
			},
			ok: true,
		},
		"endpoints-has-int-port": {
			obj: &v1.Endpoints{
				Subsets: []v1.EndpointSubset{
					{
						Ports: []v1.EndpointPort{
							{
								Name: "http",
								Port: 8080,
							},
							{
								Name: "grpc",
								Port: 9090,
							},
						},
					},
					{
						Ports: []v1.EndpointPort{
							{
								Name: "metrics",
								Port: 8081,
							},
							{
								Name: "pprof",
								Port: 9091,
							},
						},
					},
				},
			},
			port: intstr.FromInt(9090),
			out: v1.EndpointPort{
				Name: "grpc",
				Port: 9090,
			},
			ok: true,
		},
	} {
		out, ok := GetEndpointsPort(test.obj, test.port)
		assert.Equalf(t, test.out, out, "test '%s' value mismatch", name)
		assert.Equalf(t, test.ok, ok, "test '%s' exists mismatch", name)
	}
}

func TestGetSecretCert(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		in  *v1.Secret
		out []byte
		ok  bool
	}{
		"secret-nil": {
			in:  nil,
			out: []byte(nil),
			ok:  false,
		},
		"secret-empty": {
			in:  &v1.Secret{},
			out: []byte(nil),
			ok:  false,
		},
		"secret-no-cert": {
			in: &v1.Secret{
				Data: map[string][]byte{
					"cert.tls": []byte("fake-cert"),
				},
			},
			out: []byte(nil),
			ok:  false,
		},
		"secret-has-cert": {
			in: &v1.Secret{
				Data: map[string][]byte{
					"cert.pem": []byte("fake-cert"),
				},
			},
			out: []byte("fake-cert"),
			ok:  true,
		},
	} {
		out, ok := GetSecretCert(test.in)
		assert.Equalf(t, test.out, out, "test '%s' value mismatch", name)
		assert.Equalf(t, test.ok, ok, "test '%s' condition mismatch", name)
	}
}

func TestGetServicePort(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		obj  *v1.Service
		port intstr.IntOrString
		out  v1.ServicePort
		ok   bool
	}{
		"service-nil": {
			obj:  nil,
			port: intstr.FromInt(80),
			out:  v1.ServicePort{},
			ok:   false,
		},
		"service-empty": {
			obj:  &v1.Service{},
			port: intstr.FromInt(80),
			out:  v1.ServicePort{},
			ok:   false,
		},
		"service-no-ports": {
			obj: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{},
				},
			},
			out: v1.ServicePort{},
			ok:  false,
		},
		"service-no-str-port": {
			obj: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Name: "unit-a",
							Port: 8080,
						},
						{
							Name: "unit-b",
							Port: 9090,
						},
					},
				},
			},
			port: intstr.FromString("http"),
			out:  v1.ServicePort{},
			ok:   false,
		},
		"service-no-int-port": {
			obj: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Name: "unit-a",
							Port: 8080,
						},
						{
							Name: "unit-b",
							Port: 9090,
						},
					},
				},
			},
			port: intstr.FromInt(80),
			out:  v1.ServicePort{},
			ok:   false,
		},
		"service-has-str-port": {
			obj: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Name: "http",
							Port: 8080,
						},
						{
							Name: "grpc",
							Port: 9090,
						},
					},
				},
			},
			port: intstr.FromString("http"),
			out: v1.ServicePort{
				Name: "http",
				Port: 8080,
			},
			ok: true,
		},
		"service-has-int-port": {
			obj: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Name: "http",
							Port: 8080,
						},
						{
							Name: "grpc",
							Port: 9090,
						},
					},
				},
			},
			port: intstr.FromInt(9090),
			out: v1.ServicePort{
				Name: "grpc",
				Port: 9090,
			},
			ok: true,
		},
	} {
		out, ok := GetServicePort(test.obj, test.port)
		assert.Equalf(t, test.out, out, "test '%s' value mismatch", name)
		assert.Equalf(t, test.ok, ok, "test '%s' exists mismatch", name)
	}
}
