package argotunnel

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/cache"
)

func TestIngressSecretIndexFunc(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		obj interface{}
		out []string
		err error
	}{
		"obj-nil": {
			obj: nil,
			out: []string{},
			err: fmt.Errorf("index unexpected obj type: %T", nil),
		},
		"obj-not-ing": {
			obj: &unit{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unit",
					Namespace: "unit",
				},
			},
			out: []string{},
			err: fmt.Errorf("index unexpected obj type: %T", &unit{}),
		},
		"obj-ing-class-mismatch": {
			obj: &v1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unit",
					Namespace: "unit",
					Annotations: map[string]string{
						"kubernetes.io/ingress.class": "not-unit",
					},
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
								HTTP: &v1beta1.HTTPIngressRuleValue{},
							},
						},
					},
				},
			},
			out: nil,
			err: nil,
		},
		"obj-ing-secs": {
			obj: &v1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unit",
					Namespace: "unit",
					Annotations: map[string]string{
						"kubernetes.io/ingress.class": "unit",
					},
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
						{
							Hosts: []string{
								"c.unit.com",
							},
							SecretName: "sec-c",
						},
						{
							Hosts: []string{
								"f.unit.com",
							},
							SecretName: "sec-f",
						},
					},
					Rules: []v1beta1.IngressRule{
						{
							Host: "a.unit.com",
							IngressRuleValue: v1beta1.IngressRuleValue{
								HTTP: &v1beta1.HTTPIngressRuleValue{},
							},
						},
						{
							Host: "b.unit.com",
							IngressRuleValue: v1beta1.IngressRuleValue{
								HTTP: &v1beta1.HTTPIngressRuleValue{},
							},
						},
						{
							Host: "c.unit.com",
							IngressRuleValue: v1beta1.IngressRuleValue{
								HTTP: &v1beta1.HTTPIngressRuleValue{},
							},
						},
						{
							Host: "d.unit.com",
							IngressRuleValue: v1beta1.IngressRuleValue{
								HTTP: &v1beta1.HTTPIngressRuleValue{},
							},
						},
					},
				},
			},
			out: []string{
				"unit/sec-a",
				"unit/sec-c",
			},
			err: nil,
		},
	} {
		indexFunc := ingressSecretIndexFunc("unit", nil, nil, nil)
		out, err := indexFunc(test.obj)
		assert.Equalf(t, test.out, out, "test '%s' index mismatch", name)
		assert.Equalf(t, test.err, err, "test '%s' error mismatch", name)
	}
}

func TestIngressServiceIndexFunc(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		obj interface{}
		out []string
		err error
	}{
		"obj-nil": {
			obj: nil,
			out: []string{},
			err: fmt.Errorf("index unexpected obj type: %T", nil),
		},
		"obj-not-ing": {
			obj: &unit{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unit",
					Namespace: "unit",
				},
			},
			out: []string{},
			err: fmt.Errorf("index unexpected obj type: %T", &unit{}),
		},
		"obj-ing-class-mismatch": {
			obj: &v1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unit",
					Namespace: "unit",
					Annotations: map[string]string{
						"kubernetes.io/ingress.class": "not-unit",
					},
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "v1beta1",
				},
				Spec: v1beta1.IngressSpec{
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
			out: nil,
			err: nil,
		},
		"obj-ing-svcs": {
			obj: &v1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unit",
					Namespace: "unit",
					Annotations: map[string]string{
						"kubernetes.io/ingress.class": "unit",
					},
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "v1beta1",
				},
				Spec: v1beta1.IngressSpec{
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
										{
											Backend: v1beta1.IngressBackend{
												ServiceName: "",
												ServicePort: intstr.FromString("http"),
											},
										},
										{
											Backend: v1beta1.IngressBackend{
												ServiceName: "svc-c",
												ServicePort: intstr.FromString("http"),
											},
										},
									},
								},
							},
						},
						{
							Host: "b.unit.com",
							IngressRuleValue: v1beta1.IngressRuleValue{
								HTTP: &v1beta1.HTTPIngressRuleValue{
									Paths: []v1beta1.HTTPIngressPath{
										{
											Backend: v1beta1.IngressBackend{
												ServiceName: "svc-d",
												ServicePort: intstr.FromString("http"),
											},
										},
										{
											Backend: v1beta1.IngressBackend{
												ServiceName: "",
												ServicePort: intstr.FromString("http"),
											},
										},
										{
											Backend: v1beta1.IngressBackend{
												ServiceName: "svc-f",
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
			out: []string{
				"unit/svc-a",
				"unit/svc-c",
				"unit/svc-d",
				"unit/svc-f",
			},
			err: nil,
		},
	} {
		indexFunc := ingressServiceIndexFunc("unit")
		out, err := indexFunc(test.obj)
		assert.Equalf(t, test.out, out, "test '%s' index mismatch", name)
		assert.Equalf(t, test.err, err, "test '%s' error mismatch", name)
	}
}

func TestGetDomainSecret(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		host    string
		secrets map[string]*resource
		out     *resource
		ok      bool
	}{
		"empty-host": {
			host: "",
			secrets: map[string]*resource{
				"test.com": {
					name:      "test-a",
					namespace: "test-a",
				},
			},
			out: nil,
			ok:  false,
		},
		"empty-secrets": {
			host:    "a.test.com",
			secrets: nil,
			out:     nil,
			ok:      false,
		},
		"no-match": {
			host: "a.unit.com",
			secrets: map[string]*resource{
				"test.com": {
					name:      "test-a",
					namespace: "test-a",
				},
			},
			out: nil,
			ok:  false,
		},
		"match": {
			host: "a.test.com",
			secrets: map[string]*resource{
				"test.com": {
					name:      "test-a",
					namespace: "test-a",
				},
			},
			out: &resource{
				name:      "test-a",
				namespace: "test-a",
			},
			ok: true,
		},
	} {
		out, ok := getDomainSecret(test.host, test.secrets)
		assert.Equalf(t, test.out, out, "test '%s' secret mismatch", name)
		assert.Equalf(t, test.ok, ok, "test '%s' condition mismatch", name)
	}
}

func TestParseDomain(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		host   string
		domain string
		ok     bool
	}{
		"empty": {
			host:   "",
			domain: "",
			ok:     false,
		},
		"missing-hostname": {
			host:   ".test.io",
			domain: "",
			ok:     false,
		},
		"missing-domain": {
			host:   "a.",
			domain: "",
			ok:     false,
		},
		"2-part": {
			host:   "a.test.io",
			domain: "test.io",
			ok:     true,
		},
		"n-part": {
			host:   "a.b.c.d.e.test.io",
			domain: "b.c.d.e.test.io",
			ok:     true,
		},
	} {
		domain, ok := parseDomain(test.host)
		assert.Equalf(t, test.domain, domain, "test '%s' domain mismatch", name)
		assert.Equalf(t, test.ok, ok, "test '%s' condition mismatch", name)
	}
}

func TestItemKeyFunc(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		namespace string
		name      string
		key       string
	}{
		"empty": {
			namespace: "",
			name:      "",
			key:       "/",
		},
		"no-name": {
			namespace: "unit-ns",
			name:      "",
			key:       "unit-ns/",
		},
		"no-namespace": {
			namespace: "",
			name:      "unit-n",
			key:       "/unit-n",
		},
		"okay": {
			namespace: "unit-ns",
			name:      "unit-n",
			key:       "unit-ns/unit-n",
		},
	} {
		key := itemKeyFunc(test.namespace, test.name)
		assert.Equalf(t, test.key, key, "test '%s' key mismatch", name)
	}
}

type mockSharedIndexInformer struct {
	mock.Mock
}

func (i *mockSharedIndexInformer) AddEventHandler(handler cache.ResourceEventHandler) {
	i.Called(handler)
}
func (i *mockSharedIndexInformer) AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, resyncPeriod time.Duration) {
	i.Called(handler, resyncPeriod)
}
func (i *mockSharedIndexInformer) GetStore() cache.Store {
	args := i.Called()
	return args.Get(0).(cache.Store)
}
func (i *mockSharedIndexInformer) GetController() cache.Controller {
	args := i.Called()
	return args.Get(0).(cache.Controller)
}
func (i *mockSharedIndexInformer) Run(stopCh <-chan struct{}) {
	i.Called(stopCh)
}
func (i *mockSharedIndexInformer) HasSynced() bool {
	args := i.Called()
	return args.Get(0).(bool)
}
func (i *mockSharedIndexInformer) LastSyncResourceVersion() string {
	args := i.Called()
	return args.Get(0).(string)
}
func (i *mockSharedIndexInformer) AddIndexers(indexers cache.Indexers) error {
	args := i.Called(indexers)
	return args.Error(0)
}
func (i *mockSharedIndexInformer) GetIndexer() cache.Indexer {
	args := i.Called()
	return args.Get(0).(cache.Indexer)
}

type mockIndexer struct {
	mock.Mock
}

func (i *mockIndexer) Add(obj interface{}) error {
	args := i.Called(obj)
	return args.Error(0)
}
func (i *mockIndexer) Update(obj interface{}) error {
	args := i.Called(obj)
	return args.Error(0)
}
func (i *mockIndexer) Delete(obj interface{}) error {
	args := i.Called(obj)
	return args.Error(0)
}
func (i *mockIndexer) List() []interface{} {
	args := i.Called()
	return args.Get(0).([]interface{})
}
func (i *mockIndexer) ListKeys() []string {
	args := i.Called()
	return args.Get(0).([]string)
}
func (i *mockIndexer) Get(obj interface{}) (item interface{}, exists bool, err error) {
	args := i.Called(obj)
	return args.Get(0).(interface{}), args.Get(1).(bool), args.Error(2)
}
func (i *mockIndexer) GetByKey(key string) (item interface{}, exists bool, err error) {
	args := i.Called(key)
	return args.Get(0).(interface{}), args.Get(1).(bool), args.Error(2)
}
func (i *mockIndexer) Replace(a []interface{}, b string) error {
	args := i.Called(a, b)
	return args.Error(0)
}
func (i *mockIndexer) Resync() error {
	args := i.Called()
	return args.Error(0)
}
func (i *mockIndexer) Index(indexName string, obj interface{}) ([]interface{}, error) {
	args := i.Called(indexName, obj)
	return args.Get(0).([]interface{}), args.Error(1)
}
func (i *mockIndexer) IndexKeys(indexName, indexKey string) ([]string, error) {
	args := i.Called(indexName, indexKey)
	return args.Get(0).([]string), args.Error(1)
}
func (i *mockIndexer) ListIndexFuncValues(indexName string) []string {
	args := i.Called(indexName)
	return args.Get(0).([]string)
}
func (i *mockIndexer) ByIndex(indexName, indexKey string) ([]interface{}, error) {
	args := i.Called(indexName, indexKey)
	return args.Get(0).([]interface{}), args.Error(1)
}
func (i *mockIndexer) GetIndexers() cache.Indexers {
	args := i.Called()
	return args.Get(0).(cache.Indexers)
}
func (i *mockIndexer) AddIndexers(newIndexers cache.Indexers) error {
	args := i.Called(newIndexers)
	return args.Error(0)
}
