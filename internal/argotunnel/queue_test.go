package argotunnel

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type unit struct {
	metav1.ObjectMeta
}

func TestEndpointFilterFunc(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		obj interface{}
		out bool
	}{
		"obj-nil": {
			obj: nil,
			out: false,
		},
		"obj-not-ep": {
			obj: &unit{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unit",
					Namespace: "unit",
				},
			},
			out: false,
		},
		"obj-ep-no-subsets": {
			obj: &v1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unit",
					Namespace: "unit",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Endpoints",
					APIVersion: "v1",
				},
				Subsets: []v1.EndpointSubset{},
			},
			out: false,
		},
		"obj-ep-with-subsets": {
			obj: &v1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unit",
					Namespace: "unit",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Endpoints",
					APIVersion: "v1",
				},
				Subsets: []v1.EndpointSubset{
					{
						Addresses: []v1.EndpointAddress{
							{
								IP:       "1.1.1.1",
								Hostname: "unit.com",
							},
						},
						Ports: []v1.EndpointPort{
							{
								Name:     "unit",
								Port:     8080,
								Protocol: "TCP",
							},
						},
					},
				},
			},
			out: true,
		},
	} {
		filterFunc := endpointFilterFunc()
		out := filterFunc(test.obj)
		assert.Equalf(t, test.out, out, "test '%s' condition mismatch", name)
	}
}

func TestIngressFilterFunc(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		obj      interface{}
		ingclass string
		out      bool
	}{
		"obj-nil": {
			ingclass: "",
			obj:      nil,
			out:      false,
		},
		"obj-not-ing": {
			ingclass: "",
			obj: &unit{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unit",
					Namespace: "unit",
				},
			},
			out: false,
		},
		"obj-ing-no-class": {
			ingclass: "",
			obj: &v1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unit",
					Namespace: "unit",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "v1beta1",
				},
			},
			out: false,
		},
		"obj-ing-mismatch-class": {
			ingclass: "unit",
			obj: &v1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unit",
					Namespace: "unit",
					Annotations: map[string]string{
						annotationIngressClass: "other",
					},
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "v1beta1",
				},
			},
			out: false,
		},
		"obj-ing-match-class": {
			ingclass: "unit",
			obj: &v1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unit",
					Namespace: "unit",
					Annotations: map[string]string{
						annotationIngressClass: "unit",
					},
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "v1beta1",
				},
			},
			out: true,
		},
	} {
		filterFunc := ingressFilterFunc(test.ingclass)
		out := filterFunc(test.obj)
		assert.Equalf(t, test.out, out, "test '%s' condition mismatch", name)
	}
}

func TestSecretFilterFunc(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		obj interface{}
		out bool
	}{
		"obj-nil": {
			obj: nil,
			out: false,
		},
		"obj-not-secret": {
			obj: &unit{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unit",
					Namespace: "unit",
				},
			},
			out: false,
		},
		"obj-secret-no-cert": {
			obj: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unit",
					Namespace: "unit",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				Data: map[string][]byte{
					"not-cert": []byte("fake-cert"),
				},
			},
			out: false,
		},
		"obj-secret-with-cert": {
			obj: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unit",
					Namespace: "unit",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				Data: map[string][]byte{
					"cert.pem": []byte("fake-cert"),
				},
			},
			out: true,
		},
	} {
		filterFunc := secretFilterFunc()
		out := filterFunc(test.obj)
		assert.Equalf(t, test.out, out, "test '%s' condition mismatch", name)
	}
}

func TestServiceFilterFunc(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		obj interface{}
		out bool
	}{
		"obj-nil": {
			obj: nil,
			out: false,
		},
		"obj-not-secret": {
			obj: &unit{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unit",
					Namespace: "unit",
				},
			},
			out: false,
		},
		"obj-service-no-ports": {
			obj: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unit",
					Namespace: "unit",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Service",
					APIVersion: "v1",
				},
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{},
				},
			},
			out: false,
		},
		"obj-service-with-ports": {
			obj: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unit",
					Namespace: "unit",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Service",
					APIVersion: "v1",
				},
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Name: "http",
							Port: 8080,
						},
					},
				},
			},
			out: true,
		},
	} {
		filterFunc := serviceFilterFunc()
		out := filterFunc(test.obj)
		assert.Equalf(t, test.out, out, "test '%s' condition mismatch", name)
	}
}

func TestResourceKeyFunc(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		in  interface{}
		out string
		err error
	}{
		"obj-no-metaobj": {
			in: &metav1.TypeMeta{
				Kind:       "unit",
				APIVersion: "none",
			},
			out: "",
			err: fmt.Errorf("object has no meta: object does not implement the Object interfaces"),
		},
		"obj-partial": {
			in: &unit{
				ObjectMeta: metav1.ObjectMeta{
					Name: "a",
				},
			},
			out: "unit/a",
			err: nil,
		},
		"obj-complete": {
			in: &unit{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "a",
					Namespace: "b",
				},
			},
			out: "unit/b/a",
			err: nil,
		},
	} {
		out, err := resourceKeyFunc("unit", test.in)
		assert.Equalf(t, test.out, out, "test '%s' value mismatch", name)
		assert.Equalf(t, test.err, err, "test '%s' error mismatch", name)
	}
}

func TestSplitResourceKey(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		in   string
		kind string
		ns   string
		n    string
		err  error
	}{
		"key-too-many": {
			in:   "a/b/c/d",
			kind: "",
			ns:   "",
			n:    "",
			err:  fmt.Errorf("unexpected key format: %q", "a/b/c/d"),
		},
		"key-too-few": {
			in:   "a",
			kind: "",
			ns:   "",
			n:    "",
			err:  fmt.Errorf("unexpected key format: %q", "a"),
		},
		"key-partial": {
			in:   "a/c",
			kind: "a",
			ns:   "",
			n:    "c",
			err:  nil,
		},
		"key-complete": {
			in:   "a/b/c",
			kind: "a",
			ns:   "b",
			n:    "c",
			err:  nil,
		},
	} {
		kind, ns, n, err := splitResourceKey(test.in)
		assert.Equalf(t, test.kind, kind, "test '%s' kind mismatch", name)
		assert.Equalf(t, test.ns, ns, "test '%s' namespace mismatch", name)
		assert.Equalf(t, test.n, n, "test '%s' name mismatch", name)
		assert.Equalf(t, test.err, err, "test '%s' error mismatch", name)
	}
}

func TestSplitKindMetaKey(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		in   string
		kind string
		meta string
		err  error
	}{
		"key-too-few": {
			in:   "a",
			kind: "",
			meta: "",
			err:  fmt.Errorf("unexpected key format: %q", "a"),
		},
		"key-complete": {
			in:   "a/b/c",
			kind: "a",
			meta: "b/c",
			err:  nil,
		},
	} {
		kind, meta, err := splitKindMetaKey(test.in)
		assert.Equalf(t, test.kind, kind, "test '%s' kind mismatch", name)
		assert.Equalf(t, test.meta, meta, "test '%s' meta mismatch", name)
		assert.Equalf(t, test.err, err, "test '%s' error mismatch", name)
	}
}
