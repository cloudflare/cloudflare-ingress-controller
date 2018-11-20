package argotunnel

import (
	"testing"

	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestUpdateRoute(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		router *syncTunnelRouter
		route  *tunnelRoute
		items  map[string]*tunnelRoute
		err    error
	}{
		"router-add-route": {
			router: &syncTunnelRouter{
				items: map[string]*tunnelRoute{},
			},
			route: &tunnelRoute{
				namespace: "unit",
				name:      "a",
				links: tunnelRouteLinkMap{
					tunnelRule{port: 8080}: func() tunnelLink {
						l := &mockTunnelLink{}
						l.On("start").Return(nil)
						return l
					}(),
				},
			},
			items: map[string]*tunnelRoute{
				"unit/a": {
					namespace: "unit",
					name:      "a",
					links: tunnelRouteLinkMap{
						tunnelRule{port: 8080}: nil,
					},
				},
			},
			err: nil,
		},
		"router-add-route-link": {
			router: &syncTunnelRouter{
				items: map[string]*tunnelRoute{
					"unit/a": {
						namespace: "unit",
						name:      "a",
						links: tunnelRouteLinkMap{
							tunnelRule{port: 8080}: func() tunnelLink {
								l := &mockTunnelLink{}
								l.On("equal", mock.Anything).Return(true)
								return l
							}(),
						},
					},
				},
			},
			route: &tunnelRoute{
				namespace: "unit",
				name:      "a",
				links: tunnelRouteLinkMap{
					tunnelRule{port: 8080}: &mockTunnelLink{},
					tunnelRule{port: 8081}: func() tunnelLink {
						l := &mockTunnelLink{}
						l.On("start").Return(nil)
						return l
					}(),
				},
			},
			items: map[string]*tunnelRoute{
				"unit/a": {
					namespace: "unit",
					name:      "a",
					links: tunnelRouteLinkMap{
						tunnelRule{port: 8080}: nil,
						tunnelRule{port: 8081}: nil,
					},
				},
			},
			err: nil,
		},
		"router-delete-route-link": {
			router: &syncTunnelRouter{
				items: map[string]*tunnelRoute{
					"unit/a": {
						namespace: "unit",
						name:      "a",
						links: tunnelRouteLinkMap{
							tunnelRule{port: 8080}: func() tunnelLink {
								l := &mockTunnelLink{}
								l.On("equal", mock.Anything).Return(true)
								return l
							}(),
							tunnelRule{port: 8081}: func() tunnelLink {
								l := &mockTunnelLink{}
								l.On("stop").Return(nil)
								return l
							}(),
						},
					},
				},
			},
			route: &tunnelRoute{
				namespace: "unit",
				name:      "a",
				links: tunnelRouteLinkMap{
					tunnelRule{port: 8080}: &mockTunnelLink{},
					tunnelRule{port: 8082}: func() tunnelLink {
						l := &mockTunnelLink{}
						l.On("start").Return(nil)
						return l
					}(),
				},
			},
			items: map[string]*tunnelRoute{
				"unit/a": {
					namespace: "unit",
					name:      "a",
					links: tunnelRouteLinkMap{
						tunnelRule{port: 8080}: nil,
						tunnelRule{port: 8082}: nil,
					},
				},
			},
			err: nil,
		},
		"router-update-route-link": {
			router: &syncTunnelRouter{
				items: map[string]*tunnelRoute{
					"unit/a": {
						namespace: "unit",
						name:      "a",
						links: tunnelRouteLinkMap{
							tunnelRule{port: 8080}: func() tunnelLink {
								l := &mockTunnelLink{}
								l.On("equal", mock.Anything).Return(false)
								l.On("stop").Return(nil)
								return l
							}(),
						},
					},
				},
			},
			route: &tunnelRoute{
				namespace: "unit",
				name:      "a",
				links: tunnelRouteLinkMap{
					tunnelRule{port: 8080}: func() tunnelLink {
						l := &mockTunnelLink{}
						l.On("start").Return(nil)
						return l
					}(),
				},
			},
			items: map[string]*tunnelRoute{
				"unit/a": {
					namespace: "unit",
					name:      "a",
					links: tunnelRouteLinkMap{
						tunnelRule{port: 8080}: nil,
					},
				},
			},
			err: nil,
		},
	} {
		logger, hook := logtest.NewNullLogger()
		test.router.log = logger

		err := test.router.updateRoute(test.route)
		items := func() map[string]*tunnelRoute {
			for _, val := range test.router.items {
				for subkey := range val.links {
					val.links[subkey] = nil
				}
			}
			return test.router.items
		}()
		assert.Equalf(t, test.items, items, "test '%s' items mismatch", name)
		assert.Equalf(t, test.err, err, "test '%s' error mismatch", name)

		hook.Reset()
		assert.Nil(t, hook.LastEntry())
	}
}

func TestDeleteByRoute(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		router    *syncTunnelRouter
		namespace string
		name      string
		items     map[string]*tunnelRoute
		err       error
	}{
		"router-empty": {
			router: &syncTunnelRouter{
				items: map[string]*tunnelRoute{},
			},
			namespace: "unit",
			name:      "a",
			items:     map[string]*tunnelRoute{},
			err:       nil,
		},
		"router-delete-route": {
			router: &syncTunnelRouter{
				items: map[string]*tunnelRoute{
					"unit/a": {
						namespace: "unit",
						name:      "a",
						links: tunnelRouteLinkMap{
							tunnelRule{port: 8080}: func() tunnelLink {
								l := &mockTunnelLink{}
								l.On("stop").Return(nil)
								return l
							}(),
						},
					},
					"unit/b": {
						namespace: "unit",
						name:      "b",
						links: tunnelRouteLinkMap{
							tunnelRule{port: 8080}: &mockTunnelLink{},
						},
					},
				},
			},
			namespace: "unit",
			name:      "a",
			items: map[string]*tunnelRoute{
				"unit/b": {
					namespace: "unit",
					name:      "b",
					links: tunnelRouteLinkMap{
						tunnelRule{port: 8080}: nil,
					},
				},
			},
			err: nil,
		},
	} {
		logger, hook := logtest.NewNullLogger()
		test.router.log = logger

		err := test.router.deleteByRoute(test.namespace, test.name)
		items := func() map[string]*tunnelRoute {
			for _, val := range test.router.items {
				for subkey := range val.links {
					val.links[subkey] = nil
				}
			}
			return test.router.items
		}()
		assert.Equalf(t, test.items, items, "test '%s' items mismatch", name)
		assert.Equalf(t, test.err, err, "test '%s' error mismatch", name)

		hook.Reset()
		assert.Nil(t, hook.LastEntry())
	}
}

func TestDeleteByKindKeys(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		router    *syncTunnelRouter
		kind      string
		namespace string
		name      string
		keys      []string
		items     map[string]*tunnelRoute
		err       error
	}{
		"router-empty": {
			router: &syncTunnelRouter{
				items: map[string]*tunnelRoute{},
			},
			kind:      "unit",
			namespace: "unit",
			name:      "a",
			keys:      []string{},
			items:     map[string]*tunnelRoute{},
			err:       nil,
		},
		"router-delete-route-links": {
			router: &syncTunnelRouter{
				items: map[string]*tunnelRoute{
					"unit/a": {
						namespace: "unit",
						name:      "a",
						links: tunnelRouteLinkMap{
							tunnelRule{
								service: resource{
									namespace: "unit",
									name:      "svc-a",
								},
								port: 8080,
							}: func() tunnelLink {
								l := &mockTunnelLink{}
								l.On("stop").Return(nil)
								return l
							}(),
							tunnelRule{
								service: resource{
									namespace: "unit",
									name:      "svc-b",
								},
								port: 8081,
							}: &mockTunnelLink{},
							tunnelRule{
								service: resource{
									namespace: "unit",
									name:      "svc-a",
								},
								port: 8082,
							}: func() tunnelLink {
								l := &mockTunnelLink{}
								l.On("stop").Return(nil)
								return l
							}(),
						},
					},
					"unit/b": {
						namespace: "unit",
						name:      "b",
						links: tunnelRouteLinkMap{
							tunnelRule{
								service: resource{
									namespace: "unit",
									name:      "svc-b",
								},
								port: 8080,
							}: &mockTunnelLink{},
							tunnelRule{
								service: resource{
									namespace: "unit",
									name:      "svc-a",
								},
								port: 8081,
							}: func() tunnelLink {
								l := &mockTunnelLink{}
								l.On("stop").Return(nil)
								return l
							}(),
							tunnelRule{
								service: resource{
									namespace: "unit",
									name:      "svc-b",
								},
								port: 8082,
							}: &mockTunnelLink{},
						},
					},
				},
			},
			kind:      serviceKind,
			namespace: "unit",
			name:      "svc-a",
			keys: []string{
				"unit/a",
				"unit/b",
			},
			items: map[string]*tunnelRoute{
				"unit/a": {
					namespace: "unit",
					name:      "a",
					links: tunnelRouteLinkMap{
						tunnelRule{
							service: resource{
								namespace: "unit",
								name:      "svc-b",
							},
							port: 8081,
						}: nil,
					},
				},
				"unit/b": {
					namespace: "unit",
					name:      "b",
					links: tunnelRouteLinkMap{
						tunnelRule{
							service: resource{
								namespace: "unit",
								name:      "svc-b",
							},
							port: 8080,
						}: nil,
						tunnelRule{
							service: resource{
								namespace: "unit",
								name:      "svc-b",
							},
							port: 8082,
						}: nil,
					},
				},
			},
			err: nil,
		},
	} {
		logger, hook := logtest.NewNullLogger()
		test.router.log = logger

		err := test.router.deleteByKindKeys(test.kind, test.namespace, test.name, test.keys)
		items := func() map[string]*tunnelRoute {
			for _, val := range test.router.items {
				for subkey := range val.links {
					val.links[subkey] = nil
				}
			}
			return test.router.items
		}()
		assert.Equalf(t, test.items, items, "test '%s' items mismatch", name)
		assert.Equalf(t, test.err, err, "test '%s' error mismatch", name)

		hook.Reset()
		assert.Nil(t, hook.LastEntry())
	}
}

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
