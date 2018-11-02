package argotunnel

import (
	"sync"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
)

type tunnelRouter interface {
	updateRoute(newRoute *tunnelRoute) (err error)
	updateByKindRoutes(kind, namespace, name string, routes []*tunnelRoute) (err error)
	deleteByRoute(namespace, name string) (err error)
	deleteByKindKeys(kind, namespace, name string, keys []string) (err error)
	run(stopCh <-chan struct{}) (err error)
}

type syncTunnelRouter struct {
	mu      sync.RWMutex
	items   map[string]*tunnelRoute
	log     *logrus.Logger
	options options
}

func (r *syncTunnelRouter) updateRoute(newRoute *tunnelRoute) (err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.unsafeUpdateRoute(newRoute)
	return
}

func (r *syncTunnelRouter) updateByKindRoutes(kind, namespace, name string, routes []*tunnelRoute) (err error) {
	r.log.Debugf("router update by %s: %s/%s", kind, namespace, name)
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, newRoute := range routes {
		r.unsafeUpdateRoute(newRoute)
	}
	return
}

// unsafeUpdateRoute requires the lock to be handled prior to call
func (r *syncTunnelRouter) unsafeUpdateRoute(newRoute *tunnelRoute) (err error) {
	r.log.Debugf("router update route: %s/%s", newRoute.namespace, newRoute.name)
	key := itemKeyFunc(newRoute.namespace, newRoute.name)

	oldRoute, exists := r.items[key]
	r.items[key] = newRoute

	if !exists {
		for _, newLink := range newRoute.links {
			newLink.start()
		}
	} else {
		swapLinks := tunnelRouteLinkMap{}
		for newRule, newLink := range newRoute.links {
			oldLink, ok := oldRoute.links[newRule]
			if !ok {
				newLink.start()
			} else {
				delete(oldRoute.links, newRule)
				if !oldLink.equal(newLink) {
					oldLink.stop()
					newLink.start()
				} else {
					swapLinks[newRule] = oldLink
				}
			}
		}
		for oldRule, oldLink := range swapLinks {
			newRoute.links[oldRule] = oldLink
		}
		for _, oldLink := range oldRoute.links {
			oldLink.stop()
		}
	}
	return
}

func (r *syncTunnelRouter) deleteByRoute(namespace, name string) (err error) {
	r.log.Debugf("router delete route: %s/%s", namespace, name)
	var wg wait.Group
	func() {
		key := itemKeyFunc(namespace, name)

		r.mu.Lock()
		defer r.mu.Unlock()

		oldRoute, exists := r.items[key]
		if !exists {
			return
		}

		delete(r.items, key)
		for _, oldLink := range oldRoute.links {
			wg.Start(stopLinkFunc(oldLink))
		}
	}()
	wg.Wait()
	return
}

func (r *syncTunnelRouter) deleteByKindKeys(kind, namespace, name string, keys []string) (err error) {
	r.log.Debugf("router delete by %s: %s/%s", kind, namespace, name)

	var wg wait.Group
	func() {
		r.mu.Lock()
		defer r.mu.Unlock()

		for _, key := range keys {
			r.log.Debugf("router delete route: %s", key)
			if oldRoute, exists := r.items[key]; exists {
				oldLinks := oldRoute.links
				newLinks := tunnelRouteLinkMap{}
				for oldRule, oldLink := range oldLinks {
					rc := getKindRuleResource(kind, oldRule)
					if rc.namespace == namespace && rc.name == name {
						wg.Start(stopLinkFunc(oldLink))
					} else {
						newLinks[oldRule] = oldLink
					}
				}
				oldRoute.links = newLinks
				r.items[key] = oldRoute
			}
		}
	}()
	wg.Wait()
	return
}

func (r *syncTunnelRouter) run(stopCh <-chan struct{}) (err error) {
	r.log.Debugf("starting argo-tunnel ingress tunnel router...")
	<-stopCh
	r.log.Debugf("stopping argo-tunnel ingress tunnel router...")
	r.halt()
	return
}

func (r *syncTunnelRouter) halt() (err error) {
	var wg wait.Group
	func() {
		r.mu.RLock()
		defer r.mu.RUnlock()

		for _, c := range r.items {
			for _, l := range c.links {
				wg.Start(stopLinkFunc(l))
			}
		}
	}()
	wg.Wait()
	return
}

func newTunnelRouter(log *logrus.Logger, opts options) tunnelRouter {
	return &syncTunnelRouter{
		items:   map[string]*tunnelRoute{},
		log:     log,
		options: opts,
	}
}

func stopLinkFunc(link tunnelLink) func() {
	return func() {
		link.stop()
	}
}

func getKindRuleResource(kind string, rule tunnelRule) (r *resource) {
	resourceFuncs := map[string]func(rule tunnelRule) *resource{
		endpointKind: func(rule tunnelRule) *resource {
			return &rule.service
		},
		secretKind: func(rule tunnelRule) *resource {
			return &rule.secret
		},
		serviceKind: func(rule tunnelRule) *resource {
			return &rule.service
		},
	}
	if resourceFunc, ok := resourceFuncs[kind]; ok {
		r = resourceFunc(rule)
	}
	return
}
