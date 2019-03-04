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
	"github.com/cloudflare/cloudflared/tunnelrpc/pogs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

func TestParseTags(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		in  string
		n   int
		out []pogs.Tag
	}{
		"empty": {
			in:  "",
			n:   -1,
			out: []pogs.Tag{},
		},
		"with-no-pairs": {
			in:  "any-other-value",
			out: []pogs.Tag{},
			n:   -1,
		},
		"with-partial-pairs": {
			in: "key0=,=val1,,key2=val2,key3=",
			n:  -1,
			out: []pogs.Tag{
				{
					Name:  "key2",
					Value: "val2",
				},
			},
		},
		"with-pairs": {
			in: "key0=val0,key1=val1,key2=val=,,key3=val3",
			n:  -1,
			out: []pogs.Tag{
				{
					Name:  "key0",
					Value: "val0",
				},
				{
					Name:  "key1",
					Value: "val1",
				},
				{
					Name:  "key2",
					Value: "val=",
				},
				{
					Name:  "key3",
					Value: "val3",
				},
			},
		},
		"with-pairs-cap": {
			in: "key0=val0,key1=val1,key2=val=,,key3=val3",
			n:  2,
			out: []pogs.Tag{
				{
					Name:  "key0",
					Value: "val0",
				},
				{
					Name:  "key1",
					Value: "val1",
				},
			},
		},
	} {
		out := parseTags(test.in, test.n)
		assert.Equalf(t, test.out, out, "test '%s' value mismatch", name)
	}
}

func TestSetRepairBackoff(t *testing.T) {
	repairDelay := repairBackoff.delay
	repairJitter := repairBackoff.jitter
	repairSteps := repairBackoff.steps
	repairs := []struct {
		delay  time.Duration
		jitter float64
		steps  uint
	}{
		{
			delay:  1 * time.Millisecond,
			jitter: 0.25,
			steps:  1,
		},
		{
			delay:  10 * time.Millisecond,
			jitter: .125,
			steps:  2,
		},
		{
			delay:  100 * time.Millisecond,
			jitter: .0625,
			steps:  3,
		},
	}

	for _, r := range repairs {
		SetRepairBackoff(r.delay, r.jitter, r.steps)
	}

	assert.Equalf(t, repairs[0].delay, repairBackoff.delay, "test repair delay matches first set")
	assert.Equalf(t, repairs[0].jitter, repairBackoff.jitter, "test repair jitter matches first set")
	assert.Equalf(t, repairs[0].steps, repairBackoff.steps, "test repair steps matches first set")
	assert.NotEqualf(t, repairBackoff.delay, repairDelay, "test repair delay does not match default")
	assert.NotEqualf(t, repairBackoff.jitter, repairJitter, "test repair jitter does not match default")
	assert.NotEqualf(t, repairBackoff.steps, repairSteps, "test repair steps does not match default")
}

func TestRepairDelay(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		step   uint
		delay  time.Duration
		jitter float64
		steps  uint
		out    time.Duration
	}{
		"step-0-no-jitter": {
			step:   0,
			delay:  10 * time.Millisecond,
			jitter: 0.0,
			steps:  4,
			out:    10 * time.Millisecond,
		},
		"step-0-no-jitter-no-steps": {
			step:   0,
			delay:  10 * time.Millisecond,
			jitter: 0.0,
			steps:  0,
			out:    10 * time.Millisecond,
		},
		"step-1-no-jitter": {
			step:   1,
			delay:  10 * time.Millisecond,
			jitter: 0.0,
			steps:  4,
			out:    20 * time.Millisecond,
		},
		"step-2-no-jitter": {
			step:   2,
			delay:  10 * time.Millisecond,
			jitter: 0.0,
			steps:  4,
			out:    40 * time.Millisecond,
		},
		"step-4-no-jitter": {
			step:   4,
			delay:  10 * time.Millisecond,
			jitter: 0.0,
			steps:  4,
			out:    10 * time.Millisecond,
		},
	} {
		out := repairDelay(test.step, test.delay, test.jitter, test.steps)
		assert.Equalf(t, test.out, out, "test '%s' value mismatch", name)
	}
}

func TestSetTagLimit(t *testing.T) {
	tagLimit := tagConfig.limit
	limits := []int{
		128,
		64,
		32,
		16,
	}

	for _, n := range limits {
		SetTagLimit(n)
	}

	assert.Equalf(t, limits[0], tagConfig.limit, "test tag limit matches first set")
	assert.NotEqualf(t, tagConfig.limit, tagLimit, "test repair delay does not match default")
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

type mockTunnelLink struct {
	mock.Mock
}

func (l *mockTunnelLink) host() string {
	args := l.Called()
	return args.Get(0).(string)
}
func (l *mockTunnelLink) routeRule() tunnelRule {
	args := l.Called()
	return args.Get(0).(tunnelRule)
}
func (l *mockTunnelLink) originURL() string {
	args := l.Called()
	return args.Get(0).(string)
}
func (l *mockTunnelLink) originCert() []byte {
	args := l.Called()
	return args.Get(0).([]byte)
}
func (l *mockTunnelLink) options() tunnelOptions {
	args := l.Called()
	return args.Get(0).(tunnelOptions)
}
func (l *mockTunnelLink) equal(obj tunnelLink) bool {
	args := l.Called(obj)
	return args.Get(0).(bool)
}
func (l *mockTunnelLink) start() error {
	args := l.Called()
	return args.Error(0)
}
func (l *mockTunnelLink) stop() error {
	args := l.Called()
	return args.Error(0)
}
