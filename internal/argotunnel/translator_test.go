package argotunnel

import (
	"fmt"
	"testing"

	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"

	/*
		"k8s.io/api/core/v1"
		"k8s.io/api/extensions/v1beta1"
		metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
		"k8s.io/apimachinery/pkg/util/intstr"
	*/
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

/*
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
		"ing-no-rules": {
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
				options:   collectTunnelOptions(parseIngressTunnelOptions(&v1beta1.Ingress{})),
				links:     tunnelRouteLinkMap{},
			},
		},
		"ing-rule-path": {
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
					TLS: []v1beta1.IngressTLS{},
					Rules: []v1beta1.IngressRule{
						{
							Host: "a.unit.com",
							IngressRuleValue: v1beta1.IngressRuleValue{
								HTTP: &v1beta1.HTTPIngressRuleValue{
									Paths: []v1beta1.HTTPIngressPath{
										{
											Path: "/unit",
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
				options:   collectTunnelOptions(parseIngressTunnelOptions(&v1beta1.Ingress{})),
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
										Addresses: []v1.EndpointAddress{
											{
												IP:       "x.x.x.x",
												Hostname: "a.unit.com",
											},
										},
									},
								},
							}, true, nil)
							return idx
						}())
						return i
					}(),
					ingress: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						return i
					}(),
					secret: func() cache.SharedIndexInformer {
						i := &mockSharedIndexInformer{}
						i.On("GetIndexer").Return(func() cache.Indexer {
							idx := &mockIndexer{}
							idx.On("GetByKey", "unit/sec-a").Return(&v1.Secret{
								Data: map[string][]byte{
									"cert.pem": []byte("sec-a-data"),
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
											Name: "http",
											Port: 8080,
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
				options:   collectTunnelOptions(parseIngressTunnelOptions(&v1beta1.Ingress{})),
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
*/

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
