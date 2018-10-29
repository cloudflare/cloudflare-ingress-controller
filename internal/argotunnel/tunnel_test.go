package argotunnel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetOriginUrl(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		rule tunnelRule
		url  string
	}{
		"empty-rule": {
			rule: tunnelRule{},
			url:  ".:0",
		},
		"no-service-namespace": {
			rule: tunnelRule{
				service: resource{
					name: "unit-n",
				},
				port: 8080,
			},
			url: "unit-n.:8080",
		},
		"no-service-name": {
			rule: tunnelRule{
				service: resource{
					namespace: "unit-ns",
				},
				port: 8080,
			},
			url: ".unit-ns:8080",
		},
		"okay": {
			rule: tunnelRule{
				service: resource{
					namespace: "unit-ns",
					name:      "unit-n",
				},
				port: 8080,
			},
			url: "unit-n.unit-ns:8080",
		},
	} {
		url := getOriginURL(test.rule)
		assert.Equalf(t, test.url, url, "test '%s' url mismatch", name)
	}
}
