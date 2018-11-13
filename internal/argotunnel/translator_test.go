package argotunnel

import (
	"fmt"
	"testing"

	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/cache"
)

func TestHandleResource(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		tr   *syncTranslator
		kind string
		key  string
		out  error
	}{
		"kind-unexpected": {
			tr:   newMockedSyncTranslator(),
			kind: "unit",
			key:  "unit/unit",
			out:  fmt.Errorf("unexpected kind (%q) in key (%q)", "unit", "unit/unit"),
		},
		"kind-endpoint-idx-err": {
			tr: &syncTranslator{
				informers: informerset{
					endpoint: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("GetByKey", "unit/svc-a").Return(struct{}{}, false, fmt.Errorf("short-circuit"))
							return idx
						}())
						return i
					}(),
					ingress: &mockSharedIndexInformer{},
					secret:  &mockSharedIndexInformer{},
					service: &mockSharedIndexInformer{},
				},
			},
			kind: "endpoint",
			key:  "unit/svc-a",
			out:  fmt.Errorf("short-circuit"),
		},
		"kind-ingress-idx-err": {
			tr: &syncTranslator{
				informers: informerset{
					endpoint: &mockSharedIndexInformer{},
					ingress: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("GetByKey", "unit/ing-a").Return(struct{}{}, false, fmt.Errorf("short-circuit"))
							return idx
						}())
						return i
					}(),
					secret:  &mockSharedIndexInformer{},
					service: &mockSharedIndexInformer{},
				},
			},
			kind: "ingress",
			key:  "unit/ing-a",
			out:  fmt.Errorf("short-circuit"),
		},
		"kind-secret-idx-err": {
			tr: &syncTranslator{
				informers: informerset{
					endpoint: &mockSharedIndexInformer{},
					ingress:  &mockSharedIndexInformer{},
					secret: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("GetByKey", "unit/sec-a").Return(struct{}{}, false, fmt.Errorf("short-circuit"))
							return idx
						}())
						return i
					}(),
					service: &mockSharedIndexInformer{},
				},
			},
			kind: "secret",
			key:  "unit/sec-a",
			out:  fmt.Errorf("short-circuit"),
		},
		"kind-service-idx-err": {
			tr: &syncTranslator{
				informers: informerset{
					endpoint: &mockSharedIndexInformer{},
					ingress:  &mockSharedIndexInformer{},
					secret:   &mockSharedIndexInformer{},
					service: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("GetByKey", "unit/svc-a").Return(struct{}{}, false, fmt.Errorf("short-circuit"))
							return idx
						}())
						return i
					}(),
				},
			},
			kind: "service",
			key:  "unit/svc-a",
			out:  fmt.Errorf("short-circuit"),
		},
	} {
		logger, hook := logtest.NewNullLogger()
		test.tr.log = logger
		out := test.tr.handleResource(test.kind, test.key)
		assert.Equalf(t, test.out, out, "test '%s' error mismatch", name)
		hook.Reset()
		assert.Nil(t, hook.LastEntry())
	}
}

func TestHandleByKind(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		tr   *syncTranslator
		kind string
		key  string
		out  error
	}{
		"kind-unexpected": {
			tr:   newMockedSyncTranslator(),
			kind: "unit",
			key:  "unit/unit",
			out:  fmt.Errorf("unexpected kind (%q)", "unit"),
		},
		"kind-secret-idx-err": {
			tr: &syncTranslator{
				informers: informerset{
					endpoint: &mockSharedIndexInformer{},
					ingress:  &mockSharedIndexInformer{},
					secret: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("GetByKey", "unit/sec-a").Return(struct{}{}, false, fmt.Errorf("short-circuit"))
							return idx
						}())
						return i
					}(),
					service: &mockSharedIndexInformer{},
				},
			},
			kind: "secret",
			key:  "unit/sec-a",
			out:  fmt.Errorf("short-circuit"),
		},
		"kind-secret-idx-update-err": {
			tr: &syncTranslator{
				informers: informerset{
					endpoint: &mockSharedIndexInformer{},
					ingress: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("ByIndex", "secret", "unit/sec-a").Return(make([]interface{}, 0, 0), fmt.Errorf("short-circuit"))
							return idx
						}())
						return i
					}(),
					secret: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("GetByKey", "unit/sec-a").Return(struct{}{}, true, nil)
							return idx
						}())
						return i
					}(),
					service: &mockSharedIndexInformer{},
				},
			},
			kind: "secret",
			key:  "unit/sec-a",
			out:  fmt.Errorf("short-circuit"),
		},
		"kind-secret-idx-delete-err": {
			tr: &syncTranslator{
				informers: informerset{
					endpoint: &mockSharedIndexInformer{},
					ingress: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("IndexKeys", "secret", "unit/sec-a").Return(make([]string, 0, 0), fmt.Errorf("short-circuit"))
							return idx
						}())
						return i
					}(),
					secret: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("GetByKey", "unit/sec-a").Return(struct{}{}, false, nil)
							return idx
						}())
						return i
					}(),
					service: &mockSharedIndexInformer{},
				},
			},
			kind: "secret",
			key:  "unit/sec-a",
			out:  fmt.Errorf("short-circuit"),
		},
	} {
		logger, hook := logtest.NewNullLogger()
		test.tr.log = logger
		out := test.tr.handleByKind(test.kind, test.key)
		assert.Equalf(t, test.out, out, "test '%s' error mismatch", name)
		hook.Reset()
		assert.Nil(t, hook.LastEntry())
	}
}

func TestHandleEndpoint(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		tr   *syncTranslator
		kind string
		key  string
		out  error
	}{
		"kind-endpoint-idx-err": {
			tr: &syncTranslator{
				informers: informerset{
					endpoint: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("GetByKey", "unit/svc-a").Return(struct{}{}, false, fmt.Errorf("short-circuit"))
							return idx
						}())
						return i
					}(),
					ingress: &mockSharedIndexInformer{},
					secret:  &mockSharedIndexInformer{},
					service: &mockSharedIndexInformer{},
				},
			},
			kind: "endpoint",
			key:  "unit/svc-a",
			out:  fmt.Errorf("short-circuit"),
		},
		"kind-endpoint-idx-update-err": {
			tr: &syncTranslator{
				informers: informerset{
					endpoint: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("GetByKey", "unit/svc-a").Return(struct{}{}, true, nil)
							return idx
						}())
						return i
					}(),
					ingress: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("ByIndex", "service", "unit/svc-a").Return(make([]interface{}, 0, 0), fmt.Errorf("short-circuit"))
							return idx
						}())
						return i
					}(),
					secret:  &mockSharedIndexInformer{},
					service: &mockSharedIndexInformer{},
				},
			},
			kind: "endpoint",
			key:  "unit/svc-a",
			out:  fmt.Errorf("short-circuit"),
		},
		"kind-secret-idx-delete-err": {
			tr: &syncTranslator{
				informers: informerset{
					endpoint: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("GetByKey", "unit/svc-a").Return(struct{}{}, false, nil)
							return idx
						}())
						return i
					}(),
					ingress: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("IndexKeys", "service", "unit/svc-a").Return(make([]string, 0, 0), fmt.Errorf("short-circuit"))
							return idx
						}())
						return i
					}(),
					secret:  &mockSharedIndexInformer{},
					service: &mockSharedIndexInformer{},
				},
			},
			kind: "secret",
			key:  "unit/svc-a",
			out:  fmt.Errorf("short-circuit"),
		},
	} {
		logger, hook := logtest.NewNullLogger()
		test.tr.log = logger
		out := test.tr.handleEndpoint(test.kind, test.key)
		assert.Equalf(t, test.out, out, "test '%s' error mismatch", name)
		hook.Reset()
		assert.Nil(t, hook.LastEntry())
	}
}

func TestGetRouteFromIngress(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		tr  *syncTranslator
		ing *v1beta1.Ingress
		out *tunnelRoute
	}{
		"ing-nil": {
			tr:  newMockedSyncTranslator(),
			ing: nil,
			out: nil,
		},
		"ing-empty": {
			tr: newMockedSyncTranslator(),
			ing: &v1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unit",
					Namespace: "unit",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "v1beta1",
				},
				Spec: v1beta1.IngressSpec{
					TLS:   []v1beta1.IngressTLS{},
					Rules: []v1beta1.IngressRule{},
				},
			},
			out: &tunnelRoute{
				name:      "unit",
				namespace: "unit",
				links:     tunnelRouteLinkMap{},
			},
		},
		"ing-add-rule": {
			tr: &syncTranslator{
				informers: informerset{
					endpoint: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("GetByKey", "unit/svc-a").Return(&v1.Endpoints{
								Subsets: []v1.EndpointSubset{
									{
										Ports: []v1.EndpointPort{
											{
												Name:     "http",
												Port:     9090,
												Protocol: v1.ProtocolTCP,
											},
										},
									},
								},
							}, true, nil)
							return idx
						}())
						return i
					}(),
					ingress: &mockSharedIndexInformer{},
					secret: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("GetByKey", "unit/sec-a").Return(&v1.Secret{
								Data: map[string][]byte{
									"cert.pem": genCertforHost("a.unit.com"),
								},
							}, true, nil)
							return idx
						}())
						return i
					}(),
					service: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("GetByKey", "unit/svc-a").Return(&v1.Service{
								Spec: v1.ServiceSpec{
									Ports: []v1.ServicePort{
										{
											Name:       "http",
											Port:       8080,
											TargetPort: intstr.FromInt(9090),
											Protocol:   v1.ProtocolTCP,
										},
									},
								},
							}, true, nil)
							return idx
						}())
						return i
					}(),
				},
				router: func() tunnelRouter {
					r := &mockTunnelRouter{}
					return r
				}(),
			},
			ing: &v1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unit",
					Namespace: "unit",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "v1beta1",
				},
				Spec: v1beta1.IngressSpec{
					TLS: []v1beta1.IngressTLS{
						{
							Hosts: []string{
								"a.unit.com",
							},
							SecretName: "sec-a",
						},
					},
					Rules: []v1beta1.IngressRule{
						{
							Host: "a.unit.com",
							IngressRuleValue: v1beta1.IngressRuleValue{
								HTTP: &v1beta1.HTTPIngressRuleValue{
									Paths: []v1beta1.HTTPIngressPath{
										{
											Backend: v1beta1.IngressBackend{
												ServiceName: "svc-a",
												ServicePort: intstr.FromString("http"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			out: &tunnelRoute{
				name:      "unit",
				namespace: "unit",
				links: tunnelRouteLinkMap{
					tunnelRule{
						host: "a.unit.com",
						port: 8080,
						service: resource{
							namespace: "unit",
							name:      "svc-a",
						},
						secret: resource{
							namespace: "unit",
							name:      "sec-a",
						},
					}: nil,
				},
			},
		},
	} {
		logger, hook := logtest.NewNullLogger()
		test.tr.log = logger
		out := func() (r *tunnelRoute) {
			if r = test.tr.getRouteFromIngress(test.ing); r != nil {
				l := tunnelRouteLinkMap{}
				for k := range r.links {
					l[k] = nil
				}
				r.links = l
			}
			return
		}()
		assert.Equalf(t, test.out, out, "test '%s' route mismatch", name)
		hook.Reset()
		assert.Nil(t, hook.LastEntry())
	}
}

func TestGetVerifiedCert(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		tr     *syncTranslator
		secret resource
		host   string
		cert   []byte
		exists bool
		err    error
	}{
		"secret-does-not-exist": {
			tr: &syncTranslator{
				informers: informerset{
					secret: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("GetByKey", "unit/sec-a").Return(&v1.Secret{}, false, nil)
							return idx
						}())
						return i
					}(),
				},
				router: &mockTunnelRouter{},
			},
			secret: resource{
				namespace: "unit",
				name:      "sec-a",
			},
			host:   "a.unit.com",
			cert:   nil,
			exists: false,
			err:    fmt.Errorf("secret 'unit/sec-a' does not exist"),
		},
		"secret-lookup-error": {
			tr: &syncTranslator{
				informers: informerset{
					secret: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("GetByKey", "unit/sec-a").Return(&v1.Secret{}, false, fmt.Errorf("lookup-error"))
							return idx
						}())
						return i
					}(),
				},
				router: &mockTunnelRouter{},
			},
			secret: resource{
				namespace: "unit",
				name:      "sec-a",
			},
			host:   "a.unit.com",
			cert:   nil,
			exists: false,
			err:    fmt.Errorf("lookup-error"),
		},
		"secret-no-cert": {
			tr: &syncTranslator{
				informers: informerset{
					secret: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("GetByKey", "unit/sec-a").Return(&v1.Secret{
								Data: map[string][]byte{},
							}, true, nil)
							return idx
						}())
						return i
					}(),
				},
				router: &mockTunnelRouter{},
			},
			secret: resource{
				namespace: "unit",
				name:      "sec-a",
			},
			host:   "a.unit.com",
			cert:   nil,
			exists: false,
			err:    fmt.Errorf("secret 'unit/sec-a' missing 'cert.pem'"),
		},
		"secret-okay": {
			tr: &syncTranslator{
				informers: informerset{
					secret: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("GetByKey", "unit/sec-a").Return(&v1.Secret{
								Data: map[string][]byte{
									"cert.pem": genCertforHost("a.unit.com"),
								},
							}, true, nil)
							return idx
						}())
						return i
					}(),
				},
				router: &mockTunnelRouter{},
			},
			secret: resource{
				namespace: "unit",
				name:      "sec-a",
			},
			host:   "a.unit.com",
			cert:   nil,
			exists: true,
			err:    nil,
		},
	} {
		cert, exists, err := test.tr.getVerifiedCert(test.secret.namespace, test.secret.name, test.host)
		assert.Equalf(t, test.exists, exists, "test '%s' exists mismatch", name)
		assert.Equalf(t, test.err, err, "test '%s' error mismatch", name)
		if exists {
			assert.NotEmptyf(t, cert, "test '%s' cert not empty", name)
		} else {
			assert.Emptyf(t, cert, "test '%s' cert empty", name)
		}
	}
}

func TestGetVerifiedPort(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		tr      *syncTranslator
		service resource
		port    intstr.IntOrString
		out     int32
		exists  bool
		err     error
	}{
		"service-does-not-exist": {
			tr: &syncTranslator{
				informers: informerset{
					service: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("GetByKey", "unit/svc-a").Return(&v1.Service{}, false, nil)
							return idx
						}())
						return i
					}(),
				},
				router: &mockTunnelRouter{},
			},
			service: resource{
				namespace: "unit",
				name:      "svc-a",
			},
			port:   intstr.FromInt(8080),
			out:    0,
			exists: false,
			err:    fmt.Errorf("service 'unit/svc-a' does not exist"),
		},
		"service-lookup-error": {
			tr: &syncTranslator{
				informers: informerset{
					service: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("GetByKey", "unit/svc-a").Return(&v1.Service{}, false, fmt.Errorf("lookup-error"))
							return idx
						}())
						return i
					}(),
				},
				router: &mockTunnelRouter{},
			},
			service: resource{
				namespace: "unit",
				name:      "svc-a",
			},
			port:   intstr.FromInt(8080),
			out:    0,
			exists: false,
			err:    fmt.Errorf("lookup-error"),
		},
		"service-missing-port": {
			tr: &syncTranslator{
				informers: informerset{
					service: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("GetByKey", "unit/svc-a").Return(&v1.Service{
								Spec: v1.ServiceSpec{
									Ports: []v1.ServicePort{
										{
											Name:     "port-a",
											Port:     8080,
											Protocol: v1.ProtocolTCP,
										},
									},
								},
							}, true, nil)
							return idx
						}())
						return i
					}(),
				},
				router: &mockTunnelRouter{},
			},
			service: resource{
				namespace: "unit",
				name:      "svc-a",
			},
			port:   intstr.FromString("port-b"),
			out:    0,
			exists: false,
			err:    fmt.Errorf("service 'unit/svc-a' missing port 'port-b'"),
		},
		"endpoints-do-not-exist": {
			tr: &syncTranslator{
				informers: informerset{
					endpoint: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("GetByKey", "unit/svc-a").Return(&v1.Endpoints{}, false, nil)
							return idx
						}())
						return i
					}(),
					service: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("GetByKey", "unit/svc-a").Return(&v1.Service{
								Spec: v1.ServiceSpec{
									Ports: []v1.ServicePort{
										{
											Name:     "port-a",
											Port:     8080,
											Protocol: v1.ProtocolTCP,
										},
									},
								},
							}, true, nil)
							return idx
						}())
						return i
					}(),
				},
				router: &mockTunnelRouter{},
			},
			service: resource{
				namespace: "unit",
				name:      "svc-a",
			},
			port:   intstr.FromInt(8080),
			out:    0,
			exists: false,
			err:    fmt.Errorf("endpoints 'unit/svc-a' do not exist"),
		},
		"endpoints-lookup-error": {
			tr: &syncTranslator{
				informers: informerset{
					endpoint: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("GetByKey", "unit/svc-a").Return(&v1.Endpoints{}, false, fmt.Errorf("lookup-error"))
							return idx
						}())
						return i
					}(),
					service: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("GetByKey", "unit/svc-a").Return(&v1.Service{
								Spec: v1.ServiceSpec{
									Ports: []v1.ServicePort{
										{
											Name:     "port-a",
											Port:     8080,
											Protocol: v1.ProtocolTCP,
										},
									},
								},
							}, true, nil)
							return idx
						}())
						return i
					}(),
				},
				router: &mockTunnelRouter{},
			},
			service: resource{
				namespace: "unit",
				name:      "svc-a",
			},
			port:   intstr.FromInt(8080),
			out:    0,
			exists: false,
			err:    fmt.Errorf("lookup-error"),
		},
		"endpoints-missing-subsets": {
			tr: &syncTranslator{
				informers: informerset{
					endpoint: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("GetByKey", "unit/svc-a").Return(&v1.Endpoints{
								Subsets: []v1.EndpointSubset{},
							}, true, nil)
							return idx
						}())
						return i
					}(),
					service: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("GetByKey", "unit/svc-a").Return(&v1.Service{
								Spec: v1.ServiceSpec{
									Ports: []v1.ServicePort{
										{
											Name:       "port-a",
											Port:       8080,
											TargetPort: intstr.FromInt(8080),
											Protocol:   v1.ProtocolTCP,
										},
									},
								},
							}, true, nil)
							return idx
						}())
						return i
					}(),
				},
				router: &mockTunnelRouter{},
			},
			service: resource{
				namespace: "unit",
				name:      "svc-a",
			},
			port:   intstr.FromInt(8080),
			out:    0,
			exists: false,
			err:    fmt.Errorf("endpoints 'unit/svc-a' missing subsets for port '8080'"),
		},
		"service-endpoints-okay": {
			tr: &syncTranslator{
				informers: informerset{
					endpoint: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("GetByKey", "unit/svc-a").Return(&v1.Endpoints{
								Subsets: []v1.EndpointSubset{
									{
										Ports: []v1.EndpointPort{
											{
												Name:     "port-a",
												Port:     9090,
												Protocol: v1.ProtocolTCP,
											},
										},
									},
								},
							}, true, nil)
							return idx
						}())
						return i
					}(),
					service: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("GetByKey", "unit/svc-a").Return(&v1.Service{
								Spec: v1.ServiceSpec{
									Ports: []v1.ServicePort{
										{
											Name:       "port-a",
											Port:       8080,
											TargetPort: intstr.FromInt(9090),
											Protocol:   v1.ProtocolTCP,
										},
									},
								},
							}, true, nil)
							return idx
						}())
						return i
					}(),
				},
				router: &mockTunnelRouter{},
			},
			service: resource{
				namespace: "unit",
				name:      "svc-a",
			},
			port:   intstr.FromInt(8080),
			out:    8080,
			exists: true,
			err:    nil,
		},
	} {
		out, exists, err := test.tr.getVerifiedPort(test.service.namespace, test.service.name, test.port)
		assert.Equalf(t, test.out, out, "test '%s' port mismatch", name)
		assert.Equalf(t, test.exists, exists, "test '%s' exists mismatch", name)
		assert.Equalf(t, test.err, err, "test '%s' error mismatch", name)
	}
}

func newMockedSyncTranslator() *syncTranslator {
	return &syncTranslator{
		informers: informerset{
			endpoint: func() cache.SharedIndexInformer {
				i := &mockSharedIndexInformer{}
				return i
			}(),
			ingress: func() cache.SharedIndexInformer {
				i := &mockSharedIndexInformer{}
				return i
			}(),
			secret: func() cache.SharedIndexInformer {
				i := &mockSharedIndexInformer{}
				return i
			}(),
			service: func() cache.SharedIndexInformer {
				i := &mockSharedIndexInformer{}
				return i
			}(),
		},
		router: func() tunnelRouter {
			r := &mockTunnelRouter{}
			return r
		}(),
	}
}

type mockTranslator struct {
	mock.Mock
}

func (t *mockTranslator) handleResource(kind, key string) error {
	args := t.Called(kind, key)
	return args.Error(0)
}
func (t *mockTranslator) waitForCacheSync(stopCh <-chan struct{}) bool {
	args := t.Called(stopCh)
	return args.Get(0).(bool)
}
func (t *mockTranslator) run(stopCh <-chan struct{}) error {
	args := t.Called(stopCh)
	return args.Error(0)
}
