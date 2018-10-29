package argotunnel

import (
	"fmt"

	"github.com/cloudflare/cloudflare-ingress-controller/internal/k8s"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"
)

type translator interface {
	handleResource(kind, key string) (err error)
	waitForCacheSync(stopCh <-chan struct{}) (ok bool)
	run(stopCh <-chan struct{}) (err error)
}

func newTranslator(informers informerset, log *logrus.Logger, opts options) translator {
	return &syncTranslator{
		informers: informers,
		router:    newTunnelRouter(log, opts),
		log:       log,
		options:   opts,
	}
}

type syncTranslator struct {
	informers informerset
	router    tunnelRouter
	log       *logrus.Logger
	options   options
}

func (t *syncTranslator) run(stopCh <-chan struct{}) (err error) {
	t.informers.run(stopCh)
	return
}

func (t *syncTranslator) waitForCacheSync(stopCh <-chan struct{}) (ok bool) {
	ok = t.informers.waitForCacheSync(stopCh)
	return
}

func (t *syncTranslator) handleResource(kind, key string) (err error) {
	handlerFuncs := map[string]func(kind, key string) error{
		endpointKind: t.handleEndpoint,
		ingressKind:  t.handleIngress,
		secretKind:   t.handleSecret,
		serviceKind:  t.handleService,
	}
	if handlerFunc, ok := handlerFuncs[kind]; ok {
		err = handlerFunc(kind, key)
	} else {
		err = fmt.Errorf("unexpected kind (%q) in key (%q)", kind, key)
	}
	return
}

func (t *syncTranslator) handleEndpoint(kind, key string) (err error) {
	_, exists, err := t.informers.endpoint.GetIndexer().GetByKey(key)
	if err == nil {
		if exists {
			t.updateByKind(kind, key)
		} else {
			t.deleteByKind(kind, key)
		}
	}
	return
}

func (t *syncTranslator) handleIngress(kind, key string) (err error) {
	obj, exists, err := t.informers.ingress.GetIndexer().GetByKey(key)
	if err == nil {
		if exists {
			t.updateIngress(key, obj.(*v1beta1.Ingress))
		} else {
			t.deleteIngress(key)
		}
	}
	return
}

func (t *syncTranslator) handleSecret(kind, key string) (err error) {
	_, exists, err := t.informers.secret.GetIndexer().GetByKey(key)
	if err == nil {
		if exists {
			t.updateByKind(kind, key)
		} else {
			t.deleteByKind(kind, key)
		}
	}
	return
}

func (t *syncTranslator) handleService(kind, key string) (err error) {
	_, exists, err := t.informers.service.GetIndexer().GetByKey(key)
	if err == nil {
		if exists {
			t.updateByKind(kind, key)
		} else {
			t.deleteByKind(kind, key)
		}
	}
	return
}

func (t *syncTranslator) updateByKind(kind, key string) (err error) {
	t.log.Debugf("translator update %s: %s", kind, key)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return
	}

	objs, err := t.informers.ingress.GetIndexer().ByIndex(kind, key)
	if err != nil {
		return
	} else if len(objs) == 0 {
		return
	}

	routes := make([]*tunnelRoute, 0, len(objs))
	for _, obj := range objs {
		if route := t.getRouteFromIngress(obj.(*v1beta1.Ingress)); route != nil {
			routes = append(routes, route)
		}
	}
	t.router.updateByKindRoutes(kind, namespace, name, routes)
	return
}

func (t *syncTranslator) deleteByKind(kind, key string) (err error) {
	t.log.Debugf("translator delete %s: %s", kind, key)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return
	}

	keys, err := t.informers.ingress.GetIndexer().IndexKeys(kind, key)
	if err != nil {
		return
	} else if len(keys) == 0 {
		return
	}
	err = t.router.deleteByKindKeys(kind, namespace, name, keys)
	return
}

func (t *syncTranslator) updateIngress(key string, ing *v1beta1.Ingress) (err error) {
	if route := t.getRouteFromIngress(ing); route != nil {
		err = t.router.updateRoute(route)
	}
	return
}

func (t *syncTranslator) deleteIngress(key string) (err error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return
	}

	t.log.Debugf("translator delete ingress: %s", key)
	err = t.router.deleteByRoute(namespace, name)
	return
}

func (t *syncTranslator) getRouteFromIngress(ing *v1beta1.Ingress) (r *tunnelRoute) {
	switch {
	case ing == nil:
		return
	}

	opts := collectTunnelOptions(parseIngressTunnelOptions(ing))
	hostsecret := make(map[string]*resource)
	for _, tls := range ing.Spec.TLS {
		for _, host := range tls.Hosts {
			hostsecret[host] = &resource{
				name:      tls.SecretName,
				namespace: ing.Namespace,
			}
		}
	}
	linkmap := tunnelRouteLinkMap{}
	ingkey := itemKeyFunc(ing.Namespace, ing.Name)
	for _, rule := range ing.Spec.Rules {
		if rule.HTTP == nil || len(rule.Host) == 0 {
			continue
		}
		host := rule.Host
		secret := func() *resource {
			if r, ok := hostsecret[rule.Host]; ok {
				return r
			} else if t.options.secret != nil {
				return t.options.secret
			}
			return nil
		}()
		for _, path := range rule.HTTP.Paths {
			// ingress rule path
			if len(path.Path) > 0 {
				t.log.Warnf("translator path routing not supported on ingress: %s, host: %s, path: %+v", ingkey, host, path)
				continue
			}

			// service
			if len(path.Backend.ServiceName) == 0 {
				t.log.Warnf("translator service empty on ingress: %s, host: %s, path: %+v", ingkey, host, path)
				continue
			}
			key := itemKeyFunc(ing.Namespace, path.Backend.ServiceName)
			obj, exists, err := t.informers.service.GetIndexer().GetByKey(key)
			if err != nil {
				t.log.Errorf("translator service lookup failed on ingress: %s, host: %s, path: %+v, err: %v", ingkey, host, path, err)
				continue
			} else if !exists {
				t.log.Warnf("translator service missing on ingress: %s, host: %s, path: %+v", ingkey, host, path)
				continue
			}
			port, exists := k8s.GetServicePort(obj.(*v1.Service), path.Backend.ServicePort)
			if !exists {
				t.log.Warnf("translator service port missing on ingress: %s, host: %s, path: %+v", ingkey, host, path)
				continue
			}

			// endpoints
			obj, exists, err = t.informers.endpoint.GetIndexer().GetByKey(key)
			if err != nil {
				t.log.Errorf("translator endpoints lookup failed on ingress: %s, host: %s, path: %+v, err: %v", ingkey, host, path, err)
				continue
			} else if !exists {
				t.log.Warnf("translator endpoints missing on ingress: %s, host: %s, path: %+v", ingkey, host, path)
				continue
			}
			if ok := k8s.EndpointsHaveSubsets(obj.(*v1.Endpoints)); !ok {
				t.log.Warnf("translator endpoint subsets missing on ingress: %s, host: %s, path: %+v", ingkey, host, path)
				continue
			}

			// TODO: verify that the service port matches subset ports

			// secret
			if secret == nil {
				t.log.Warnf("translator secret not defined on ingress: %s, host: %s, path: %+v", ingkey, host, path)
				continue
			}
			key = itemKeyFunc(secret.namespace, secret.name)
			obj, exists, err = t.informers.secret.GetIndexer().GetByKey(key)
			if err != nil {
				t.log.Errorf("translator secret lookup failed on ingress: %s, host: %s, path: %+v, err: %v", ingkey, host, path, err)
				continue
			} else if !exists {
				t.log.Warnf("translator secret missing on ingress: %s, host: %s, path: %+v", ingkey, host, path)
				continue
			}
			cert, exists := k8s.GetSecretCert(obj.(*v1.Secret))
			if !exists {
				t.log.Warnf("translator secret 'cert.pem' missing on ingress: %s, host: %s, path: %+v", ingkey, host, path)
			}

			// TODO: validate certificate against host
			// with the validation, a link is created and started but stuck
			// in a repair loop (errors immediately on launch)
			// https://golang.org/src/crypto/x509/example_test.go
			// https://golang.org/pkg/crypto/x509/#Certificate.VerifyHostname

			// attach rule|link to route
			rule := tunnelRule{
				host: host,
				port: port,
				service: resource{
					namespace: ing.Namespace,
					name:      path.Backend.ServiceName,
				},
				secret: *secret,
			}
			t.log.Debugf("translator attach tunnel: %s, rule: %+v", ingkey, rule)
			linkmap[rule] = newTunnelLink(rule, cert, opts)
		}
	}
	r = &tunnelRoute{
		name:      ing.Name,
		namespace: ing.Namespace,
		options:   opts,
		links:     linkmap,
	}
	return
}
