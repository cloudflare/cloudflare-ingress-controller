package controller

import (
	"testing"

	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"
)

func TestIngressClassAnnotation(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		in  *v1beta1.Ingress
		out string
	}{
		"empty-ingress": {
			in:  &v1beta1.Ingress{},
			out: "",
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
		},
	} {
		out, _ := parseIngressClass(test.in)
		assert.Equalf(t, test.out, out, "test '%s' value mismatch", name)
	}
}

func TestIngressLoadBalancerAnnotation(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		in  *v1beta1.Ingress
		out string
	}{
		"empty-ingress": {
			in:  &v1beta1.Ingress{},
			out: "",
		},
		"without-ingress-lb-pool": {
			in: &v1beta1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "extensions/v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					Annotations: map[string]string{
						annotationIngressLoadBalancer + "-without": "test",
					},
				},
			},
			out: "",
		},
		"with-ingress-lb-pool": {
			in: &v1beta1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "extensions/v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					Annotations: map[string]string{
						annotationIngressLoadBalancer: "test",
					},
				},
			},
			out: "test",
		},
	} {
		out, _ := parseIngressLoadBalancer(test.in)
		assert.Equalf(t, test.out, out, "test '%s' value mismatch", name)
	}
}
