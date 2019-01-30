package argotunnel

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestParseIngressClassAnnotation(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		in  *v1beta1.Ingress
		out string
		ok  bool
	}{
		"empty-ingress": {
			in:  &v1beta1.Ingress{},
			out: "",
			ok:  false,
		},
		"without-ingress-class": {
			in: &v1beta1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "extensions/v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					Annotations: map[string]string{
						annotationIngressClass + "-without": "test",
					},
				},
			},
			out: "",
			ok:  false,
		},
		"with-ingress-class": {
			in: &v1beta1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "extensions/v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					Annotations: map[string]string{
						annotationIngressClass: "test",
					},
				},
			},
			out: "test",
			ok:  true,
		},
	} {
		out, ok := parseIngressClass(test.in)
		assert.Equalf(t, test.out, out, "test '%s' value mismatch", name)
		assert.Equalf(t, test.ok, ok, "test '%s' found mismatch", name)
	}
}

func TestParseIngressTunnelOptions(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		in  *v1beta1.Ingress
		out tunnelOptions
	}{
		"empty-ingress": {
			in:  &v1beta1.Ingress{},
			out: collectTunnelOptions(nil),
		},
		"without-options": {
			in: &v1beta1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "extensions/v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					Annotations: map[string]string{
						annotationIngressClass: "test",
					},
				},
			},
			out: collectTunnelOptions(nil),
		},
		"without-some-options": {
			in: &v1beta1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "extensions/v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					Annotations: map[string]string{
						annotationIngressClass:         "test",
						annotationIngressHAConnections: "2",
						annotationIngressLoadBalancer:  "test-lb-pool",
						annotationIngressRetries:       "8",
					},
				},
			},
			out: tunnelOptions{
				haConnections:     2,
				heartbeatCount:    5,
				heartbeatInterval: 5 * time.Second,
				lbPool:            "test-lb-pool",
				retries:           8,
			},
		},
		"with-all-options": {
			in: &v1beta1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "extensions/v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					Annotations: map[string]string{
						annotationIngressClass:              "test",
						annotationIngressCompressionQuality: "1",
						annotationIngressHAConnections:      "2",
						annotationIngressHeartbeatCount:     "4",
						annotationIngressHeartbeatInterval:  "4ms",
						annotationIngressLoadBalancer:       "test-lb-pool",
						annotationIngressNoChunkedEncoding:  "true",
						annotationIngressRetries:            "8",
						annotationIngressTag:                "key1=val1"},
				},
			},
			out: tunnelOptions{
				compressionQuality: 1,
				haConnections:      2,
				heartbeatCount:     4,
				heartbeatInterval:  4 * time.Millisecond,
				lbPool:             "test-lb-pool",
				noChunkedEncoding:  true,
				retries:            8,
				tags:               "key1=val1",
			},
		},
	} {
		out := collectTunnelOptions(parseIngressTunnelOptions(test.in))
		assert.Equalf(t, test.out, out, "test '%s' value mismatch", name)
	}
}

func TestParseMetaBool(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		in  *v1beta1.Ingress
		out bool
		ok  bool
	}{
		"empty-meta": {
			in:  &v1beta1.Ingress{},
			out: false,
			ok:  false,
		},
		"with-any-other-value": {
			in: &v1beta1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "extensions/v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					Annotations: map[string]string{
						"test": "any-other-value",
					},
				},
			},
			out: false,
			ok:  false,
		},
		"with-false": {
			in: &v1beta1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "extensions/v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					Annotations: map[string]string{
						"test": "false",
					},
				},
			},
			out: false,
			ok:  true,
		},
		"with-true": {
			in: &v1beta1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "extensions/v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					Annotations: map[string]string{
						"test": "true",
					},
				},
			},
			out: true,
			ok:  true,
		},
	} {
		obj, _ := meta.Accessor(test.in)
		out, ok := parseMetaBool(obj, "test")
		assert.Equalf(t, test.out, out, "test '%s' value mismatch", name)
		assert.Equalf(t, test.ok, ok, "test '%s' found mismatch", name)
	}
}

func TestParseMetaDuration(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		in  *v1beta1.Ingress
		out time.Duration
		ok  bool
	}{
		"empty-ingress": {
			in:  &v1beta1.Ingress{},
			out: 0,
			ok:  false,
		},
		"with-non-duration": {
			in: &v1beta1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "extensions/v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					Annotations: map[string]string{
						"test": "not-a-duration",
					},
				},
			},
			out: 0,
			ok:  false,
		},
		"with-duration": {
			in: &v1beta1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "extensions/v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					Annotations: map[string]string{
						"test": "100ms",
					},
				},
			},
			out: 100 * time.Millisecond,
			ok:  true,
		},
	} {
		obj, _ := meta.Accessor(test.in)
		out, ok := parseMetaDuration(obj, "test")
		assert.Equalf(t, test.out, out, "test '%s' value mismatch", name)
		assert.Equalf(t, test.ok, ok, "test '%s' found mismatch", name)
	}
}

func TestParseMetaInt(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		in  *v1beta1.Ingress
		out uint
		ok  bool
	}{
		"empty-ingress": {
			in:  &v1beta1.Ingress{},
			out: 0,
			ok:  false,
		},
		"with-non-int": {
			in: &v1beta1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "extensions/v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					Annotations: map[string]string{
						"test": "non-numeric",
					},
				},
			},
			out: 0,
			ok:  false,
		},
		"with-int": {
			in: &v1beta1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "extensions/v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					Annotations: map[string]string{
						"test": "9",
					},
				},
			},
			out: 9,
			ok:  true,
		},
	} {
		obj, _ := meta.Accessor(test.in)
		out, ok := parseMetaUint(obj, "test")
		assert.Equalf(t, test.out, out, "test '%s' value mismatch", name)
		assert.Equalf(t, test.ok, ok, "test '%s' found mismatch", name)
	}
}

func TestParseMetaUint(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		in  *v1beta1.Ingress
		out uint
		ok  bool
	}{
		"empty-ingress": {
			in:  &v1beta1.Ingress{},
			out: 0,
			ok:  false,
		},
		"with-non-uint": {
			in: &v1beta1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "extensions/v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					Annotations: map[string]string{
						"test": "-9",
					},
				},
			},
			out: 0,
			ok:  false,
		},
		"with-uint": {
			in: &v1beta1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "extensions/v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					Annotations: map[string]string{
						"test": "9",
					},
				},
			},
			out: 9,
			ok:  true,
		},
	} {
		obj, _ := meta.Accessor(test.in)
		out, ok := parseMetaUint(obj, "test")
		assert.Equalf(t, test.out, out, "test '%s' value mismatch", name)
		assert.Equalf(t, test.ok, ok, "test '%s' found mismatch", name)
	}
}

func TestParseMetaUint64(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		in  *v1beta1.Ingress
		out uint
		ok  bool
	}{
		"empty-ingress": {
			in:  &v1beta1.Ingress{},
			out: 0,
			ok:  false,
		},
		"with-non-uint64": {
			in: &v1beta1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "extensions/v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					Annotations: map[string]string{
						"test": "-8",
					},
				},
			},
			out: 0,
			ok:  false,
		},
		"with-uint64": {
			in: &v1beta1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "extensions/v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					Annotations: map[string]string{
						"test": "8",
					},
				},
			},
			out: 8,
			ok:  true,
		},
	} {
		obj, _ := meta.Accessor(test.in)
		out, ok := parseMetaUint(obj, "test")
		assert.Equalf(t, test.out, out, "test '%s' value mismatch", name)
		assert.Equalf(t, test.ok, ok, "test '%s' found mismatch", name)
	}
}
