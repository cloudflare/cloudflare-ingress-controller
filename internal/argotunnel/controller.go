package argotunnel

import (
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
)

// Controller translates kubernetes events into tunnels.
type Controller struct {
	client  kubernetes.Interface
	log     *logrus.Logger
	options options
}

// NewController create a new controller
func NewController(client kubernetes.Interface, log *logrus.Logger, options ...Option) *Controller {
	o := collectOptions(options)
	return &Controller{
		client:  client,
		log:     log,
		options: o,
	}
}

// Run starts processing
func (c *Controller) Run(stopCh <-chan struct{}) (err error) {
	defer runtime.HandleCrash()

	q := queue("queue")
	defer q.ShutDown()

	eph := newEndpointEventHander(q)
	ingh := newIngressEventHander(q, c.options.ingressClass)
	sech := newSecretEventHander(q)
	svch := newServiceEventHander(q)

	i := informerset{
		endpoint: newEndpointInformer(c.client, c.options, eph),
		ingress:  newIngressInformer(c.client, c.options, ingh),
		secret:   newSecretInformer(c.client, c.options, sech),
		service:  newServiceInformer(c.client, c.options, svch),
	}

	t := newTranslator(i, c.log, c.options)

	w := worker{
		queue:      q,
		translator: t,
		log:        c.log,
		options:    c.options,
	}
	c.log.Infof("starting argo-tunnel ingress...")
	w.log.Debugf("argo-tunnel ingress options=%+v", c.options)
	err = w.run(stopCh)
	c.log.Infof("stopping argo-tunnel ingress...")
	return
}
