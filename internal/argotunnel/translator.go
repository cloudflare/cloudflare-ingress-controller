package argotunnel

import (
	"fmt"

	"github.com/cloudflare/cloudflare-ingress-controller/internal/k8s"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/util/intstr"
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
		secretKind:   t.handleByKind,
		serviceKind:  t.handleByKind,
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
			err = t.updateByKind(serviceKind, key)
		} else {
			err = t.deleteByKind(serviceKind, key)
		}
	}
	return
}

func (t *syncTranslator) handleByKind(kind, key string) (err error) {
	indexer, err := t.informers.getKindIndexer(kind)
	if err != nil {
		return
	}
	_, exists, err := indexer.GetByKey(key)
	if err == nil {
		if exists {
			err = t.updateByKind(kind, key)
		} else {
			err = t.deleteByKind(kind, key)
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

func (t *syncTranslator) handleIngress(kind, key string) (err error) {
	obj, exists, err := t.informers.ingress.GetIndexer().GetByKey(key)
	if err == nil {
		if exists {
			err = t.updateIngress(key, obj.(*v1beta1.Ingress))
		} else {
			err = t.deleteIngress(key)
		}
	}
	return
}

func (t *syncTranslator) updateIngress(key string, ing *v1beta1.Ingress) (err error) {
	t.log.Debugf("translator update ingress: %s", key)
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
	// TODO: update function to allow specific failure detection for testing
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
			} else if r, ok := t.options.originSecrets[rule.Host]; ok {
				return r
			} else if t.options.secret != nil {
				return t.options.secret
			}
			return nil
		}()

		// secret
		var cert []byte
		{
			var err error
			var exists bool
			if secret == nil {
				t.log.Errorf("translator secret not defined on ingress: %s, host: %s", ingkey, host)
				continue
			}
			cert, exists, err = t.getVerifiedCert(secret.namespace, secret.name, host)
			if err != nil {
				t.log.Errorf("translator secret issue on ingress: %s, host: %s, err: %v", ingkey, host, err)
				continue
			} else if !exists {
				t.log.Errorf("translator secret missing cert on ingress: %s, host: %s", ingkey, host)
				continue
			}
		}

		for _, path := range rule.HTTP.Paths {
			// ingress
			if len(path.Path) > 0 && path.Path != "/" {
				t.log.Errorf("translator path routing not supported on ingress: %s, host: %s, path: %+v", ingkey, host, path)
				continue
			}
			if len(path.Backend.ServiceName) == 0 {
				t.log.Errorf("translator service empty on ingress: %s, host: %s, path: %+v", ingkey, host, path)
				continue
			}

			// service
			var port int32
			{
				var err error
				var exists bool
				port, exists, err = t.getVerifiedPort(ing.Namespace, path.Backend.ServiceName, path.Backend.ServicePort)
				if err != nil {
					t.log.Errorf("translator service issue on ingress: %s, host: %s, path: %+v, err: %q", ingkey, host, path, err)
					continue
				} else if !exists {
					t.log.Errorf("translator service missing port on ingress: %s, host: %s, path: %+v", ingkey, host, path)
					continue
				}
			}

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
		links:     linkmap,
	}
	return
}

func (t *syncTranslator) getVerifiedCert(namespace, name, host string) (cert []byte, exists bool, err error) {
	key := itemKeyFunc(namespace, name)
	obj, exists, err := t.informers.secret.GetIndexer().GetByKey(key)
	if err != nil {
		return
	} else if !exists {
		err = fmt.Errorf("secret '%s' does not exist", key)
		return
	}

	cert, exists = k8s.GetSecretCert(obj.(*v1.Secret))
	if !exists {
		err = fmt.Errorf("secret '%s' missing 'cert.pem'", key)
		return
	}

	err = verifyCertForHost(cert, host)
	if err != nil {
		cert = nil
	}
	return
}

func (t *syncTranslator) getVerifiedPort(namespace, name string, port intstr.IntOrString) (val int32, exists bool, err error) {
	key := itemKeyFunc(namespace, name)
	obj, exists, err := t.informers.service.GetIndexer().GetByKey(key)
	if err != nil {
		return
	} else if !exists {
		err = fmt.Errorf("service '%s' does not exist", key)
		return
	}

	svcport, exists := k8s.GetServicePort(obj.(*v1.Service), port, v1.ProtocolTCP)
	if !exists {
		err = fmt.Errorf("service '%s' missing port '%s'", key, port.String())
		return
	}

	obj, exists, err = t.informers.endpoint.GetIndexer().GetByKey(key)
	if err != nil {
		return
	} else if !exists {
		err = fmt.Errorf("endpoints '%s' do not exist", key)
		return
	}

	exists = k8s.HasEndpointsAddresses(obj.(*v1.Endpoints))
	if !exists {
		err = fmt.Errorf("endpoints '%s' missing subsets for port '%s'", key, port.String())
		return
	}

	val = svcport.Port
	return
}
