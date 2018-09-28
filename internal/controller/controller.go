package controller

import (
	"fmt"
	"strings"
	"time"

	"github.com/cloudflare/cloudflare-ingress-controller/internal/tunnel"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
	log     *logrus.Logger

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

func NewTunnelController(client kubernetes.Interface, log *logrus.Logger, options ...Option) *TunnelController {
	opts := collectOptions(options)
	informer, indexer, queue := createIngressInformer(client, log, opts.ingressClass)

	c := &TunnelController{
		client:  client,
		options: opts,
		log:     log,

		ingressInformer:  informer,
		ingressWorkqueue: queue,
		ingressLister:    lister_v1beta1.NewIngressLister(indexer),
	}
	c.configureServiceInformer()
	c.configureEndpointInformer()
	return c
}

func (c *TunnelController) getTunnelsForService(namespace, name string) []string {
	var keys []string
	c.tunnels.Range(func(k string, t tunnel.Tunnel) bool {
		if name == t.Route().ServiceName && namespace == t.Route().Namespace {
			keys = append(keys, k)
		}
		return true
	})
	return keys
}

func createIngressInformer(client kubernetes.Interface, log *logrus.Logger, ingressClass string) (cache.Controller, cache.Indexer, workqueue.RateLimitingInterface) {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	shouldHandleIngress := handleIngressFunction(ingressClass, log)

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
func handleIngressFunction(ingressClass string, log *logrus.Logger) func(obj interface{}) (string, bool) {
	return func(obj interface{}) (string, bool) {

		ingress, ok := obj.(*v1beta1.Ingress)
		if !ok {
			log.Debugf("object is not an ingress, don't handle")
			return "", false
		}
		val, ok := parseIngressClass(ingress)
		if !ok {
			log.Debugf("ingress '%s/%s' defines no ingress class", ingress.Namespace, ingress.Name)
			return "", false
		}

		if val != ingressClass {
			log.Debugf("ingress %s/%s class does not match '%s != %s'", ingress.Namespace, ingress.Name, ingressClass, val)
			return "", false
		}
		log.Infof("ingress %s/%s class matches '%s", ingress.Namespace, ingress.Name, val)
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

	// todo: namespace, name, and port all need to be considered
	name, ns := svcMeta.GetName(), svcMeta.GetNamespace()
	c.tunnels.Range(func(k string, t tunnel.Tunnel) bool {
		if name == t.Route().ServiceName && ns == t.Route().Namespace {
			c.log.Debugf("watching service %s/%s", ns, name)
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
		if name == t.Route().ServiceName {
			c.log.Debugf("watching endpoint %s/%s", ns, name)
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

	c.log.Infof("starting argo tunnel controller '%+v'", c.options)

	go c.serviceInformer.Run(stopCh)
	go c.endpointsInformer.Run(stopCh)
	go c.ingressInformer.Run(stopCh)

	// Wait for all caches to be synced, before processing items from the queue is started
	// todo: the endpoint informer also needs to sync
	if !cache.WaitForCacheSync(stopCh,
		c.ingressInformer.HasSynced,
		c.serviceInformer.HasSynced) {
		util_runtime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
		return
	}

	go wait.Until(c.runIngressWorker, time.Second, stopCh)
	go wait.Until(c.runServiceWorker, time.Second, stopCh)

	<-stopCh
	c.log.Infof("stopping argo tunnel controller")
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
	handleErr(key, err, c.ingressWorkqueue, c.log)
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
			c.log.Debugf("all ingresses in '%s' '%v'", "*", all)
			return fmt.Errorf("failed to retrieve ingress by name %q: %v", ingressname, err)
		}
		key := constructIngressKey(ingress)

		tunnel, ok := c.tunnels.Load(key)
		if ok {
			c.log.Infof("tunnel ('%s' -> '%s') already exists", key, tunnel.Route().ExternalHostname)
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
			c.log.Debugf("all ingresses in '%s' '%v'", "*", all)
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
	handleErr(key, err, c.serviceWorkqueue, c.log)
	return true
}

func (c *TunnelController) processService(queueKey string) error {

	_, namespace, serviceName := parseServiceKey(queueKey)

	keys := c.getTunnelsForService(namespace, serviceName)
	if len(keys) == 0 {
		// no tunnels or ingresses exist for this service
		return nil
	}

	var msg string
	for _, key := range keys {
		err := c.evaluateTunnelStatus(key)
		if err != nil {
			if msg == "" {
				msg = err.Error()
			} else {
				msg = msg + "; " + err.Error()
			}
		}
	}
	if msg != "" {
		return fmt.Errorf("at least one error occurred handling %s: %s", queueKey, msg)
	}
	return nil

}

func handleErr(key interface{}, err error, queue workqueue.RateLimitingInterface, log *logrus.Logger) {
	if err == nil {
		queue.Forget(key)
		return
	}

	// This controller retries twice if something goes wrong...
	if queue.NumRequeues(key) < 2 {
		log.Warnf("retry processing '%v', err '%v'", key, err)
		queue.AddRateLimited(key)
		return
	}

	queue.Forget(key)
	log.Errorf("dropping object '%q' out of the queue, err '%v'", key, err)
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
			c.log.Infof("secret '%s' found for label '%s=%s'", secret.ObjectMeta.Name, SecretLabelDomain, domain)
			return &secret, nil
		}
	}

	c.log.Warnf("secret not found for label '%s', hostname '%s', trying default name '%s'", SecretLabelDomain, hostname, c.options.secretName)
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

	ns := ingress.GetNamespace()
	ingName := ingress.GetName()
	svcName := c.getServiceNameForIngress(ingress)
	// key := fmt.Sprintf("%s/%s", ingress.ObjectMeta.Namespace, serviceName)
	c.log.Infof("creating tunnel for ingress '%s/%s' ('%s')", ns, ingName, key)

	svcPort := c.getServicePortForIngress(ingress)
	hostName := c.getHostNameForIngress(ingress)
	options := parseIngressTunnelOptions(ingress)

	originCert, err := c.readOriginCert(hostName)
	if err != nil {
		return err
	}

	route := tunnel.Route{
		ServiceName:      svcName,
		Namespace:        ns,
		ServicePort:      svcPort,
		IngressName:      ingName,
		ExternalHostname: hostName,
		OriginCert:       originCert,
		Version:          c.options.version,
	}

	tunnel, err := tunnel.NewArgoTunnel(route, c.log, options...)
	if err != nil {
		return err
	}
	c.tunnels.Store(key, tunnel)
	c.log.Infof("created tunnel for ingress '%s/%s' ('%s')", ns, ingName, key)
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

	r := t.Route()
	if r.ExternalHostname != hostName || r.ServicePort != servicePort || r.ServiceName != serviceName {
		c.log.Infof("ingress tunnel origin has changed, recreating tunnel '%s'", key)
		c.removeTunnel(key)
		return c.createTunnel(key, ingress)
	}

	opts := tunnel.CollectOptions(parseIngressTunnelOptions(ingress))
	if t.Options() != opts {
		c.log.Infof("ingress tunnel options have changed, recreating tunnel '%s'", key)
		c.removeTunnel(key)
		return c.createTunnel(key, ingress)
	}
	return nil
}

// starts or stops the tunnel depending on the existence of
// the associated service and endpoints
// todo: should the tunnel be removed from tunnels on status errors?
func (c *TunnelController) evaluateTunnelStatus(key string) error {
	t, ok := c.tunnels.Load(key)
	if !ok {
		return fmt.Errorf("tunnel not found for key '%s'", key)
	}

	serviceName := t.Route().ServiceName
	namespace := t.Route().Namespace

	service, err := c.serviceLister.Services(namespace).Get(serviceName)
	if service == nil || err != nil {
		c.log.Infof("service not found for tunnel '%s'", key)
		return c.stopTunnel(t)
	}

	endpoints, err := c.endpointsLister.Endpoints(namespace).Get(serviceName)
	if err != nil || endpoints == nil {
		c.log.Infof("endpoints not found for tunnel '%s'", key)
		return c.stopTunnel(t)
	}
	readyEndpointCount := 0
	for _, subset := range endpoints.Subsets {
		readyEndpointCount = readyEndpointCount + len(subset.Addresses)
	}
	if readyEndpointCount == 0 {
		c.log.Infof("endpoints not ready for tunnel '%s'", key)
		return c.stopTunnel(t)
	}

	c.log.Debugf("safe to start tunnel '%s' with '%d' endpoint(s)", key, readyEndpointCount)
	if !t.Active() {
		return c.startTunnel(t, service)
	}
	c.log.Infof("tunnel to '%s' is already started.", key)
	return nil
}

func (c *TunnelController) startTunnel(t tunnel.Tunnel, service *v1.Service) error {

	var port int32
	ingressServicePort := t.Route().ServicePort
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
	c.log.Infof("starting tunnel to origin '%s'", url)
	err := t.Start(url)

	if err != nil {
		return err
	}
	return c.setIngressEndpoint(t, t.Route().ExternalHostname)
}

func (c *TunnelController) setIngressEndpoint(t tunnel.Tunnel, hostname string) error {
	namespace := t.Route().Namespace
	ingressName := t.Route().IngressName
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
		c.log.Warnf("error updating ingress %s/%s, '%v'", namespace, ingressName, err)
	}
	return err
}

func (c *TunnelController) stopTunnel(t tunnel.Tunnel) error {
	if t.Active() {
		err := t.Stop()
		if err != nil {
			c.log.Warnf("error stopping tunnel to '%s', %v", t.Origin(), err)
			return err
		}
		return c.setIngressEndpoint(t, "")
	}
	return nil
}

func (c *TunnelController) removeTunnel(key string) error {
	c.log.Infof("removing tunnel '%s'", key)
	t, ok := c.tunnels.LoadAndDelete(key)
	if !ok {
		return fmt.Errorf("tunnel not found for key '%s'", key)
	}
	// Issue: if stopping the tunnel errors, the reference to the object
	// will be lost; but the tunnel may not have been detached and cleaned.
	// (the issue is historic and being preserved)
	return c.stopTunnel(t)
}

func (c *TunnelController) tearDown() error {
	c.log.Infof("tearing down all tunnels")
	var wg wait.Group
	c.tunnels.Filter(func(k string, t tunnel.Tunnel) bool {
		wg.Start(func() {
			// Issue: use of teardown vs stop is suspect.
			if err := t.TearDown(); err != nil {
				c.log.Warnf("error halting tunnel to '%s', '%v'", t.Origin(), err)
			}
		})
		return true
	})
	wg.Wait()
	return nil
}
