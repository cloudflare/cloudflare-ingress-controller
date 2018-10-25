package argotunnel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetKindRuleResource(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		kind string
		rule tunnelRule
		out  *resource
	}{
		"empty-kind": {
			kind: "",
			rule: tunnelRule{},
			out:  nil,
		},
		"empty-rule": {
			kind: serviceKind,
			rule: tunnelRule{},
			out:  &resource{},
		},
		"unsupported-kind": {
			kind: "unsupported",
			rule: tunnelRule{
				host: "unit.com",
				port: 8080,
				service: resource{
					namespace: "svc-ns",
					name:      "svc-n",
				},
				secret: resource{
					namespace: "sec-ns",
					name:      "sec-n",
				},
			},
			out: nil,
		},
		"endpoint": {
			kind: endpointKind,
			rule: tunnelRule{
				host: "unit.com",
				port: 8080,
				service: resource{
					namespace: "svc-ns",
					name:      "svc-n",
				},
				secret: resource{
					namespace: "sec-ns",
					name:      "sec-n",
				},
			},
			out: &resource{
				namespace: "svc-ns",
				name:      "svc-n",
			},
		},
		"service": {
			kind: serviceKind,
			rule: tunnelRule{
				host: "unit.com",
				port: 8080,
				service: resource{
					namespace: "svc-ns",
					name:      "svc-n",
				},
				secret: resource{
					namespace: "sec-ns",
					name:      "sec-n",
				},
			},
			out: &resource{
				namespace: "svc-ns",
				name:      "svc-n",
			},
		},
		"secret": {
			kind: secretKind,
			rule: tunnelRule{
				host: "unit.com",
				port: 8080,
				service: resource{
					namespace: "svc-ns",
					name:      "svc-n",
				},
				secret: resource{
					namespace: "sec-ns",
					name:      "sec-n",
				},
			},
			out: &resource{
				namespace: "sec-ns",
				name:      "sec-n",
			},
		},
	} {
		out := getKindRuleResource(test.kind, test.rule)
		assert.Equalf(t, test.out, out, "test '%s' resource mismatch", name)
	}
}

type mockTunnelRouter struct {
	mock.Mock
}

func (r *mockTunnelRouter) updateRoute(newRoute *tunnelRoute) (err error) {
	args := r.Called(newRoute)
	return args.Error(0)
}
func (r *mockTunnelRouter) updateByKindRoutes(kind, namespace, name string, routes []*tunnelRoute) (err error) {
	args := r.Called(kind, namespace, name, routes)
	return args.Error(0)
}
func (r *mockTunnelRouter) deleteByRoute(namespace, name string) (err error) {
	args := r.Called(namespace, name)
	return args.Error(0)
}
func (r *mockTunnelRouter) deleteByKindKeys(kind, namespace, name string, keys []string) (err error) {
	args := r.Called(kind, namespace, name, keys)
	return args.Error(0)
}
func (r *mockTunnelRouter) run(stopCh <-chan struct{}) (err error) {
	args := r.Called(stopCh)
	return args.Error(0)
}
