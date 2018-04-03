package controller

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/golang/glog"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	// v1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"k8s.io/apimachinery/pkg/util/intstr"

	lister_v1 "k8s.io/client-go/listers/core/v1"
	lister_v1beta1 "k8s.io/client-go/listers/extensions/v1beta1"

	"github.com/cloudflare/cloudflare-warp-ingress/pkg/tunnel"
)

const (
	maxRetries                = 5
	ingressClassKey           = "kubernetes.io/ingress.class"
	cloudflareWarpIngressType = "cloudflare-warp"
	ingressAnnotationLBPool   = "warp.cloudflare.com/lb-pool"
	secretLabelDomain         = "warp.cloudflare.com/domain"
)

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

	informer, indexer, queue := createIngressInformer(client)
	tunnels := make(map[string]tunnel.Tunnel, 0)

	w := &WarpController{
		client: client,

		metricsConfig: tunnel.NewDummyMetrics(),

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

// EnableMetrics configures a new metrics config for the controller
func (w *WarpController) EnableMetrics() {
	w.metricsConfig = tunnel.NewMetrics()
}

func createIngressInformer(client kubernetes.Interface) (cache.Controller, cache.Indexer, workqueue.RateLimitingInterface) {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	indexer, informer := cache.NewIndexerInformer(

		&cache.ListWatch{
			ListFunc: func(lo meta_v1.ListOptions) (runtime.Object, error) {
				return client.ExtensionsV1beta1().Ingresses(v1.NamespaceAll).List(lo)
			},
			WatchFunc: func(lo meta_v1.ListOptions) (watch.Interface, error) {
				return client.ExtensionsV1beta1().Ingresses(v1.NamespaceAll).Watch(lo)
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
// key is: namespace+"/"+ingressname+"/"+serviceName
// allows lookup by either the ingress name or the service name
func shouldHandleIngress(obj interface{}) (string, bool) {

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
	if val != cloudflareWarpIngressType {
		return "", false
	}

	rules := ingress.Spec.Rules
	if len(rules) == 0 {
		glog.V(2).Infof("Cannot create tunnel for ingress with no rules")
		return "", false
	}
	if len(rules) > 1 {
		glog.V(2).Infof("Cannot create tunnel for ingress with multiple rules")
		return "", false
	}
	paths := rules[0].HTTP.Paths
	if len(paths) == 0 {
		glog.V(2).Infof("Cannot create tunnel for ingress with no paths")
		return "", false
	}
	if len(paths) > 1 {
		glog.V(2).Infof("Cannot create tunnel for ingress with multiple paths")
		return "", false
	}
	return constructIngressKey(ingress), true
}

func (w *WarpController) configureServiceInformer() {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	indexer, informer := cache.NewIndexerInformer(

		&cache.ListWatch{
			ListFunc: func(lo meta_v1.ListOptions) (runtime.Object, error) {
				return w.client.CoreV1().Services(v1.NamespaceAll).List(lo)
			},
			WatchFunc: func(lo meta_v1.ListOptions) (watch.Interface, error) {
				return w.client.CoreV1().Services(v1.NamespaceAll).Watch(lo)
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
				queue.Add("add:" + constructServiceKey(svc))
			},
			UpdateFunc: func(old, new interface{}) {
				svc, ok := new.(*v1.Service)
				if !ok || !w.isWatchedService(svc) {
					return
				}
				queue.Add("update:" + constructServiceKey(svc))
			},
			DeleteFunc: func(obj interface{}) {
				svc, ok := obj.(*v1.Service)
				if !ok || !w.isWatchedService(svc) {
					return
				}
				queue.Add("delete:" + constructServiceKey(svc))
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
		if service.ObjectMeta.Name == tunnel.Config().ServiceName && service.ObjectMeta.Namespace == tunnel.Config().ServiceNamespace {
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
				return w.client.CoreV1().Endpoints(v1.NamespaceAll).List(lo)
			},
			WatchFunc: func(lo meta_v1.ListOptions) (watch.Interface, error) {
				return w.client.CoreV1().Endpoints(v1.NamespaceAll).Watch(lo)
			},
		},

		// The types of objects this informer will return
		&v1.Endpoints{},

		// The resync period of this object.
		60*time.Second,

		// Queue all these changes as an update to the service using the endpoint ns/name == service ns/name
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				ep, ok := obj.(*v1.Endpoints)
				if !ok || !w.isWatchedEndpoint(ep) {
					return
				}
				w.serviceWorkqueue.Add("update:" + constructEndpointKey(ep))
			},
			UpdateFunc: func(old, new interface{}) {
				ep, ok := new.(*v1.Endpoints)
				if !ok || !w.isWatchedEndpoint(ep) {
					return
				}
				w.serviceWorkqueue.Add("update:" + constructEndpointKey(ep))
			},
			DeleteFunc: func(obj interface{}) {
				ep, ok := obj.(*v1.Endpoints)
				if !ok || !w.isWatchedEndpoint(ep) {
					return
				}
				w.serviceWorkqueue.Add("update:" + constructEndpointKey(ep))
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

func constructServiceKey(service *v1.Service) string {
	return fmt.Sprintf("%s/%s", service.ObjectMeta.Namespace, service.ObjectMeta.Name)
}

func constructEndpointKey(ep *v1.Endpoints) string {
	return fmt.Sprintf("%s/%s", ep.ObjectMeta.Namespace, ep.ObjectMeta.Name)
}

func parseServiceKey(queueKey string) (string, string, string) {

	identifiers := strings.SplitN(queueKey, ":", 2)
	op := identifiers[0]
	key := identifiers[1]

	identifiers = strings.SplitN(key, "/", 2)
	namespace := identifiers[0]
	serviceName := identifiers[1]

	return op, namespace, serviceName
}

func constructIngressKey(ingress *v1beta1.Ingress) string {
	return fmt.Sprintf("%s/%s/%s", ingress.ObjectMeta.Namespace, ingress.ObjectMeta.Name, ingress.Spec.Rules[0].HTTP.Paths[0].Backend.ServiceName)
}

func parseIngressKey(queueKey string) (string, string, string, string) {

	identifiers := strings.SplitN(queueKey, ":", 2)
	op := identifiers[0]
	key := identifiers[1]

	identifiers = strings.SplitN(key, "/", 3)
	namespace := identifiers[0]
	ingressName := identifiers[1]
	serviceName := identifiers[2]

	return op, namespace, ingressName, serviceName
}

func (w *WarpController) processIngress(queueKey string) error {

	op, namespace, ingressname, serviceName := parseIngressKey(queueKey)
	key := namespace + "/" + serviceName

	switch op {

	case "add":

		ingress, err := w.ingressLister.Ingresses(namespace).Get(ingressname)
		tunnel := w.tunnels[key]
		if tunnel != nil {
			glog.V(5).Infof("Tunnel \"%s\" (%s) already exists", serviceName, tunnel.Config().ExternalHostname)
			// return tunnel.CheckStatus()
			return nil
		}
		if err != nil {

			all, _ := w.ingressLister.Ingresses(namespace).List(labels.Everything())
			glog.V(2).Infof("all ingresses in %s: %v", "*", all)

			return fmt.Errorf("failed to retrieve ingress by name %q: %v", ingressname, err)
		}

		return w.createTunnel(ingress)

	case "delete":

		return w.removeTunnel(namespace, serviceName)

	case "update":
		// Not clear how much work we should put into watching the running state of the tunnel so
		// lets just do CheckStatus here every time we see an ingress update
		//
		// if the ingress has been edited to change the hostname, we should update
		tunnel := w.tunnels[key]

		if tunnel == nil {
			glog.V(5).Infof("Ingress %s is missing a tunnel, creating now", serviceName)

			ingress, err := w.ingressLister.Ingresses(namespace).Get(ingressname)
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

	op, namespace, serviceName := parseServiceKey(queueKey)

	t, found := w.getTunnelForService(namespace, serviceName)
	if !found {
		return nil
	}

	switch op {

	case "add":
		return w.startOrStop(namespace, serviceName)

	case "delete":
		return t.Stop()

	case "update":
		return w.startOrStop(namespace, serviceName)

	default:
		return fmt.Errorf("Unhandled operation \"%s\", %s", op, serviceName)

	}
}

func (w *WarpController) getTunnelForService(namespace, serviceName string) (tunnel.Tunnel, bool) {
	for _, t := range w.tunnels {
		if serviceName == t.Config().ServiceName && namespace == t.Config().ServiceNamespace {
			return t, true
		}
	}
	return nil, false
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
	if len(rules) == 0 {
		return fmt.Errorf("Cannot create tunnel for ingress with no rules")
	}
	if len(rules) > 1 {
		return fmt.Errorf("Cannot create tunnel for ingress with multiple rules")
	}
	paths := rules[0].HTTP.Paths
	if len(paths) == 0 {
		return fmt.Errorf("Cannot create tunnel for ingress with no paths")
	}
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
func (w *WarpController) getServicePortForIngress(ingress *v1beta1.Ingress) intstr.IntOrString {
	return ingress.Spec.Rules[0].HTTP.Paths[0].Backend.ServicePort
}

// assumes validation
func (w *WarpController) getHostNameForIngress(ingress *v1beta1.Ingress) string {
	return ingress.Spec.Rules[0].Host
}

// assumes validation
func (w *WarpController) getLBPoolForIngress(ingress *v1beta1.Ingress) string {
	// // disabled: multiple pools per hostname is not yet supported in cloudflare-warp
	// // instead, use the hostname itself as the name of the pool
	//
	// lbPoolName := ingress.ObjectMeta.Annotations[ingressAnnotationLBPool]
	// if lbPoolName == "" {
	// 	lbPoolName = w.getServiceNameForIngress(ingress) + "." + ingress.ObjectMeta.Namespace
	// }
	// return lbPoolName
	//
	return w.getHostNameForIngress(ingress)
}

func (w *WarpController) readSecret(hostname string) (*v1.Secret, error) {

	var certSecret *v1.Secret
	var certSecretList *v1.SecretList
	// loop over decrements of the hostname
	certSecretList, err := w.client.CoreV1().Secrets(w.namespace).List(
		meta_v1.ListOptions{
			LabelSelector: secretLabelDomain + "=" + hostname,
		},
	)
	if err != nil {
		return nil, err
	}
	if len(certSecretList.Items) > 0 {
		return &certSecretList.Items[0], nil
	}

	secretName := "cloudflare-warp-cert"

	certSecret, err = w.client.CoreV1().Secrets(w.namespace).Get(secretName, meta_v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return certSecret, nil
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

// creates a tunnel and stores a reference to it by serviceName
func (w *WarpController) createTunnel(ingress *v1beta1.Ingress) error {
	err := w.validateIngress(ingress)
	if err != nil {
		return err
	}
	glog.V(5).Infof("creating tunnel for ingress %s", ingress.GetName())
	serviceName := w.getServiceNameForIngress(ingress)
	servicePort := w.getServicePortForIngress(ingress)
	hostName := w.getHostNameForIngress(ingress)
	lbPool := w.getLBPoolForIngress(ingress)

	originCert, err := w.readOriginCert(hostName)
	if err != nil {
		return err
	}

	config := &tunnel.Config{
		ServiceName:      serviceName,
		ServiceNamespace: ingress.ObjectMeta.Namespace,
		ServicePort:      servicePort,
		ExternalHostname: hostName,
		LBPool:           lbPool,
		OriginCert:       originCert,
	}

	tunnel, err := tunnel.NewWarpManager(config, w.metricsConfig)

	if err != nil {
		return err
	}
	key := fmt.Sprintf("%s/%s", ingress.ObjectMeta.Namespace, serviceName)
	w.tunnels[key] = tunnel
	glog.V(5).Infof("added tunnel for ingress %s, service %s", ingress.GetName(), serviceName)

	return w.startOrStop(ingress.ObjectMeta.Namespace, serviceName)
}

// starts or stops the tunnel depending on the existence of
// the associated service and endpoints
func (w *WarpController) startOrStop(namespace, serviceName string) error {
	glog.V(5).Infof("Start or Stop %s", serviceName)

	key := fmt.Sprintf("%s/%s", namespace, serviceName)
	t := w.tunnels[key]
	if t == nil {
		return fmt.Errorf("Tunnel not found for key %s", key)
	}

	service, err := w.serviceLister.Services(namespace).Get(serviceName)
	if service == nil || err != nil {
		glog.V(2).Infof("Service %s not found for tunnel", key)
		if t.Active() {
			return t.Stop()
		}
		return nil
	}
	endpoints, err := w.endpointsLister.Endpoints(namespace).Get(serviceName)
	if err != nil || endpoints == nil || len(endpoints.Subsets) == 0 {
		glog.V(2).Infof("Endpoints %s not found for tunnel", key)

		if t.Active() {
			return t.Stop()
		}
		return nil
	}

	glog.V(5).Infof("Validation ok for starting %s/%d", key, len(endpoints.Subsets))
	if !t.Active() {
		var port int32
		ingressServicePort := t.Config().ServicePort
		for _, p := range service.Spec.Ports {

			// equality
			if (ingressServicePort.Type == intstr.Int && p.Port == ingressServicePort.IntVal) ||
				(ingressServicePort.Type == intstr.String && p.Name == ingressServicePort.StrVal) {
				port = p.Port
			}
		}
		if port == 0 {
			return fmt.Errorf("Unable to match port %s to service %s", ingressServicePort.String(), key)
		}
		url := fmt.Sprintf("%s.%s:%d", service.ObjectMeta.Name, service.ObjectMeta.Namespace, port)
		glog.V(5).Infof("Starting tunnel to url %s", url)
		return t.Start(url)
	}
	return nil
}

func (w *WarpController) removeTunnel(namespace, serviceName string) error {
	key := fmt.Sprintf("%s/%s", namespace, serviceName)

	t := w.tunnels[key]
	if t == nil {
		return fmt.Errorf("Tunnel not found for key %s", key)
	}
	err := t.Stop()
	delete(w.tunnels, key)
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
