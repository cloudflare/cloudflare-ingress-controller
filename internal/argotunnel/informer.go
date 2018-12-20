package argotunnel

import (
	"fmt"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// TODO: consider registering indexers by kind in a map
type informerset struct {
	endpoint cache.SharedIndexInformer
	ingress  cache.SharedIndexInformer
	secret   cache.SharedIndexInformer
	service  cache.SharedIndexInformer
}

func (i *informerset) run(stopCh <-chan struct{}) {
	go i.endpoint.Run(stopCh)
	go i.ingress.Run(stopCh)
	go i.secret.Run(stopCh)
	go i.service.Run(stopCh)
}

func (i *informerset) getKindIndexer(kind string) (idx cache.Indexer, err error) {
	indexerFuncs := map[string]func() cache.Indexer{
		endpointKind: i.endpoint.GetIndexer,
		ingressKind:  i.ingress.GetIndexer,
		secretKind:   i.secret.GetIndexer,
		serviceKind:  i.service.GetIndexer,
	}
	if indexerFunc, ok := indexerFuncs[kind]; ok {
		idx = indexerFunc()
	} else {
		err = fmt.Errorf("unexpected kind (%q)", kind)
	}
	return
}

func (i *informerset) waitForCacheSync(stopCh <-chan struct{}) bool {
	return cache.WaitForCacheSync(stopCh,
		i.endpoint.HasSynced,
		i.ingress.HasSynced,
		i.secret.HasSynced,
		i.service.HasSynced,
	)
}

func newEndpointInformer(client kubernetes.Interface, opts options, rs ...cache.ResourceEventHandler) cache.SharedIndexInformer {
	return newInformer(client.CoreV1().RESTClient(), opts.watchNamespace, "endpoints", new(v1.Endpoints), opts.resyncPeriod, rs...)
}

func newIngressInformer(client kubernetes.Interface, opts options, rs ...cache.ResourceEventHandler) cache.SharedIndexInformer {
	i := newInformer(client.ExtensionsV1beta1().RESTClient(), opts.watchNamespace, "ingresses", new(v1beta1.Ingress), opts.resyncPeriod, rs...)
	i.AddIndexers(cache.Indexers{
		secretKind:  ingressSecretIndexFunc(opts.ingressClass, opts.originSecrets, opts.secret),
		serviceKind: ingressServiceIndexFunc(opts.ingressClass),
	})
	return i
}

func newSecretInformer(client kubernetes.Interface, opts options, rs ...cache.ResourceEventHandler) cache.SharedIndexInformer {
	return newInformer(client.CoreV1().RESTClient(), opts.watchNamespace, "secrets", new(v1.Secret), opts.resyncPeriod, rs...)
}

func newServiceInformer(client kubernetes.Interface, opts options, rs ...cache.ResourceEventHandler) cache.SharedIndexInformer {
	return newInformer(client.CoreV1().RESTClient(), opts.watchNamespace, "services", new(v1.Service), opts.resyncPeriod, rs...)
}

func newInformer(c cache.Getter, namespace string, resource string, objType runtime.Object, resyncPeriod time.Duration, rs ...cache.ResourceEventHandler) cache.SharedIndexInformer {
	lw := cache.NewListWatchFromClient(c, resource, namespace, fields.Everything())
	sw := cache.NewSharedIndexInformer(lw, objType, resyncPeriod, cache.Indexers{
		//cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
	})
	for _, r := range rs {
		sw.AddEventHandler(r)
	}
	return sw
}

func ingressSecretIndexFunc(ingressClass string, originSecrets map[string]*resource, secret *resource) func(obj interface{}) ([]string, error) {
	return func(obj interface{}) ([]string, error) {
		if ing, ok := obj.(*v1beta1.Ingress); ok {
			var idx []string
			if objIngClass, ok := parseIngressClass(ing); ok && ingressClass == objIngClass {
				hostsecret := make(map[string]*resource)
				for _, tls := range ing.Spec.TLS {
					for _, host := range tls.Hosts {
						if len(tls.SecretName) > 0 {
							hostsecret[host] = &resource{
								name:      tls.SecretName,
								namespace: ing.Namespace,
							}
						}
					}
				}
				for _, rule := range ing.Spec.Rules {
					if rule.HTTP != nil && len(rule.Host) > 0 {
						if r, ok := hostsecret[rule.Host]; ok {
							idx = append(idx, itemKeyFunc(r.namespace, r.name))
						} else if r, ok := originSecrets[rule.Host]; ok {
							idx = append(idx, itemKeyFunc(r.namespace, r.name))
						} else if secret != nil {
							idx = append(idx, itemKeyFunc(secret.namespace, secret.name))
						}
					}
				}
			}
			return idx, nil
		}
		return []string{}, fmt.Errorf("index unexpected obj type: %T", obj)
	}
}

func ingressServiceIndexFunc(ingressClass string) func(obj interface{}) ([]string, error) {
	return func(obj interface{}) ([]string, error) {
		if ing, ok := obj.(*v1beta1.Ingress); ok {
			var idx []string
			if objIngClass, ok := parseIngressClass(ing); ok && ingressClass == objIngClass {
				for _, rule := range ing.Spec.Rules {
					if rule.HTTP != nil && len(rule.Host) > 0 {
						for _, path := range rule.HTTP.Paths {
							if len(path.Backend.ServiceName) > 0 {
								idx = append(idx, itemKeyFunc(ing.Namespace, path.Backend.ServiceName))
							}
						}
					}
				}
			}
			return idx, nil
		}
		return []string{}, fmt.Errorf("index unexpected obj type: %T", obj)
	}
}

func itemKeyFunc(namespace, name string) (key string) {
	key = namespace + "/" + name
	return
}
