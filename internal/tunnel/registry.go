package tunnel

import (
	"sync"
)

// Registry key/tunnel map protected by a RWMutex
// The registry is a temporary mechanism used to enforce thread-safety.
// It is likely to replaced/removed with an update to the informers.
type Registry struct {
	mu  sync.RWMutex
	obj map[string]Tunnel
}

// Load recovers a tunnel by key
func (r *Registry) Load(key string) (val Tunnel, ok bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	val, ok = r.obj[key]
	return
}

// Store registers a tunnel by key
func (r *Registry) Store(key string, val Tunnel) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.obj == nil {
		r.obj = make(map[string]Tunnel)
	}
	r.obj[key] = val
}

// Delete removes a tunnel by key
func (r *Registry) Delete(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.obj, key)
	return
}

// LoadAndDelete deletes the tunnel by key and returns the stored value
func (r *Registry) LoadAndDelete(key string) (val Tunnel, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if val, ok = r.obj[key]; ok {
		delete(r.obj, key)
	}
	return
}

// Range processes the function on all tunnels (false terminates iteration).
func (r *Registry) Range(f func(key string, val Tunnel) bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for k, v := range r.obj {
		if !f(k, v) {
			break
		}
	}
}

// Filter processes the function on all tunnels (true deletes the tunnel).
func (r *Registry) Filter(f func(key string, val Tunnel) bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k, v := range r.obj {
		if f(k, v) {
			delete(r.obj, k)
		}
	}
}

// Len returns the number of items in the registry
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.obj)
}
