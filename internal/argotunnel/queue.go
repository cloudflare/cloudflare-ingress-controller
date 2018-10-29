package argotunnel

import (
	"fmt"
	"strings"

	"github.com/cloudflare/cloudflare-ingress-controller/internal/k8s"
	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

func queue(name string) workqueue.RateLimitingInterface {
	l := workqueue.DefaultControllerRateLimiter()
	return workqueue.NewNamedRateLimitingQueue(l, name)
}

func newEndpointEventHander(q workqueue.RateLimitingInterface) cache.ResourceEventHandler {
	return cache.FilteringResourceEventHandler{
		FilterFunc: endpointFilterFunc(),
		Handler:    newKindQueueEventHander(endpointKind, q),
	}
}

func newIngressEventHander(q workqueue.RateLimitingInterface, ingclass string) cache.ResourceEventHandler {
	return cache.FilteringResourceEventHandler{
		FilterFunc: ingressFilterFunc(ingclass),
		Handler:    newKindQueueEventHander(ingressKind, q),
	}
}

func newSecretEventHander(q workqueue.RateLimitingInterface) cache.ResourceEventHandler {
	return cache.FilteringResourceEventHandler{
		FilterFunc: secretFilterFunc(),
		Handler:    newKindQueueEventHander(secretKind, q),
	}
}

func newServiceEventHander(q workqueue.RateLimitingInterface) cache.ResourceEventHandler {
	return newKindQueueEventHander(serviceKind, q)
}

func newKindQueueEventHander(kind string, q workqueue.RateLimitingInterface) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := resourceKeyFunc(kind, obj)
			if err == nil {
				q.Add(key)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			key, err := resourceKeyFunc(kind, newObj)
			if err == nil {
				if !equality.Semantic.DeepEqual(newObj, oldObj) {
					q.Add(key)
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := deletionHandlingResourceKeyFunc(kind, obj)
			if err == nil {
				q.Add(key)
			}
		},
	}
}

func endpointFilterFunc() func(obj interface{}) bool {
	return func(obj interface{}) bool {
		if ep, ok := obj.(*v1.Endpoints); ok {
			return len(ep.Subsets) > 0
		}
		return false
	}
}

func ingressFilterFunc(ingressClass string) func(obj interface{}) bool {
	return func(obj interface{}) bool {
		if ing, ok := obj.(*v1beta1.Ingress); ok {
			if objIngressClass, ok := parseIngressClass(ing); ok {
				return ingressClass == objIngressClass
			}
		}
		return false
	}
}

func secretFilterFunc() func(obj interface{}) bool {
	return func(obj interface{}) bool {
		if sec, ok := obj.(*v1.Secret); ok {
			_, exists := sec.Data[k8s.CertPem]
			return exists
		}
		return false
	}
}

func serviceFilterFunc() func(obj interface{}) bool {
	return func(obj interface{}) bool {
		if svc, ok := obj.(*v1.Service); ok {
			return len(svc.Spec.Ports) > 0
		}
		return false
	}
}

func splitResourceKey(key string) (kind, namespace, name string, err error) {
	parts := strings.SplitN(key, "/", 4)
	switch len(parts) {
	case 2:
		return parts[0], "", parts[1], nil
	case 3:
		return parts[0], parts[1], parts[2], nil
	default:
		return "", "", "", fmt.Errorf("unexpected key format: %q", key)
	}
}

func splitKindMetaKey(key string) (kind, meta string, err error) {
	parts := strings.SplitN(key, "/", 2)
	switch len(parts) {
	case 2:
		return parts[0], parts[1], nil
	default:
		return "", "", fmt.Errorf("unexpected key format: %q", key)
	}
}

func resourceKeyFunc(kind string, obj interface{}) (string, error) {
	if key, ok := obj.(cache.ExplicitKey); ok {
		return string(key), nil
	}

	metaObj, err := meta.Accessor(obj)
	if err != nil {
		return "", fmt.Errorf("object has no meta: %v", err)
	}

	name := metaObj.GetName()
	ns := metaObj.GetNamespace()

	if len(ns) > 0 {
		return kind + "/" + ns + "/" + name, nil
	}
	return kind + "/" + name, nil
}

func deletionHandlingResourceKeyFunc(kind string, obj interface{}) (string, error) {
	if d, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		return d.Key, nil
	}
	return resourceKeyFunc(kind, obj)
}
