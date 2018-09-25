package controller

import (
	"fmt"
	"strings"
	"time"

	"github.com/cloudflare/cloudflare-ingress-controller/internal/tunnel"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	util_runtime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	lister_v1 "k8s.io/client-go/listers/core/v1"
	lister_v1beta1 "k8s.io/client-go/listers/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// TunnelController object
type TunnelController struct {
	client  kubernetes.Interface
	options options

	metricsConfig *tunnel.MetricsConfig

	ingressLister    lister_v1beta1.IngressLister
	ingressInformer  cache.Controller
	ingressWorkqueue workqueue.RateLimitingInterface

	serviceLister    lister_v1.ServiceLister
	serviceInformer  cache.Controller
	serviceWorkqueue workqueue.RateLimitingInterface

	endpointsLister   lister_v1.EndpointsLister
	endpointsInformer cache.Controller

	// tunnel registry, key = (namespace, ingressname and servicename)
	tunnels tunnel.Registry
}

func NewTunnelController(client kubernetes.Interface, options ...Option) *TunnelController {
	opts := collectOptions(options)
	informer, indexer, queue := createIngressInformer(client, opts.ingressClass)

	c := &TunnelController{
		client:        client,
		options:       opts,
		metricsConfig: tunnel.NewDummyMetrics(),

		ingressInformer:  informer,
		ingressWorkqueue: queue,
		ingressLister:    lister_v1beta1.NewIngressLister(indexer),
	}
	c.configureServiceInformer()
	c.configureEndpointInformer()

	if opts.enableMetrics {
		c.metricsConfig = tunnel.NewMetrics()
	}
	return c
}

func (c *TunnelController) getTunnelsForService(namespace, name string) []string {
	var keys []string
	c.tunnels.Range(func(k string, t tunnel.Tunnel) bool {
		if name == t.Config().ServiceName && namespace == t.Config().ServiceNamespace {
			keys = append(keys, k)
		}
		return true
	})
	return keys
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
		val, ok := parseIngressClass(ingress)
		if !ok {
			glog.V(5).Infof("No ingress class defined for ingress %s/%s", ingress.Namespace, ingress.Name)
			return "", false
		}
		glog.V(5).Infof("Ingress %s/%s class=%s", ingress.Namespace, ingress.Name, val)
		if val != ingressClass {
			return "", false
		}
		return constructIngressKey(ingress), true
	}
}

func (c *TunnelController) configureServiceInformer() {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	indexer, informer := cache.NewIndexerInformer(

		&cache.ListWatch{
			ListFunc: func(lo meta_v1.ListOptions) (runtime.Object, error) {
				return c.client.CoreV1().Services(v1.NamespaceAll).List(lo)
			},
			WatchFunc: func(lo meta_v1.ListOptions) (watch.Interface, error) {
				return c.client.CoreV1().Services(v1.NamespaceAll).Watch(lo)
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
				if !ok || !c.isWatchedService(svc) {
					return
				}
				queue.Add("add:" + constructServiceKey(svc))
			},
			UpdateFunc: func(old, new interface{}) {
				svc, ok := new.(*v1.Service)
				if !ok || !c.isWatchedService(svc) {
					return
				}
				queue.Add("update:" + constructServiceKey(svc))
			},
			DeleteFunc: func(obj interface{}) {
				svc, ok := obj.(*v1.Service)
				if !ok || !c.isWatchedService(svc) {
					return
				}
				queue.Add("delete:" + constructServiceKey(svc))
			},
		},
		cache.Indexers{},
	)
	c.serviceInformer = informer
	c.serviceLister = lister_v1.NewServiceLister(indexer)
	c.serviceWorkqueue = queue
}

// is this service one of the ones we have a tunnel for?
func (c *TunnelController) isWatchedService(service *v1.Service) (ok bool) {
	svcMeta, err := meta.Accessor(service)
	if err != nil {
		return
	}

	name, ns := svcMeta.GetName(), svcMeta.GetNamespace()
	c.tunnels.Range(func(k string, t tunnel.Tunnel) bool {
		if name == t.Config().ServiceName && ns == t.Config().ServiceNamespace {
			glog.V(5).Infof("Watching service %s/%s", ns, name)
			// set outer return and trigger stop condition
			ok = true
			return false
		}
		return true
	})
	return
}

func (c *TunnelController) configureEndpointInformer() {

	indexer, informer := cache.NewIndexerInformer(

		&cache.ListWatch{
			ListFunc: func(lo meta_v1.ListOptions) (runtime.Object, error) {
				return c.client.CoreV1().Endpoints(v1.NamespaceAll).List(lo)
			},
			WatchFunc: func(lo meta_v1.ListOptions) (watch.Interface, error) {
				return c.client.CoreV1().Endpoints(v1.NamespaceAll).Watch(lo)
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
				if !ok || !c.isWatchedEndpoint(ep) {
					return
				}
				c.serviceWorkqueue.Add("update:" + constructEndpointKey(ep))
			},
			UpdateFunc: func(old, new interface{}) {
				ep, ok := new.(*v1.Endpoints)
				if !ok || !c.isWatchedEndpoint(ep) {
					return
				}
				c.serviceWorkqueue.Add("update:" + constructEndpointKey(ep))
			},
			DeleteFunc: func(obj interface{}) {
				ep, ok := obj.(*v1.Endpoints)
				if !ok || !c.isWatchedEndpoint(ep) {
					return
				}
				c.serviceWorkqueue.Add("update:" + constructEndpointKey(ep))
			},
		},
		cache.Indexers{},
	)
	c.endpointsInformer = informer
	c.endpointsLister = lister_v1.NewEndpointsLister(indexer)
}

// is this endpoint interesting?
func (c *TunnelController) isWatchedEndpoint(ep *v1.Endpoints) (ok bool) {
	epMeta, err := meta.Accessor(ep)
	if err != nil {
		return
	}

	name, ns := epMeta.GetName(), epMeta.GetNamespace()
	c.tunnels.Range(func(k string, t tunnel.Tunnel) bool {
		if name == t.Config().ServiceName {
			glog.V(5).Infof("Watching endpoint %s/%s", ns, name)
			// set outer return and trigger stop condition
			ok = true
			return false
		}
		return true
	})
	return
}

func (c *TunnelController) Run(stopCh <-chan struct{}) {
	defer util_runtime.HandleCrash()
	defer c.ingressWorkqueue.ShutDown()
	defer c.serviceWorkqueue.ShutDown()

	glog.Info("Starting ArgoController")

	go c.serviceInformer.Run(stopCh)
	go c.endpointsInformer.Run(stopCh)
	go c.ingressInformer.Run(stopCh)

	// Wait for all caches to be synced, before processing items from the queue is started
	if !cache.WaitForCacheSync(stopCh, c.ingressInformer.HasSynced) {
		glog.Error(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}
	if !cache.WaitForCacheSync(stopCh, c.serviceInformer.HasSynced) {
		glog.Error(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	go wait.Until(c.runIngressWorker, time.Second, stopCh)
	go wait.Until(c.runServiceWorker, time.Second, stopCh)

	<-stopCh
	glog.Info("Stopping ArgoController ")
	c.tearDown()
}

func (c *TunnelController) runIngressWorker() {
	for c.processNextIngress() {
	}
}

func (c *TunnelController) processNextIngress() bool {

	key, quit := c.ingressWorkqueue.Get()
	if quit {
		return false
	}
	defer c.ingressWorkqueue.Done(key)

	err := c.processIngress(key.(string))
	handleErr(err, key, c.ingressWorkqueue)
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

func (c *TunnelController) processIngress(queueKey string) error {

	op, namespace, ingressname, servicename := parseIngressKey(queueKey)

	switch op {

	case "add":

		ingress, err := c.ingressLister.Ingresses(namespace).Get(ingressname)
		if err != nil {
			all, _ := c.ingressLister.Ingresses(namespace).List(labels.Everything())
			glog.V(2).Infof("all ingresses in %s: %v", "*", all)
			return fmt.Errorf("failed to retrieve ingress by name %q: %v", ingressname, err)
		}
		key := constructIngressKey(ingress)

		tunnel, ok := c.tunnels.Load(key)
		if ok {
			glog.V(5).Infof("Tunnel \"%s\" (%s) already exists", key, tunnel.Config().ExternalHostname)
			return nil
		}

		return c.createTunnel(key, ingress)

	case "delete":
		key := constructIngressKeyFromStrings(namespace, ingressname, servicename)
		return c.removeTunnel(key)

	case "update":

		ingress, err := c.ingressLister.Ingresses(namespace).Get(ingressname)
		if err != nil {
			all, _ := c.ingressLister.Ingresses(namespace).List(labels.Everything())
			glog.V(2).Infof("all ingresses in %s: %v", "*", all)
			return fmt.Errorf("failed to retrieve ingress by name %q: %v", ingressname, err)
		}
		key := constructIngressKey(ingress)

		return c.updateTunnel(key, ingress)

	default:
		return fmt.Errorf("Unhandled operation \"%s\"", op)

	}
}

func (c *TunnelController) runServiceWorker() {
	for c.processNextService() {
	}
}

func (c *TunnelController) processNextService() bool {

	key, quit := c.serviceWorkqueue.Get()
	if quit {
		return false
	}
	defer c.serviceWorkqueue.Done(key)

	err := c.processService(key.(string))
	handleErr(err, key, c.serviceWorkqueue)
	return true
}

func (c *TunnelController) processService(queueKey string) error {

	_, namespace, serviceName := parseServiceKey(queueKey)

	keys := c.getTunnelsForService(namespace, serviceName)
	if len(keys) == 0 {
		// no tunnels or ingresses exist for this service
		return nil
	}

	var errorMessage string
	for _, key := range keys {
		err := c.evaluateTunnelStatus(key)
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
func (c *TunnelController) validateIngress(ingress *v1beta1.Ingress) error {
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
func (c *TunnelController) getServiceNameForIngress(ingress *v1beta1.Ingress) string {
	return ingress.Spec.Rules[0].HTTP.Paths[0].Backend.ServiceName
}

// assumes validation
func (c *TunnelController) getServicePortForIngress(ingress *v1beta1.Ingress) intstr.IntOrString {
	return ingress.Spec.Rules[0].HTTP.Paths[0].Backend.ServicePort
}

// assumes validation
func (c *TunnelController) getHostNameForIngress(ingress *v1beta1.Ingress) string {
	return ingress.Spec.Rules[0].Host
}

func (c *TunnelController) readSecret(hostname string) (*v1.Secret, error) {

	var certSecret *v1.Secret

	elements := strings.Split(hostname, ".")
	// decrementing is overkill, because the only valid choice is one level down.
	for i := range elements {
		domain := strings.Join(elements[i:], ".")
		certSecretList, err := c.client.CoreV1().Secrets(c.options.secretNamespace).List(
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

	glog.V(5).Infof("Secret not found for label %s, hostname %s, trying default name %s", SecretLabelDomain, hostname, c.options.secretName)
	certSecret, err := c.client.CoreV1().Secrets(c.options.secretNamespace).Get(c.options.secretName, meta_v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return certSecret, nil
}

// obtains the origin cert for a particular hostname
func (c *TunnelController) readOriginCert(hostname string) ([]byte, error) {
	// in the future, we will have multiple secrets
	// at present the mapping of hostname -> secretname is "*" -> "cloudflared-cert"

	// certSecret, err := c.client.CoreV1().Secrets(c.namespace).Get(SecretName, meta_v1.GetOptions{})
	certSecret, err := c.readSecret(hostname)
	if err != nil {
		return []byte{}, err
	}
	certFileName := "cert.pem"
	originCert := certSecret.Data[certFileName]
	if len(originCert) == 0 {
		return []byte{}, fmt.Errorf("Certificate data not found for host %s in secret %s/%s", hostname, c.options.secretName, certFileName)
	}
	return originCert, nil
}

// creates a tunnel and stores a reference to it by serviceName
func (c *TunnelController) createTunnel(key string, ingress *v1beta1.Ingress) error {
	err := c.validateIngress(ingress)
	if err != nil {
		return err
	}
	ingressName := ingress.GetName()
	serviceName := c.getServiceNameForIngress(ingress)
	// key := fmt.Sprintf("%s/%s", ingress.ObjectMeta.Namespace, serviceName)
	glog.V(5).Infof("creating tunnel for ingress %s, %s", ingressName, key)

	servicePort := c.getServicePortForIngress(ingress)
	hostName := c.getHostNameForIngress(ingress)
	lbPool, _ := parseIngressLoadBalancer(ingress)

	originCert, err := c.readOriginCert(hostName)
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
		Version:          c.options.version,
	}

	tunnel, err := tunnel.NewArgoTunnel(config, c.metricsConfig)
	if err != nil {
		return err
	}
	c.tunnels.Store(key, tunnel)
	glog.V(5).Infof("created tunnel for ingress %s,  %s", ingressName, key)

	return c.evaluateTunnelStatus(key)
}

// updates a tunnel and stores a reference to it by serviceName
func (c *TunnelController) updateTunnel(key string, ingress *v1beta1.Ingress) error {
	err := c.validateIngress(ingress)
	if err != nil {
		c.removeTunnel(key)
		return err
	}
	t, ok := c.tunnels.Load(key)
	if !ok {
		return c.createTunnel(key, ingress)
	}

	servicePort := c.getServicePortForIngress(ingress)
	serviceName := c.getServiceNameForIngress(ingress)
	hostName := c.getHostNameForIngress(ingress)
	lbPool, _ := parseIngressLoadBalancer(ingress)

	config := t.Config()
	if config.LBPool != lbPool || config.ExternalHostname != hostName || config.ServicePort != servicePort || config.ServiceName != serviceName {
		glog.V(2).Infof("Ingress parameters have changed, recreating tunnel for %s", key)
		c.removeTunnel(key)
		return c.createTunnel(key, ingress)
	}
	return nil
}

// starts or stops the tunnel depending on the existence of
// the associated service and endpoints
func (c *TunnelController) evaluateTunnelStatus(key string) error {
	t, ok := c.tunnels.Load(key)
	if !ok {
		return fmt.Errorf("Tunnel not found for key %s", key)
	}

	serviceName := t.Config().ServiceName
	namespace := t.Config().ServiceNamespace

	service, err := c.serviceLister.Services(namespace).Get(serviceName)
	if service == nil || err != nil {
		glog.V(2).Infof("Service %s not found for tunnel", key)
		return c.stopTunnel(t)
	}

	endpoints, err := c.endpointsLister.Endpoints(namespace).Get(serviceName)
	if err != nil || endpoints == nil {
		glog.V(2).Infof("Endpoints not found for tunnel %s", key)
		return c.stopTunnel(t)
	}
	readyEndpointCount := 0
	for _, subset := range endpoints.Subsets {
		readyEndpointCount = readyEndpointCount + len(subset.Addresses)
	}
	if readyEndpointCount == 0 {
		glog.V(2).Infof("Endpoints not ready for tunnel %s", key)
		return c.stopTunnel(t)
	}

	glog.V(5).Infof("Validation ok for running %s with %d endpoint(s)", key, readyEndpointCount)
	if !t.Active() {
		return c.startTunnel(t, service)
	}

	return nil
}

func (c *TunnelController) startTunnel(t tunnel.Tunnel, service *v1.Service) error {

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
	return c.setIngressEndpoint(t, t.Config().ExternalHostname)
}

func (c *TunnelController) setIngressEndpoint(t tunnel.Tunnel, hostname string) error {
	namespace := t.Config().ServiceNamespace
	ingressName := t.Config().IngressName
	ingressClient := c.client.ExtensionsV1beta1().Ingresses(namespace)
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

func (c *TunnelController) stopTunnel(t tunnel.Tunnel) error {
	if t.Active() {
		err := t.Stop()
		if err != nil {
			glog.V(2).Infof("Error stopping tunnel, %v", err)
			return err
		}
		return c.setIngressEndpoint(t, "")
	}
	return nil
}

func (c *TunnelController) removeTunnel(key string) error {
	glog.V(5).Infof("Removing tunnel %s", key)
	t, ok := c.tunnels.LoadAndDelete(key)
	if !ok {
		return fmt.Errorf("Tunnel not found for key %s", key)
	}
	// Issue: if stopping the tunnel errors, the reference to the object
	// will be lost; but the tunnel may not have been detached and cleaned.
	// (the issue is historic and being preserved)
	return c.stopTunnel(t)
}

func (c *TunnelController) tearDown() error {
	glog.V(5).Infof("Tearing down tunnels")
	var wg wait.Group
	c.tunnels.Filter(func(k string, t tunnel.Tunnel) bool {
		wg.Start(func() {
			// Issue: use of teardown vs stop is suspect.
			if err := t.TearDown(); err != nil {
				glog.V(2).Infof("Error halting tunnel, %v", err)
			}
		})
		return true
	})
	wg.Wait()
	return nil
}
