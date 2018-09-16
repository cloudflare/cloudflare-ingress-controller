package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOptions(t *testing.T) {
	for name, test := range map[string]struct {
		in  []Option
		out options
	}{
		"default-options": {
			in: []Option{},
			out: options{
				ingressClass:    IngressClassDefault,
				secretName:      SecretNameDefault,
				secretNamespace: SecretNamespaceDefault,
			},
		},
		"set-one-option": {
			in: []Option{
				IngressClass("test-class"),
			},
			out: options{
				ingressClass:    "test-class",
				secretName:      SecretNameDefault,
				secretNamespace: SecretNamespaceDefault,
			},
		},
		"set-all-options": {
			in: []Option{
				IngressClass("test-class"),
				SecretName("test-secret-name"),
				SecretNamespace("test-secret-namespace"),
			},
			out: options{
				ingressClass:    "test-class",
				secretName:      "test-secret-name",
				secretNamespace: "test-secret-namespace",
			},
		},
	} {
		out := collectOptions(test.in)
		assert.Equalf(t, test.out, out, "test '%s' options mismatch", name)
	}
}
