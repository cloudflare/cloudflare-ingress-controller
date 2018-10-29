package argotunnel

import (
	"testing"

	"github.com/cloudflare/cloudflared/origin"
	"github.com/stretchr/testify/assert"
)

func TestTunnelLinkEquals(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		a   tunnelLink
		b   tunnelLink
		out bool
	}{
		"links-mismatched-host": {
			a: &syncTunnelLink{
				rule: tunnelRule{
					host: "a.unit.com",
				},
				cert: []byte("unit-cert"),
				opts: tunnelOptions{
					haConnections: 2,
				},
				config: &origin.TunnelConfig{
					OriginUrl: "unit.unit:8080",
				},
			},
			b: &syncTunnelLink{
				rule: tunnelRule{
					host: "b.unit.com",
				},
				cert: []byte("unit-cert"),
				opts: tunnelOptions{
					haConnections: 2,
				},
				config: &origin.TunnelConfig{
					OriginUrl: "unit.unit:8080",
				},
			},
			out: false,
		},
		"links-mismatched-cert": {
			a: &syncTunnelLink{
				rule: tunnelRule{
					host: "a.unit.com",
				},
				cert: []byte("unit-cert-a"),
				opts: tunnelOptions{
					haConnections: 2,
				},
				config: &origin.TunnelConfig{
					OriginUrl: "unit.unit:8080",
				},
			},
			b: &syncTunnelLink{
				rule: tunnelRule{
					host: "a.unit.com",
				},
				cert: []byte("unit-cert-b"),
				opts: tunnelOptions{
					haConnections: 2,
				},
				config: &origin.TunnelConfig{
					OriginUrl: "unit.unit:8080",
				},
			},
			out: false,
		},
		"links-mismatched-options": {
			a: &syncTunnelLink{
				rule: tunnelRule{
					host: "a.unit.com",
				},
				cert: []byte("unit-cert"),
				opts: tunnelOptions{
					haConnections: 2,
				},
				config: &origin.TunnelConfig{
					OriginUrl: "unit.unit:8080",
				},
			},
			b: &syncTunnelLink{
				rule: tunnelRule{
					host: "a.unit.com",
				},
				cert: []byte("unit-cert"),
				opts: tunnelOptions{
					haConnections: 4,
				},
				config: &origin.TunnelConfig{
					OriginUrl: "unit.unit:8080",
				},
			},
			out: false,
		},
		"links-mismatched-origin": {
			a: &syncTunnelLink{
				rule: tunnelRule{
					host: "a.unit.com",
				},
				cert: []byte("unit-cert"),
				opts: tunnelOptions{
					haConnections: 2,
				},
				config: &origin.TunnelConfig{
					OriginUrl: "unit.unit:8080",
				},
			},
			b: &syncTunnelLink{
				rule: tunnelRule{
					host: "b.unit.com",
				},
				cert: []byte("unit-cert"),
				opts: tunnelOptions{
					haConnections: 2,
				},
				config: &origin.TunnelConfig{
					OriginUrl: "unit.unit:8080",
				},
			},
			out: false,
		},
		"links-equal": {
			a: &syncTunnelLink{
				rule: tunnelRule{
					host: "a.unit.com",
				},
				cert: []byte("unit-cert"),
				opts: tunnelOptions{
					haConnections: 2,
				},
				config: &origin.TunnelConfig{
					OriginUrl: "unit.unit:8080",
				},
			},
			b: &syncTunnelLink{
				rule: tunnelRule{
					host: "a.unit.com",
				},
				cert: []byte("unit-cert"),
				opts: tunnelOptions{
					haConnections: 2,
				},
				config: &origin.TunnelConfig{
					OriginUrl: "unit.unit:8080",
				},
			},
			out: true,
		},
	} {
		out := test.a.equal(test.b)
		assert.Equalf(t, test.out, out, "test '%s' condition mismatch", name)
	}
}

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
