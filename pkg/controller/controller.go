package controller

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cloudflare/cloudflare-ingress-controller/pkg/tunnel"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	lister_v1 "k8s.io/client-go/listers/core/v1"
	lister_v1beta1 "k8s.io/client-go/listers/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// ArgoController object
type ArgoController struct {
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

	// namespace of the controller
	namespace string

	// map of key to tunnels
	// where key concatenates the namespace, ingressname and servicename
	mux     sync.Mutex
	tunnels map[string]tunnel.Tunnel
}

type Config struct {
	IngressClass   string
	KubeconfigPath string
	Namespace      string
	MaxRetries     int
}

func NewArgoController(client kubernetes.Interface, config *Config) *ArgoController {

	informer, indexer, queue := createIngressInformer(client, config.IngressClass)
	tunnels := make(map[string]tunnel.Tunnel, 0)

	argo := &ArgoController{
		client: client,

		metricsConfig: tunnel.NewDummyMetrics(),

		ingressInformer:  informer,
		ingressWorkqueue: queue,
		ingressLister:    lister_v1beta1.NewIngressLister(indexer),

		namespace: config.Namespace,
		mux:       sync.Mutex{},
		tunnels:   tunnels,
	}
	argo.configureServiceInformer()
	argo.configureEndpointInformer()

	return argo
}

func (argo *ArgoController) getTunnel(key string) tunnel.Tunnel {
	argo.mux.Lock()
	defer argo.mux.Unlock()
	return argo.tunnels[key]
}

func (argo *ArgoController) setTunnel(key string, t tunnel.Tunnel) {
	argo.mux.Lock()
	defer argo.mux.Unlock()
	argo.tunnels[key] = t
}

func (argo *ArgoController) getTunnelsForService(namespace, serviceName string) []string {
	argo.mux.Lock()
	defer argo.mux.Unlock()
	var keys []string
	for key, t := range argo.tunnels {
		if serviceName == t.Config().ServiceName && namespace == t.Config().ServiceNamespace {
			keys = append(keys, key)
		}
	}
	return keys
}

// EnableMetrics configures a new metrics config for the controller
func (argo *ArgoController) EnableMetrics() {
	argo.metricsConfig = tunnel.NewMetrics(tunnel.ArgoMetricsLabelKeys())
}

func createIngressInformer(client kubernetes.Interface, ingressClass string) (cache.Controller, cache.Indexer, workqueue.RateLimitingInterface) {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	shouldHandleIngress := handleIngressFunction(ingressClass)

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
				oldKey, oldOK := shouldHandleIngress(old)
				newKey, newOK := shouldHandleIngress(new)
				if oldOK || newOK {
					if oldOK && !newOK {
						queue.Add("delete:" + oldKey)
					} else if !oldOK && newOK {
						queue.Add("add:" + newKey)
					} else {
						if oldKey != newKey {
							queue.Add("delete:" + oldKey)
							queue.Add("add:" + newKey)
						} else {
							queue.Add("update:" + newKey)
						}
					}
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
func handleIngressFunction(ingressClass string) func(obj interface{}) (string, bool) {
	return func(obj interface{}) (string, bool) {

		ingress, ok := obj.(*v1beta1.Ingress)
		if !ok {
			glog.V(5).Infof("Object is not an ingress, don't handle")
			return "", false
		}
		val, ok := ingress.Annotations[IngressClassKey]
		if !ok {
			glog.V(5).Infof("No annotation found for %s", IngressClassKey)
			return "", false
		}
		glog.V(5).Infof("Annotation %s=%s", IngressClassKey, val)
		if val != ingressClass {
			return "", false
		}
		return constructIngressKey(ingress), true
	}
}

func (argo *ArgoController) configureServiceInformer() {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	indexer, informer := cache.NewIndexerInformer(

		&cache.ListWatch{
			ListFunc: func(lo meta_v1.ListOptions) (runtime.Object, error) {
				return argo.client.CoreV1().Services(v1.NamespaceAll).List(lo)
			},
			WatchFunc: func(lo meta_v1.ListOptions) (watch.Interface, error) {
				return argo.client.CoreV1().Services(v1.NamespaceAll).Watch(lo)
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
				if !ok || !argo.isWatchedService(svc) {
					return
				}
				queue.Add("add:" + constructServiceKey(svc))
			},
			UpdateFunc: func(old, new interface{}) {
				svc, ok := new.(*v1.Service)
				if !ok || !argo.isWatchedService(svc) {
					return
				}
				queue.Add("update:" + constructServiceKey(svc))
			},
			DeleteFunc: func(obj interface{}) {
				svc, ok := obj.(*v1.Service)
				if !ok || !argo.isWatchedService(svc) {
					return
				}
				queue.Add("delete:" + constructServiceKey(svc))
			},
		},
		cache.Indexers{},
	)
	argo.serviceInformer = informer
	argo.serviceLister = lister_v1.NewServiceLister(indexer)
	argo.serviceWorkqueue = queue
}

// is this service one of the ones we have a tunnel for?
func (argo *ArgoController) isWatchedService(service *v1.Service) bool {
	for _, tunnel := range argo.tunnels {
		if service.ObjectMeta.Name == tunnel.Config().ServiceName && service.ObjectMeta.Namespace == tunnel.Config().ServiceNamespace {
			glog.V(5).Infof("Watching service %s/%s", service.ObjectMeta.Namespace, service.ObjectMeta.Name)
			return true
		}
	}
	return false
}

func (argo *ArgoController) configureEndpointInformer() {

	indexer, informer := cache.NewIndexerInformer(

		&cache.ListWatch{
			ListFunc: func(lo meta_v1.ListOptions) (runtime.Object, error) {
				return argo.client.CoreV1().Endpoints(v1.NamespaceAll).List(lo)
			},
			WatchFunc: func(lo meta_v1.ListOptions) (watch.Interface, error) {
				return argo.client.CoreV1().Endpoints(v1.NamespaceAll).Watch(lo)
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
				if !ok || !argo.isWatchedEndpoint(ep) {
					return
				}
				argo.serviceWorkqueue.Add("update:" + constructEndpointKey(ep))
			},
			UpdateFunc: func(old, new interface{}) {
				ep, ok := new.(*v1.Endpoints)
				if !ok || !argo.isWatchedEndpoint(ep) {
					return
				}
				argo.serviceWorkqueue.Add("update:" + constructEndpointKey(ep))
			},
			DeleteFunc: func(obj interface{}) {
				ep, ok := obj.(*v1.Endpoints)
				if !ok || !argo.isWatchedEndpoint(ep) {
					return
				}
				argo.serviceWorkqueue.Add("update:" + constructEndpointKey(ep))
			},
		},
		cache.Indexers{},
	)
	argo.endpointsInformer = informer
	argo.endpointsLister = lister_v1.NewEndpointsLister(indexer)
}

// is this endpoint interesting?
func (argo *ArgoController) isWatchedEndpoint(ep *v1.Endpoints) bool {

	// XXX fix this synchronization
	argo.mux.Lock()
	defer argo.mux.Unlock()

	for _, tunnel := range argo.tunnels {
		if ep.ObjectMeta.Name == tunnel.Config().ServiceName {
			glog.V(5).Infof("Watching endpoint %s/%s", ep.ObjectMeta.Namespace, ep.ObjectMeta.Name)
			return true
		}
	}
	return false
}

func (argo *ArgoController) Run(stopCh chan struct{}) {
	defer argo.ingressWorkqueue.ShutDown()
	defer argo.serviceWorkqueue.ShutDown()

	glog.Info("Starting ArgoController")

	go argo.serviceInformer.Run(stopCh)
	go argo.endpointsInformer.Run(stopCh)
	go argo.ingressInformer.Run(stopCh)

	// Wait for all caches to be synced, before processing items from the queue is started
	if !cache.WaitForCacheSync(stopCh, argo.ingressInformer.HasSynced) {
		glog.Error(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}
	if !cache.WaitForCacheSync(stopCh, argo.serviceInformer.HasSynced) {
		glog.Error(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	go wait.Until(argo.runIngressWorker, time.Second, stopCh)
	go wait.Until(argo.runServiceWorker, time.Second, stopCh)

	<-stopCh
	glog.Info("Stopping ArgoController ")
	argo.tearDown()
}

func (argo *ArgoController) runIngressWorker() {
	for argo.processNextIngress() {
	}
}

func (argo *ArgoController) processNextIngress() bool {

	key, quit := argo.ingressWorkqueue.Get()
	if quit {
		return false
	}
	defer argo.ingressWorkqueue.Done(key)

	err := argo.processIngress(key.(string))
	handleErr(err, key, argo.ingressWorkqueue)
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
	return constructIngressKeyFromStrings(ingress.ObjectMeta.Namespace, ingress.ObjectMeta.Name, ingress.Spec.Rules[0].HTTP.Paths[0].Backend.ServiceName)
}
func constructIngressKeyFromStrings(namespace, ingressname, servicename string) string {
	return fmt.Sprintf("%s/%s/%s", namespace, ingressname, servicename)
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

func (argo *ArgoController) processIngress(queueKey string) error {

	op, namespace, ingressname, servicename := parseIngressKey(queueKey)

	switch op {

	case "add":

		ingress, err := argo.ingressLister.Ingresses(namespace).Get(ingressname)
		if err != nil {
			all, _ := argo.ingressLister.Ingresses(namespace).List(labels.Everything())
			glog.V(2).Infof("all ingresses in %s: %v", "*", all)
			return fmt.Errorf("failed to retrieve ingress by name %q: %v", ingressname, err)
		}
		key := constructIngressKey(ingress)

		tunnel := argo.getTunnel(key)
		if tunnel != nil {
			glog.V(5).Infof("Tunnel \"%s\" (%s) already exists", key, tunnel.Config().ExternalHostname)
			return nil
		}

		return argo.createTunnel(key, ingress)

	case "delete":
		key := constructIngressKeyFromStrings(namespace, ingressname, servicename)
		return argo.removeTunnel(key)

	case "update":

		ingress, err := argo.ingressLister.Ingresses(namespace).Get(ingressname)
		if err != nil {
			all, _ := argo.ingressLister.Ingresses(namespace).List(labels.Everything())
			glog.V(2).Infof("all ingresses in %s: %v", "*", all)
			return fmt.Errorf("failed to retrieve ingress by name %q: %v", ingressname, err)
		}
		key := constructIngressKey(ingress)

		return argo.updateTunnel(key, ingress)

	default:
		return fmt.Errorf("Unhandled operation \"%s\"", op)

	}
}

func (argo *ArgoController) runServiceWorker() {
	for argo.processNextService() {
	}
}

func (argo *ArgoController) processNextService() bool {

	key, quit := argo.serviceWorkqueue.Get()
	if quit {
		return false
	}
	defer argo.serviceWorkqueue.Done(key)

	err := argo.processService(key.(string))
	handleErr(err, key, argo.serviceWorkqueue)
	return true
}

func (argo *ArgoController) processService(queueKey string) error {

	_, namespace, serviceName := parseServiceKey(queueKey)

	keys := argo.getTunnelsForService(namespace, serviceName)
	if len(keys) == 0 {
		// no tunnels or ingresses exist for this service
		return nil
	}

	var errorMessage string
	for _, key := range keys {
		err := argo.evaluateTunnelStatus(key)
		if err != nil {
			if errorMessage == "" {
				errorMessage = err.Error()
			} else {
				errorMessage = errorMessage + "; " + err.Error()
			}
		}
	}
	if errorMessage != "" {
		return fmt.Errorf("at least one error occurred handling %s: %s", queueKey, errorMessage)
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
func (argo *ArgoController) validateIngress(ingress *v1beta1.Ingress) error {
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
func (argo *ArgoController) getServiceNameForIngress(ingress *v1beta1.Ingress) string {
	return ingress.Spec.Rules[0].HTTP.Paths[0].Backend.ServiceName
}

// assumes validation
func (argo *ArgoController) getServicePortForIngress(ingress *v1beta1.Ingress) intstr.IntOrString {
	return ingress.Spec.Rules[0].HTTP.Paths[0].Backend.ServicePort
}

// assumes validation
func (argo *ArgoController) getHostNameForIngress(ingress *v1beta1.Ingress) string {
	return ingress.Spec.Rules[0].Host
}

// assumes validation
func (argo *ArgoController) getLBPoolForIngress(ingress *v1beta1.Ingress) string {
	// if the value of LBPool is "", caller should assume that loadbalancing is disabled
	return ingress.ObjectMeta.Annotations[IngressAnnotationLBPool]
}

func (argo *ArgoController) readSecret(hostname string) (*v1.Secret, error) {

	var certSecret *v1.Secret

	elements := strings.Split(hostname, ".")
	// decrementing is overkill, because the only valid choice is one level down.
	for i := range elements {
		domain := strings.Join(elements[i:], ".")
		certSecretList, err := argo.client.CoreV1().Secrets(argo.namespace).List(
			meta_v1.ListOptions{
				LabelSelector: SecretLabelDomain + "=" + domain,
			},
		)
		if err != nil {
			return nil, err
		}
		if len(certSecretList.Items) > 0 {
			secret := certSecretList.Items[0]
			glog.V(5).Infof("Secret %s found for label %s=%s", secret.ObjectMeta.Name, SecretLabelDomain, domain)
			return &secret, nil
		}
	}

	glog.V(5).Infof("Secret not found for label %s, hostname %s, trying default name %s", SecretLabelDomain, hostname, SecretName)
	certSecret, err := argo.client.CoreV1().Secrets(argo.namespace).Get(SecretName, meta_v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return certSecret, nil
}

// obtains the origin cert for a particular hostname
func (argo *ArgoController) readOriginCert(hostname string) ([]byte, error) {
	// in the future, we will have multiple secrets
	// at present the mapping of hostname -> secretname is "*" -> "cloudflared-cert"

	// certSecret, err := argo.client.CoreV1().Secrets(argo.namespace).Get(SecretName, meta_v1.GetOptions{})
	certSecret, err := argo.readSecret(hostname)
	if err != nil {
		return []byte{}, err
	}
	certFileName := "cert.pem"
	originCert := certSecret.Data[certFileName]
	if len(originCert) == 0 {
		return []byte{}, fmt.Errorf("Certificate data not found for host %s in secret %s/%s", hostname, SecretName, certFileName)
	}
	return originCert, nil
}

// creates a tunnel and stores a reference to it by serviceName
func (argo *ArgoController) createTunnel(key string, ingress *v1beta1.Ingress) error {
	err := argo.validateIngress(ingress)
	if err != nil {
		return err
	}
	ingressName := ingress.GetName()
	serviceName := argo.getServiceNameForIngress(ingress)
	// key := fmt.Sprintf("%s/%s", ingress.ObjectMeta.Namespace, serviceName)
	glog.V(5).Infof("creating tunnel for ingress %s, %s", ingressName, key)

	servicePort := argo.getServicePortForIngress(ingress)
	hostName := argo.getHostNameForIngress(ingress)
	lbPool := argo.getLBPoolForIngress(ingress)

	originCert, err := argo.readOriginCert(hostName)
	if err != nil {
		return err
	}

	config := &tunnel.Config{
		ServiceName:      serviceName,
		ServiceNamespace: ingress.ObjectMeta.Namespace,
		ServicePort:      servicePort,
		IngressName:      ingress.ObjectMeta.Name,
		ExternalHostname: hostName,
		LBPool:           lbPool,
		OriginCert:       originCert,
	}

	tunnel, err := tunnel.NewArgoTunnelManager(config, argo.metricsConfig)
	if err != nil {
		return err
	}
	argo.setTunnel(key, tunnel)
	glog.V(5).Infof("created tunnel for ingress %s,  %s", ingressName, key)

	return argo.evaluateTunnelStatus(key)
}

// updates a tunnel and stores a reference to it by serviceName
func (argo *ArgoController) updateTunnel(key string, ingress *v1beta1.Ingress) error {
	err := argo.validateIngress(ingress)
	if err != nil {
		argo.removeTunnel(key)
		return err
	}
	t := argo.getTunnel(key)
	if t == nil {
		return argo.createTunnel(key, ingress)
	}

	servicePort := argo.getServicePortForIngress(ingress)
	serviceName := argo.getServiceNameForIngress(ingress)
	hostName := argo.getHostNameForIngress(ingress)
	lbPool := argo.getLBPoolForIngress(ingress)

	config := t.Config()
	if config.LBPool != lbPool || config.ExternalHostname != hostName || config.ServicePort != servicePort || config.ServiceName != serviceName {
		glog.V(2).Infof("Ingress parameters have changed, recreating tunnel for %s", key)
		argo.removeTunnel(key)
		return argo.createTunnel(key, ingress)
	}
	return nil
}

// starts or stops the tunnel depending on the existence of
// the associated service and endpoints
func (argo *ArgoController) evaluateTunnelStatus(key string) error {

	t := argo.getTunnel(key)
	if t == nil {
		return fmt.Errorf("Tunnel not found for key %s", key)
	}

	serviceName := t.Config().ServiceName
	namespace := t.Config().ServiceNamespace

	service, err := argo.serviceLister.Services(namespace).Get(serviceName)
	if service == nil || err != nil {
		glog.V(2).Infof("Service %s not found for tunnel", key)
		return argo.stopTunnel(t)
	}
	endpoints, err := argo.endpointsLister.Endpoints(namespace).Get(serviceName)

	if err != nil || endpoints == nil {
		glog.V(2).Infof("Endpoints not found for tunnel %s", key)
		return argo.stopTunnel(t)
	}
	readyEndpointCount := 0
	for _, subset := range endpoints.Subsets {
		readyEndpointCount = readyEndpointCount + len(subset.Addresses)
	}
	if readyEndpointCount == 0 {
		glog.V(2).Infof("Endpoints not ready for tunnel %s", key)
		return argo.stopTunnel(t)
	}

	glog.V(5).Infof("Validation ok for running %s with %d endpoint(s)", key, readyEndpointCount)
	if !t.Active() {
		return argo.startTunnel(t, service)
	}

	return nil
}

func (argo *ArgoController) startTunnel(t tunnel.Tunnel, service *v1.Service) error {

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
		return fmt.Errorf("Unable to match port %s to service %s", ingressServicePort.String(), service.ObjectMeta.Name)
	}
	url := fmt.Sprintf("%s.%s:%d", service.ObjectMeta.Name, service.ObjectMeta.Namespace, port)
	glog.V(5).Infof("Starting tunnel to url %s", url)
	err := t.Start(url)

	if err != nil {
		return err
	}
	return argo.setIngressEndpoint(t, t.Config().ExternalHostname)
}

func (argo *ArgoController) setIngressEndpoint(t tunnel.Tunnel, hostname string) error {
	namespace := t.Config().ServiceNamespace
	ingressName := t.Config().IngressName
	ingressClient := argo.client.ExtensionsV1beta1().Ingresses(namespace)
	currentIngress, err := ingressClient.Get(ingressName, meta_v1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("cannot find ingress %v/%v", namespace, ingressName))
	}

	var lbIngressSet []v1.LoadBalancerIngress
	if hostname != "" {
		lbIngressSet = []v1.LoadBalancerIngress{
			{
				Hostname: hostname,
			},
		}
	} else {
		lbIngressSet = []v1.LoadBalancerIngress{}
	}

	currentIngress.Status = v1beta1.IngressStatus{
		LoadBalancer: v1.LoadBalancerStatus{
			Ingress: lbIngressSet,
		},
	}
	_, err = ingressClient.UpdateStatus(currentIngress)
	if err != nil {
		glog.V(2).Infof("error updating ingress, %v", err)
	}
	return err
}

func (argo *ArgoController) stopTunnel(t tunnel.Tunnel) error {
	if t.Active() {
		err := t.Stop()
		if err != nil {
			glog.V(2).Infof("Error stopping tunnel, %v", err)
			return err
		}
		return argo.setIngressEndpoint(t, "")
	}
	return nil
}

func (argo *ArgoController) removeTunnel(key string) error {
	glog.V(5).Infof("Removing tunnel %s", key)
	t := argo.getTunnel(key)
	if t == nil {
		return fmt.Errorf("Tunnel not found for key %s", key)
	}
	err := argo.stopTunnel(t)
	argo.mux.Lock()
	delete(argo.tunnels, key)
	argo.mux.Unlock()
	return err
}

func (argo *ArgoController) tearDown() error {
	glog.V(5).Infof("Tearing down tunnels")

	for _, t := range argo.tunnels {
		t.TearDown()
	}
	argo.mux.Lock()
	argo.tunnels = make(map[string]tunnel.Tunnel)
	argo.mux.Unlock()
	return nil
}
