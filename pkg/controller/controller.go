/*
Copyright 2016 Skippbox, Ltd.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	lister_v1 "k8s.io/client-go/listers/core/v1"
	lister_v1beta1 "k8s.io/client-go/listers/extensions/v1beta1"

	"github.com/cloudflare/cloudflare-warp-ingress/pkg/tunnel"
)

const maxRetries = 5

// WarpController object
type WarpController struct {
	client kubernetes.Interface

	ingressLister    lister_v1beta1.IngressLister
	ingressInformer  cache.Controller
	ingressWorkqueue workqueue.RateLimitingInterface

	serviceLister    lister_v1.ServiceLister
	serviceInformer  cache.Controller
	serviceWorkqueue workqueue.RateLimitingInterface

	endpointsLister   lister_v1.EndpointsLister
	endpointsInformer cache.Controller

	namespace string
	tunnels   map[string]tunnel.Tunnel
}

func NewWarpController(client kubernetes.Interface, namespace string) *WarpController {

	informer, indexer, queue := createIngressInformer(client, namespace)
	tunnels := make(map[string]tunnel.Tunnel, 0)

	w := &WarpController{
		client: client,

		ingressInformer:  informer,
		ingressWorkqueue: queue,
		ingressLister:    lister_v1beta1.NewIngressLister(indexer),

		namespace: namespace,
		tunnels:   tunnels,
	}
	w.configureServiceInformer()
	w.configureEndpointInformer()

	return w
}

func createIngressInformer(client kubernetes.Interface, namespace string) (cache.Controller, cache.Indexer, workqueue.RateLimitingInterface) {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	indexer, informer := cache.NewIndexerInformer(

		&cache.ListWatch{
			ListFunc: func(lo meta_v1.ListOptions) (runtime.Object, error) {
				return client.ExtensionsV1beta1().Ingresses(namespace).List(lo)
			},
			WatchFunc: func(lo meta_v1.ListOptions) (watch.Interface, error) {
				return client.ExtensionsV1beta1().Ingresses(namespace).Watch(lo)
			},
		},

		// The types of objects this informer will return
		&v1beta1.Ingress{},

		// The resync period of this object.
		60*time.Second,

		// Callback Functions to trigger on add/update/delete
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				if !shouldHandleIngress(obj) {
					return
				}
				if key, err := cache.MetaNamespaceKeyFunc(obj); err == nil {
					queue.Add("add:" + key)
				}
			},
			UpdateFunc: func(old, new interface{}) {
				if shouldHandleIngress(old) || shouldHandleIngress(new) {
					if key, err := cache.MetaNamespaceKeyFunc(new); err == nil {
						queue.Add("update:" + key)
					}
				} else {
					return
				}
			},
			DeleteFunc: func(obj interface{}) {
				if !shouldHandleIngress(obj) {
					return
				}
				if key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj); err == nil {
					queue.Add("delete:" + key)
				}
			},
		},
		cache.Indexers{},
	)
	return informer, indexer, queue
}

// check the type and the annotation before enqueueing
func shouldHandleIngress(obj interface{}) bool {
	var ingressClassKey = "kubernetes.io/ingress.class"

	ingress, ok := obj.(*v1beta1.Ingress)
	if !ok {
		glog.V(5).Infof("Object is not an ingress, don't handle")
		return false
	}
	val, ok := ingress.Annotations[ingressClassKey]
	if !ok {
		glog.V(5).Infof("No annotation found for %s", ingressClassKey)
		return false
	}
	glog.V(5).Infof("Annotation %s=%s", ingressClassKey, val)
	if val != "cloudflare-warp" {
		return false
	}
	return true
}

func (w *WarpController) configureServiceInformer() {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	indexer, informer := cache.NewIndexerInformer(

		&cache.ListWatch{
			ListFunc: func(lo meta_v1.ListOptions) (runtime.Object, error) {
				return w.client.CoreV1().Services(w.namespace).List(lo)
			},
			WatchFunc: func(lo meta_v1.ListOptions) (watch.Interface, error) {
				return w.client.CoreV1().Services(w.namespace).Watch(lo)
			},
		},

		// The types of objects this informer will return
		&v1.Service{},

		// The resync period of this object.
		60*time.Second,

		// Callback Functions to trigger on add/update/delete
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				svc, ok := obj.(*v1.Service)
				if !ok || !w.isWatchedService(svc) {
					return
				}
				if key, err := cache.MetaNamespaceKeyFunc(obj); err == nil {
					queue.Add("add:" + key)
				}
			},
			UpdateFunc: func(old, new interface{}) {
				svc, ok := new.(*v1.Service)
				if !ok || !w.isWatchedService(svc) {
					return
				}
				if key, err := cache.MetaNamespaceKeyFunc(new); err == nil {
					queue.Add("update:" + key)
				}
			},
			DeleteFunc: func(obj interface{}) {
				svc, ok := obj.(*v1.Service)
				if !ok || !w.isWatchedService(svc) {
					return
				}
				if key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj); err == nil {
					queue.Add("delete:" + key)
				}
			},
		},
		cache.Indexers{},
	)
	w.serviceInformer = informer
	w.serviceLister = lister_v1.NewServiceLister(indexer)
	w.serviceWorkqueue = queue
}

// is this service one of the ones we have a tunnel for?
func (w *WarpController) isWatchedService(service *v1.Service) bool {
	for _, tunnel := range w.tunnels {
		if service.ObjectMeta.Name == tunnel.Config().ServiceName {
			glog.V(5).Infof("Watching service %s/%s", service.ObjectMeta.Namespace, service.ObjectMeta.Name)
			return true
		}
	}
	return false
}

func (w *WarpController) configureEndpointInformer() {

	indexer, informer := cache.NewIndexerInformer(

		&cache.ListWatch{
			ListFunc: func(lo meta_v1.ListOptions) (runtime.Object, error) {
				return w.client.CoreV1().Endpoints(w.namespace).List(lo)
			},
			WatchFunc: func(lo meta_v1.ListOptions) (watch.Interface, error) {
				return w.client.CoreV1().Endpoints(w.namespace).Watch(lo)
			},
		},

		// The types of objects this informer will return
		&v1.Endpoints{},

		// The resync period of this object.
		60*time.Second,

		// Queue all these changes as an update to the service
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				ep, ok := obj.(*v1.Endpoints)
				if !ok || !w.isWatchedEndpoint(ep) {
					return
				}
				if key, err := cache.MetaNamespaceKeyFunc(obj); err == nil {
					w.serviceWorkqueue.Add("update:" + key)
				}
			},
			UpdateFunc: func(old, new interface{}) {
				ep, ok := new.(*v1.Endpoints)
				if !ok || !w.isWatchedEndpoint(ep) {
					return
				}
				if key, err := cache.MetaNamespaceKeyFunc(new); err == nil {
					w.serviceWorkqueue.Add("update:" + key)
				}
			},
			DeleteFunc: func(obj interface{}) {
				ep, ok := obj.(*v1.Endpoints)
				if !ok || !w.isWatchedEndpoint(ep) {
					return
				}
				if key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj); err == nil {
					w.serviceWorkqueue.Add("update:" + key)
				}
			},
		},
		cache.Indexers{},
	)
	w.endpointsInformer = informer
	w.endpointsLister = lister_v1.NewEndpointsLister(indexer)
}

// is this endpoint interesting?
func (w *WarpController) isWatchedEndpoint(ep *v1.Endpoints) bool {

	for _, tunnel := range w.tunnels {
		if ep.ObjectMeta.Name == tunnel.Config().ServiceName {
			glog.V(5).Infof("Watching endpoint %s/%s", ep.ObjectMeta.Namespace, ep.ObjectMeta.Name)
			return true
		}
	}
	return false
}

func (w *WarpController) Run(stopCh chan struct{}) {
	defer w.ingressWorkqueue.ShutDown()
	defer w.serviceWorkqueue.ShutDown()

	glog.Info("Starting WarpController")

	go w.serviceInformer.Run(stopCh)
	go w.endpointsInformer.Run(stopCh)
	go w.ingressInformer.Run(stopCh)

	// Wait for all caches to be synced, before processing items from the queue is started
	if !cache.WaitForCacheSync(stopCh, w.ingressInformer.HasSynced) {
		glog.Error(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}
	if !cache.WaitForCacheSync(stopCh, w.serviceInformer.HasSynced) {
		glog.Error(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	go wait.Until(w.runIngressWorker, time.Second, stopCh)
	go wait.Until(w.runServiceWorker, time.Second, stopCh)

	<-stopCh
	glog.Info("Stopping WarpController ")
	w.tearDown()
}

func (w *WarpController) runIngressWorker() {
	for w.processNextIngress() {
	}
}

func (w *WarpController) processNextIngress() bool {

	key, quit := w.ingressWorkqueue.Get()
	if quit {
		return false
	}
	// Tell the queue that we are done with processing this key. This unblocks the key for other workers
	// This allows safe parallel processing because two pods with the same key are never processed in
	// parallel.
	defer w.ingressWorkqueue.Done(key)

	err := w.processIngress(key.(string))
	handleErr(err, key, w.ingressWorkqueue)
	return true
}

func (w *WarpController) processIngress(queueKey string) error {

	var key, op, name string

	identifiers := strings.SplitN(queueKey, ":", 2)
	op = identifiers[0]
	key = identifiers[1]

	identifiers = strings.SplitN(key, "/", 2)
	if len(identifiers) == 1 {
		// namespace = ""
		name = identifiers[0]
	} else if len(identifiers) == 2 {
		// namespace = identifiers[0]
		name = identifiers[1]
	}

	switch op {

	case "add":

		tunnel := w.tunnels[key]
		if tunnel != nil {
			glog.V(4).Infof("Tunnel \"%s\" (%s) already exists", key, tunnel.Config().ExternalHostname)
			// return tunnel.CheckStatus()
			return nil
		}
		ingress, err := w.ingressLister.Ingresses(w.namespace).Get(name)
		if err != nil {
			return fmt.Errorf("failed to retrieve ingress by key %q: %v", key, err)
		}

		return w.createTunnel(ingress, key)

	case "delete":

		if w.tunnels[key] == nil {
			glog.V(4).Infof("Cannot tear down non-existent tunnel \"%s\"", key)
			return nil
		}

		err := w.tunnels[key].Stop()
		if err != nil {
			return err
		}
		delete(w.tunnels, key)
		return nil

	case "update":
		// Not clear how much work we should put into watching the running state of the tunnel so
		// lets just do CheckStatus here every time we see an ingress update
		tunnel := w.tunnels[key]

		if tunnel == nil {
			glog.V(4).Infof("Ingress %s is missing a tunnel, creating now", key)

			ingress, err := w.ingressLister.Ingresses(w.namespace).Get(name)
			if err != nil {
				return fmt.Errorf("failed to retrieve ingress by key %q: %v", key, err)
			}
			return w.createTunnel(ingress, key)
		}

		// return tunnel.CheckStatus()
		return nil

	default:
		return fmt.Errorf("Unhandled operation \"%s\"", op)

	}
}

func (w *WarpController) runServiceWorker() {
	for w.processNextService() {
	}
}

func (w *WarpController) processNextService() bool {

	key, quit := w.serviceWorkqueue.Get()
	if quit {
		return false
	}
	defer w.serviceWorkqueue.Done(key)

	err := w.processService(key.(string))
	handleErr(err, key, w.serviceWorkqueue)
	return true
}

func (w *WarpController) processService(queueKey string) error {

	// var key, op, name string
	var name string

	identifiers := strings.SplitN(queueKey, ":", 2)
	op := identifiers[0]
	key := identifiers[1]

	identifiers = strings.SplitN(key, "/", 2)
	if len(identifiers) == 1 {
		// namespace = ""
		name = identifiers[0]
	} else if len(identifiers) == 2 {
		// namespace = identifiers[0]
		name = identifiers[1]
	}

	t := w.getTunnelForService(name)
	if t == nil {
		return nil
	}

	switch op {

	case "add":
		return w.startOrStop(key)

	case "delete":
		return t.Stop()

	case "update":
		return w.startOrStop(key)

	default:
		return fmt.Errorf("Unhandled operation \"%s\", %s", op, name)

	}
	return nil
}

func (w *WarpController) getTunnelForService(servicename string) tunnel.Tunnel {
	for _, t := range w.tunnels {
		if servicename == t.Config().ServiceName {
			return t
		}
	}
	return nil

}

func handleErr(err error, key interface{}, queue workqueue.RateLimitingInterface) {
	if err == nil {
		// Forget about the #AddRateLimited history of the key on every successful synchronization.
		// This ensures that future processing of updates for this key is not delayed because of
		// an outdated error history.
		queue.Forget(key)
		return
	}

	// This controller retries 5 times if something goes wrong...
	if queue.NumRequeues(key) < 5 {
		glog.Infof("Error processing %v: %v", key, err)

		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		queue.AddRateLimited(key)
		return
	}

	queue.Forget(key)
	glog.Errorf("Dropping object %q out of the queue: %v", key, err)
}

func (w *WarpController) createTunnel(ingress *v1beta1.Ingress, key string) error {

	glog.V(4).Infof("creating tunnel for ingress %s, key %s", ingress.GetName(), key)

	rules := ingress.Spec.Rules
	if len(rules) > 1 {
		return fmt.Errorf("Cannot create tunnel for ingress with multiple rules")
	}
	hostname := rules[0].Host
	paths := rules[0].HTTP.Paths
	if len(paths) > 1 {
		return fmt.Errorf("Cannot create tunnel for ingress with multiple paths")
	}
	servicename := paths[0].Backend.ServiceName
	// paths[0].Backend.ServicePort will be needed if we use Endpoints instead

	configMapName := "cloudflare-warp" // make configurable to support multiple users in a namespace
	config := &tunnel.Config{
		ServiceName:      servicename,
		Namespace:        w.namespace,
		ExternalHostname: hostname,
		CertificateName:  configMapName,
	}

	// tunnel, err := tunnel.NewTunnelPodManager(w.client, config)
	tunnel, err := tunnel.NewWarpManager(config)

	if err != nil {
		return err
	}
	w.tunnels[key] = tunnel

	return w.startOrStop(key)
}

// starts or stops the tunnel depending on the existence of
// the associated service and endpoints
func (w *WarpController) startOrStop(key string) error {
	glog.V(5).Infof("Start or Stop %s", key)

	t := w.tunnels[key]
	if t == nil {
		return fmt.Errorf("Tunnel not found for key %s", key)
	}
	// check service
	servicename := t.Config().ServiceName
	service, err := w.serviceLister.Services(w.namespace).Get(servicename)
	if service == nil || err != nil {
		glog.V(5).Infof("Service %s not found for tunnel", servicename)
		if t.Active() {
			return t.Stop()
		}
		return nil
	}
	endpoints, err := w.endpointsLister.Endpoints(w.namespace).Get(servicename)
	if err != nil || endpoints == nil || len(endpoints.Subsets) == 0 {
		glog.V(5).Infof("Endpoints %s not found for tunnel", servicename)

		if t.Active() {
			return t.Stop()
		}
		return nil
	}

	glog.V(5).Infof("Validation ok for starting %s/%s/%d", servicename, endpoints.Name, len(endpoints.Subsets))
	if !t.Active() {
		return t.Start()
	}
	return nil
}

func (w *WarpController) tearDown() error {
	glog.V(4).Infof("Tearing down tunnels")

	for _, t := range w.tunnels {
		t.TearDown()
	}
	w.tunnels = make(map[string]tunnel.Tunnel)
	return nil
}
