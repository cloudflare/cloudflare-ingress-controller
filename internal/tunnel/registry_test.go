package tunnel

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The tunnel interface does not provide a unique identifier for the
// tunnel.  In the tests below, tunnel is mocked to provide a dummy
// implemention where config can be used to setup and recover an
// identifying marker (ServiceName) for testing.

func TestLoad(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		obj map[string]Tunnel
		key string
		tag string
		ok  bool
	}{
		"nil-map": {
			obj: nil,
			key: "test",
			tag: "",
			ok:  false,
		},
		"not-found": {
			obj: map[string]Tunnel{
				"unit": nil,
			},
			key: "test",
			tag: "",
			ok:  false,
		},
		"load": {
			obj: map[string]Tunnel{
				"test": func() Tunnel {
					t := &mockTunnel{}
					t.On("Route").Return(Route{
						ServiceName: "test",
					})
					return t
				}(),
			},
			key: "test",
			tag: "test",
			ok:  true,
		},
	} {
		r := Registry{
			obj: test.obj,
		}

		_, ok := r.Load(test.key)
		tag := func() (tag string) {
			if r.obj != nil {
				if t, ok := r.obj[test.key]; ok {
					tag = t.Route().ServiceName
				}
			}
			return
		}()
		assert.Equalf(t, test.ok, ok, "test '%s' ok mismatch", name)
		assert.Equalf(t, test.tag, tag, "test '%s' tag mismatch", name)
	}
}

func TestStore(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		obj map[string]Tunnel
		key string
		val Tunnel
		len int
	}{
		"nil-map": {
			obj: nil,
			key: "test",
			val: func() Tunnel {
				t := &mockTunnel{}
				t.On("Route").Return(Route{
					ServiceName: "test",
				})
				return t
			}(),
			len: 1,
		},
		"append": {
			obj: map[string]Tunnel{
				"test-a": func() Tunnel {
					t := &mockTunnel{}
					t.On("Route").Return(Route{
						ServiceName: "test-a",
					})
					return t
				}(),
			},
			key: "test-b",
			val: func() Tunnel {
				t := &mockTunnel{}
				t.On("Route").Return(Route{
					ServiceName: "test-b",
				})
				return t
			}(),
			len: 2,
		},
		"overwrite": {
			obj: map[string]Tunnel{
				"test": func() Tunnel {
					t := &mockTunnel{}
					t.On("Route").Return(Route{
						ServiceName: "test-a",
					})
					return t
				}(),
			},
			key: "test",
			val: func() Tunnel {
				t := &mockTunnel{}
				t.On("Route").Return(Route{
					ServiceName: "test-b",
				})
				return t
			}(),
			len: 1,
		},
	} {
		r := Registry{
			obj: test.obj,
		}

		r.Store(test.key, test.val)
		val := func() (t Tunnel) {
			if r.obj != nil {
				t, _ = r.obj[test.key]
			}
			return
		}()
		assert.Equalf(t, test.val, val, "test '%s' store mismatch", name)
		assert.Equalf(t, test.len, len(r.obj), "test '%s' length mismatch", name)
	}
}

func TestDelete(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		obj map[string]Tunnel
		key string
		len int
	}{
		"nil-map": {
			obj: nil,
			key: "test",
			len: 0,
		},
		"not-found": {
			obj: map[string]Tunnel{
				"test-a": func() Tunnel {
					t := &mockTunnel{}
					t.On("Route").Return(Route{
						ServiceName: "test-a",
					})
					return t
				}(),
			},
			key: "test",
			len: 1,
		},
		"remove": {
			obj: map[string]Tunnel{
				"test-a": func() Tunnel {
					t := &mockTunnel{}
					t.On("Route").Return(Route{
						ServiceName: "test-a",
					})
					return t
				}(),
				"test-b": func() Tunnel {
					t := &mockTunnel{}
					t.On("Route").Return(Route{
						ServiceName: "test-b",
					})
					return t
				}(),
			},
			key: "test-a",
			len: 1,
		},
	} {
		r := Registry{
			obj: test.obj,
		}

		r.Delete(test.key)
		out := func() (t Tunnel) {
			if r.obj != nil {
				t, _ = r.obj[test.key]
			}
			return
		}()
		assert.Emptyf(t, out, "test '%s' delete found non-nil tunnel", name)
		assert.Equalf(t, test.len, len(r.obj), "test '%s' length mismatch", name)
	}
}

func TestLoadAndDelete(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		obj map[string]Tunnel
		key string
		tag string
		ok  bool
		len int
	}{
		"nil-map": {
			obj: nil,
			key: "test",
			ok:  false,
			tag: "",
			len: 0,
		},
		"not-found": {
			obj: map[string]Tunnel{
				"test-a": func() Tunnel {
					t := &mockTunnel{}
					t.On("Route").Return(Route{
						ServiceName: "test-a",
					})
					return t
				}(),
			},
			key: "test",
			ok:  false,
			tag: "",
			len: 1,
		},
		"load-and-remove": {
			obj: map[string]Tunnel{
				"test-a": func() Tunnel {
					t := &mockTunnel{}
					t.On("Route").Return(Route{
						ServiceName: "test-a",
					})
					return t
				}(),
				"test-b": func() Tunnel {
					t := &mockTunnel{}
					t.On("Route").Return(Route{
						ServiceName: "test-b",
					})
					return t
				}(),
			},
			key: "test-b",
			ok:  true,
			tag: "test-b",
			len: 1,
		},
	} {
		r := Registry{
			obj: test.obj,
		}

		val, ok := r.LoadAndDelete(test.key)
		tag := func() (tag string) {
			if val != nil {
				tag = val.Route().ServiceName
			}
			return
		}()
		out := func() (t Tunnel) {
			if r.obj != nil {
				t, _ = r.obj[test.key]
			}
			return
		}()
		assert.Emptyf(t, out, "test '%s' delete found non-nil tunnel", name)
		assert.Equalf(t, test.len, len(r.obj), "test '%s' length mismatch", name)
		assert.Equalf(t, test.ok, ok, "test '%s' ok mismatch", name)
		assert.Equalf(t, test.tag, tag, "test '%s' tag mismatch", name)
	}
}

func TestRange(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		obj map[string]Tunnel
		key string
		len int
	}{
		"nil-map": {
			obj: nil,
			key: "test-a",
			len: 0,
		},
		"visit-none": {
			obj: map[string]Tunnel{
				"test-a": func() Tunnel {
					t := &mockTunnel{}
					t.On("Route").Return(Route{
						ServiceName: "test-a",
					})
					return t
				}(),
				"test-b": func() Tunnel {
					t := &mockTunnel{}
					t.On("Route").Return(Route{
						ServiceName: "test-b",
					})
					return t
				}(),
				"test-c": func() Tunnel {
					t := &mockTunnel{}
					t.On("Route").Return(Route{
						ServiceName: "test-c",
					})
					return t
				}(),
			},
			key: "test-",
			len: 1,
		},
		"visit-all": {
			obj: map[string]Tunnel{
				"test-a": func() Tunnel {
					t := &mockTunnel{}
					t.On("Route").Return(Route{
						ServiceName: "test-a",
					})
					return t
				}(),
				"test-b": func() Tunnel {
					t := &mockTunnel{}
					t.On("Route").Return(Route{
						ServiceName: "test-b",
					})
					return t
				}(),
				"test-c": func() Tunnel {
					t := &mockTunnel{}
					t.On("Route").Return(Route{
						ServiceName: "test-c",
					})
					return t
				}(),
			},
			key: "test-z",
			len: 3,
		},
	} {
		r := Registry{
			obj: test.obj,
		}

		len := 0
		r.Range(func(k string, t Tunnel) bool {
			len++
			return !strings.Contains(k, test.key)
		})

		assert.Equalf(t, test.len, len, "test '%s' length mismatch", name)
	}
}

func TestFilter(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		obj map[string]Tunnel
		key string
		len int
	}{
		"nil-map": {
			obj: nil,
			key: "test-a",
			len: 0,
		},
		"filter-none": {
			obj: map[string]Tunnel{
				"test-a": func() Tunnel {
					t := &mockTunnel{}
					t.On("Route").Return(Route{
						ServiceName: "test-a",
					})
					return t
				}(),
				"test-b": func() Tunnel {
					t := &mockTunnel{}
					t.On("Route").Return(Route{
						ServiceName: "test-b",
					})
					return t
				}(),
				"test-c": func() Tunnel {
					t := &mockTunnel{}
					t.On("Route").Return(Route{
						ServiceName: "test-c",
					})
					return t
				}(),
			},
			key: "test-z",
			len: 3,
		},
		"filter-one": {
			obj: map[string]Tunnel{
				"test-a": func() Tunnel {
					t := &mockTunnel{}
					t.On("Route").Return(Route{
						ServiceName: "test-a",
					})
					return t
				}(),
				"test-b": func() Tunnel {
					t := &mockTunnel{}
					t.On("Route").Return(Route{
						ServiceName: "test-b",
					})
					return t
				}(),
				"test-c": func() Tunnel {
					t := &mockTunnel{}
					t.On("Route").Return(Route{
						ServiceName: "test-c",
					})
					return t
				}(),
			},
			key: "test-b",
			len: 2,
		},
		"filter-all": {
			obj: map[string]Tunnel{
				"test-a": func() Tunnel {
					t := &mockTunnel{}
					t.On("Route").Return(Route{
						ServiceName: "test-a",
					})
					return t
				}(),
				"test-b": func() Tunnel {
					t := &mockTunnel{}
					t.On("Route").Return(Route{
						ServiceName: "test-b",
					})
					return t
				}(),
				"test-c": func() Tunnel {
					t := &mockTunnel{}
					t.On("Route").Return(Route{
						ServiceName: "test-c",
					})
					return t
				}(),
			},
			key: "test-",
			len: 0,
		},
	} {
		r := Registry{
			obj: test.obj,
		}

		r.Filter(func(k string, t Tunnel) bool {
			return strings.Contains(k, test.key)
		})

		assert.Equalf(t, test.len, len(r.obj), "test '%s' length mismatch", name)
	}
}
