package cloudflare

import (
	"testing"

	"crypto/x509"
	"github.com/stretchr/testify/assert"
)

func TestGetCloudflareRootCA(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		out *x509.CertPool
	}{
		"new-origin-cert": {
			out: func() *x509.CertPool {
				c := x509.NewCertPool()
				c.AppendCertsFromPEM([]byte(cloudflareRootCA))
				return c
			}(),
		},
	} {
		out := GetCloudflareRootCA()
		assert.Equalf(t, test.out, out, "test '%s' options mismatch", name)
	}
}
