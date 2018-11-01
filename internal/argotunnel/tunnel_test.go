package argotunnel

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"testing"
	"time"

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

func TestVerifyCertForHost(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		cert []byte
		host string
		err  error
	}{
		"cert-empty": {
			cert: []byte{},
			host: "",
			err:  fmt.Errorf("pem contains no certificate"),
		},
		"cert-host-mismatch": {
			cert: genCertforHost("host.unit.com"),
			host: "host.notforcert.com",
			err:  fmt.Errorf("x509: certificate is valid for host.unit.com, not host.notforcert.com"),
		},
		"cert-host-match": {
			cert: genCertforHost("host.unit.com"),
			host: "host.unit.com",
			err:  nil,
		},
	} {
		err := func() error {
			e := verifyCertForHost(test.cert, test.host)
			switch e.(type) {
			case x509.HostnameError:
				return fmt.Errorf(e.Error())
			default:
				return e
			}
		}()
		assert.Equalf(t, test.err, err, "test '%s' error mismatch", name)
	}
}

func genCertforHost(host string) (cert []byte) {
	template := x509.Certificate{
		SerialNumber: func() (n *big.Int) {
			list := new(big.Int).Lsh(big.NewInt(1), 128)
			n, err := rand.Int(rand.Reader, list)
			if err != nil {
				n = big.NewInt(0)
			}
			return
		}(),
		Subject: pkix.Name{
			Organization: []string{"Unit Co"},
		},
		DNSNames: []string{
			host,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(12 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	var pub interface{}
	var priv interface{}
	priv, _ = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	pub = &priv.(*ecdsa.PrivateKey).PublicKey

	rawBytes, _ := x509.CreateCertificate(rand.Reader, &template, &template, pub, priv)
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: rawBytes,
	})
	return pemBytes
}
