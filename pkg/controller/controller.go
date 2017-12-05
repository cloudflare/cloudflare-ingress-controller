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

	metricsConfig *tunnel.MetricsConfig

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

	metricsConfig := tunnel.NewMetrics()

	w := &WarpController{
		client: client,

		metricsConfig: metricsConfig,

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
				if key, ok := shouldHandleIngress(obj); ok {
					queue.Add("add:" + key)
				}
			},
			UpdateFunc: func(old, new interface{}) {
				if key, ok := shouldHandleIngress(old); ok {
					queue.Add("update:" + key)
				} else {
					return
				}
			},
			DeleteFunc: func(obj interface{}) {
				if key, ok := shouldHandleIngress(obj); ok {
					queue.Add("delete:" + key)
				}
			},
		},
		cache.Indexers{},
	)
	return informer, indexer, queue
}

// check the type, annotation and conditions.  Return key, ok
// key is: ingressname+"/"+servicename
// allows lookup by either the ingress name or the service name
func shouldHandleIngress(obj interface{}) (string, bool) {
	var ingressClassKey = "kubernetes.io/ingress.class"

	ingress, ok := obj.(*v1beta1.Ingress)
	if !ok {
		glog.V(5).Infof("Object is not an ingress, don't handle")
		return "", false
	}
	val, ok := ingress.Annotations[ingressClassKey]
	if !ok {
		glog.V(5).Infof("No annotation found for %s", ingressClassKey)
		return "", false
	}
	glog.V(5).Infof("Annotation %s=%s", ingressClassKey, val)
	if val != "cloudflare-warp" {
		return "", false
	}

	rules := ingress.Spec.Rules
	if len(rules) > 1 {
		glog.V(2).Infof("Cannot create tunnel for ingress with multiple rules")
		return "", false
	}
	paths := rules[0].HTTP.Paths
	if len(paths) > 1 {
		glog.V(2).Infof("Cannot create tunnel for ingress with multiple paths")
		return "", false
	}
	return fmt.Sprintf("%s/%s", ingress.ObjectMeta.Name, ingress.Spec.Rules[0].HTTP.Paths[0].Backend.ServiceName), true
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
				queue.Add("add:" + svc.ObjectMeta.Name)
			},
			UpdateFunc: func(old, new interface{}) {
				svc, ok := new.(*v1.Service)
				if !ok || !w.isWatchedService(svc) {
					return
				}
				queue.Add("update:" + svc.ObjectMeta.Name)
			},
			DeleteFunc: func(obj interface{}) {
				svc, ok := obj.(*v1.Service)
				if !ok || !w.isWatchedService(svc) {
					return
				}
				queue.Add("delete:" + svc.ObjectMeta.Name)
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

		// Queue all these changes as an update to the service using the endpoint name == service name
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				ep, ok := obj.(*v1.Endpoints)
				if !ok || !w.isWatchedEndpoint(ep) {
					return
				}
				w.serviceWorkqueue.Add("update:" + ep.ObjectMeta.Name)
			},
			UpdateFunc: func(old, new interface{}) {
				ep, ok := new.(*v1.Endpoints)
				if !ok || !w.isWatchedEndpoint(ep) {
					return
				}
				w.serviceWorkqueue.Add("update:" + ep.ObjectMeta.Name)
			},
			DeleteFunc: func(obj interface{}) {
				ep, ok := obj.(*v1.Endpoints)
				if !ok || !w.isWatchedEndpoint(ep) {
					return
				}
				w.serviceWorkqueue.Add("update:" + ep.ObjectMeta.Name)
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

func parseIngressKey(queueKey string) (string, string, string) {

	identifiers := strings.SplitN(queueKey, ":", 2)
	op := identifiers[0]
	key := identifiers[1]

	identifiers = strings.SplitN(key, "/", 2)
	ingressname := identifiers[0]
	servicename := identifiers[1]

	return op, ingressname, servicename
}

func (w *WarpController) processIngress(queueKey string) error {

	op, servicename, ingressname := parseIngressKey(queueKey)

	switch op {

	case "add":

		ingress, err := w.ingressLister.Ingresses(w.namespace).Get(ingressname)
		tunnel := w.tunnels[servicename]
		if tunnel != nil {
			glog.V(5).Infof("Tunnel \"%s\" (%s) already exists", servicename, tunnel.Config().ExternalHostname)
			// return tunnel.CheckStatus()
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to retrieve ingress by name %q: %v", ingressname, err)
		}

		return w.createTunnel(ingress)

	case "delete":

		return w.removeTunnel(servicename)

	case "update":
		// Not clear how much work we should put into watching the running state of the tunnel so
		// lets just do CheckStatus here every time we see an ingress update
		tunnel := w.tunnels[servicename]

		if tunnel == nil {
			glog.V(5).Infof("Ingress %s is missing a tunnel, creating now", servicename)

			ingress, err := w.ingressLister.Ingresses(w.namespace).Get(ingressname)
			if err != nil {
				return fmt.Errorf("failed to retrieve ingress by key %q: %v", ingressname, err)
			}
			return w.createTunnel(ingress)
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
		queue.Forget(key)
		return
	}

	// This controller retries twice if something goes wrong...
	if queue.NumRequeues(key) < 2 {
		glog.Infof("Error processing %v: %v", key, err)
		queue.AddRateLimited(key)
		return
	}

	queue.Forget(key)
	glog.Errorf("Dropping object %q out of the queue: %v", key, err)
}

// returns non-nil error if the ingress is not something we can deal with
func (w *WarpController) validateIngress(ingress *v1beta1.Ingress) error {
	rules := ingress.Spec.Rules
	if len(rules) > 1 {
		return fmt.Errorf("Cannot create tunnel for ingress with multiple rules")
	}
	paths := rules[0].HTTP.Paths
	if len(paths) > 1 {
		return fmt.Errorf("Cannot create tunnel for ingress with multiple paths")
	}
	return nil
}

// assumes validation
func (w *WarpController) getServiceNameForIngress(ingress *v1beta1.Ingress) string {
	return ingress.Spec.Rules[0].HTTP.Paths[0].Backend.ServiceName
}

// assumes validation
func (w *WarpController) getHostNameForIngress(ingress *v1beta1.Ingress) string {
	return ingress.Spec.Rules[0].Host
}

// obtains the origin cert for a particular hostname
func (w *WarpController) readOriginCert(hostname string) ([]byte, error) {
	// in the future, we will have multiple secrets
	// at present the mapping of hostname -> secretname is "*" -> "cloudflare-warp-cert"
	secretName := "cloudflare-warp-cert"

	certSecret, err := w.client.CoreV1().Secrets(w.namespace).Get(secretName, meta_v1.GetOptions{})
	if err != nil {
		return []byte{}, err
	}
	certFileName := "cert.pem"
	originCert := certSecret.Data[certFileName]
	if len(originCert) == 0 {
		return []byte{}, fmt.Errorf("Certificate data not found for host %s in secret %s/%s", hostname, secretName, certFileName)
	}
	return originCert, nil
}

// creates a tunnel and stores a reference to it by servicename
func (w *WarpController) createTunnel(ingress *v1beta1.Ingress) error {
	err := w.validateIngress(ingress)
	if err != nil {
		return err
	}
	glog.V(5).Infof("creating tunnel for ingress %s", ingress.GetName())
	serviceName := w.getServiceNameForIngress(ingress)
	hostName := w.getHostNameForIngress(ingress)
	originCert, err := w.readOriginCert(hostName)
	if err != nil {
		return err
	}

	config := &tunnel.Config{
		ServiceName:      serviceName,
		ExternalHostname: hostName,
		OriginCert:       originCert,
	}

	tunnel, err := tunnel.NewWarpManager(config, w.metricsConfig)

	if err != nil {
		return err
	}
	w.tunnels[serviceName] = tunnel
	glog.V(5).Infof("added tunnel for ingress %s, service %s", ingress.GetName(), serviceName)

	return w.startOrStop(serviceName)
}

// starts or stops the tunnel depending on the existence of
// the associated service and endpoints
func (w *WarpController) startOrStop(servicename string) error {
	glog.V(5).Infof("Start or Stop %s", servicename)

	t := w.tunnels[servicename]
	if t == nil {
		return fmt.Errorf("Tunnel not found for key %s", servicename)
	}

	service, err := w.serviceLister.Services(w.namespace).Get(servicename)
	if service == nil || err != nil {
		glog.V(2).Infof("Service %s not found for tunnel", servicename)
		if t.Active() {
			return t.Stop()
		}
		return nil
	}
	endpoints, err := w.endpointsLister.Endpoints(w.namespace).Get(servicename)
	if err != nil || endpoints == nil || len(endpoints.Subsets) == 0 {
		glog.V(2).Infof("Endpoints %s not found for tunnel", servicename)

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

func (w *WarpController) removeTunnel(servicename string) error {

	t := w.tunnels[servicename]
	if t == nil {
		return fmt.Errorf("Tunnel not found for key %s", servicename)
	}
	err := t.Stop()
	delete(w.tunnels, servicename)
	return err
}

func (w *WarpController) tearDown() error {
	glog.V(2).Infof("Tearing down tunnels")

	for _, t := range w.tunnels {
		t.TearDown()
	}
	w.tunnels = make(map[string]tunnel.Tunnel)
	return nil
}
