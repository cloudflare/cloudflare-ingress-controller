package controller

import (
	"flag"
	"fmt"
	"testing"
	"time"

	"github.com/golang/glog"
	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

func init() {
	flag.Set("alsologtostderr", fmt.Sprintf("%t", true))
	var logLevel string
	flag.StringVar(&logLevel, "logLevel", "6", "test")
	flag.Lookup("v").Value.Set(logLevel)

	flag.Parse()
	glog.V(2).Infof("initializing test")
}

type serviceKeyCheck struct {
	queueKey, operation, namespace, name string
	service                              *v1.Service
}

func checkServiceKey(t *testing.T, check serviceKeyCheck) {
	parsedOp, parsedNS, parsedName := parseServiceKey(check.queueKey)
	assert.Equal(t, check.operation, parsedOp)
	assert.Equal(t, check.namespace, parsedNS)
	assert.Equal(t, check.name, parsedName)
	assert.Equal(t, check.queueKey, check.operation+":"+constructServiceKey(check.service))
}

func TestServiceKey(t *testing.T) {
	t.Parallel()
	checks := []serviceKeyCheck{
		{
			"add:default/nginx",
			"add",
			"default",
			"nginx",
			&v1.Service{
				ObjectMeta: meta_v1.ObjectMeta{
					Name:      "nginx",
					Namespace: "default",
				},
			},
		},
		{
			"delete:acme/nginx",
			"delete",
			"acme",
			"nginx",
			&v1.Service{
				ObjectMeta: meta_v1.ObjectMeta{
					Name:      "nginx",
					Namespace: "acme",
				},
			},
		},
		{
			"update:acme/nginx",
			"update",
			"acme",
			"nginx",
			&v1.Service{
				ObjectMeta: meta_v1.ObjectMeta{
					Name:      "nginx",
					Namespace: "acme",
				},
			},
		},
	}

	for _, c := range checks {
		checkServiceKey(t, c)
	}
}

type ingressKeyCheck struct {
	queueKey, operation, namespace, ingressName, serviceName string
	ingress                                                  *v1beta1.Ingress
}

func checkIngressKey(t *testing.T, check ingressKeyCheck) {
	parsedOp, parsedNS, parsedIngressName, parsedServiceName := parseIngressKey(check.queueKey)
	assert.Equal(t, check.operation, parsedOp)
	assert.Equal(t, check.namespace, parsedNS)
	assert.Equal(t, check.serviceName, parsedServiceName)
	assert.Equal(t, check.ingressName, parsedIngressName)
	assert.Equal(t, check.queueKey, check.operation+":"+constructIngressKey(check.ingress))
}

func TestIngressKey(t *testing.T) {
	t.Parallel()
	ingressSpecNginx := v1beta1.IngressSpec{
		Rules: []v1beta1.IngressRule{
			{
				IngressRuleValue: v1beta1.IngressRuleValue{
					HTTP: &v1beta1.HTTPIngressRuleValue{
						Paths: []v1beta1.HTTPIngressPath{
							{
								Backend: v1beta1.IngressBackend{
									ServiceName: "nginx",
								},
							},
						},
					},
				},
			},
		},
	}

	checks := []ingressKeyCheck{
		{
			"add:default/nginx-in/nginx",
			"add",
			"default",
			"nginx-in",
			"nginx",
			&v1beta1.Ingress{
				ObjectMeta: meta_v1.ObjectMeta{
					Name:      "nginx-in",
					Namespace: "default",
				},
				Spec: ingressSpecNginx,
			},
		},
		{
			"delete:acme/nginx-in/nginx",
			"delete",
			"acme",
			"nginx-in",
			"nginx",
			&v1beta1.Ingress{
				ObjectMeta: meta_v1.ObjectMeta{
					Name:      "nginx-in",
					Namespace: "acme",
				},
				Spec: ingressSpecNginx,
			},
		},
		{
			"update:acme/nginx-in/nginx",
			"update",
			"acme",
			"nginx-in",
			"nginx",
			&v1beta1.Ingress{
				ObjectMeta: meta_v1.ObjectMeta{
					Name:      "nginx-in",
					Namespace: "acme",
				},
				Spec: ingressSpecNginx,
			},
		},
	}

	for _, c := range checks {
		checkIngressKey(t, c)
	}
}

func TestSimple(t *testing.T) {
	t.Parallel()
	fakeClient := &fake.Clientset{}
	nothingPod := v1.Pod{}

	fakeClient.Fake.AddReactor("list", "pods", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, &v1.PodList{
			Items: []v1.Pod{nothingPod},
		}, nil
	})

	pods, err := fakeClient.CoreV1().Pods("default").List(meta_v1.ListOptions{})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(pods.Items))

	services, err := fakeClient.CoreV1().Services("default").List(meta_v1.ListOptions{})
	assert.Nil(t, err)
	assert.Equal(t, 0, len(services.Items))
}

func TestAction(t *testing.T) {
	t.Parallel()
	serviceNamespace := "acme"
	fakeClient := &fake.Clientset{}
	actualPod := v1.Pod{
		ObjectMeta: meta_v1.ObjectMeta{
			Name: "forreal",
		},
	}
	called := false

	fakeClient.Fake.AddWatchReactor("*", ktesting.DefaultWatchReactor(watch.NewFake(), nil))

	fakeClient.Fake.AddReactor("get", "pods", func(action ktesting.Action) (bool, runtime.Object, error) {
		called = true
		return true, &actualPod, nil
	})

	podResource := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	getPodAction := ktesting.NewGetAction(podResource, "pods", serviceNamespace)
	podObject, err := fakeClient.Invokes(getPodAction, &v1.Pod{})

	assert.True(t, called)

	assert.Nil(t, err)
	assert.NotNil(t, podObject)
	pod := podObject.(*v1.Pod)
	assert.NotNil(t, pod)
	assert.Equal(t, "forreal", pod.ObjectMeta.Name)

}

func TestNewArgoController(t *testing.T) {
	t.Parallel()
	controllerNamespace := "cloudflare" // "cloudflare"
	fakeClient := &fake.Clientset{}

	wc := NewArgoController(fakeClient,
		SecretNamespace("cloudflare"),
	)

	stopCh := make(chan struct{})
	defer close(stopCh)
	go wc.Run(stopCh)

	cache.WaitForCacheSync(stopCh,
		wc.ingressInformer.HasSynced,
		wc.serviceInformer.HasSynced,
	)

	assert.Equal(t, wc.options.secretNamespace, controllerNamespace)
	assert.Equal(t, 0, wc.tunnels.Len())
}

type tunnelItems struct {
	Certificate v1.Secret
	Ingress     v1beta1.Ingress
	Service     v1.Service
	Pods        []v1.Pod
}

func getTunnelItems(namespace string) tunnelItems {

	serviceName := "fooservice"
	servicePort := intstr.FromInt(80)
	ingressName := "foo"
	poolName := "testpool"

	ingress := v1beta1.Ingress{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:        ingressName,
			Namespace:   namespace,
			Annotations: map[string]string{annotationIngressLoadBalancer: poolName},
		},
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				{
					Host: "test.example.com",
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: []v1beta1.HTTPIngressPath{
								{
									Path: "/",
									Backend: v1beta1.IngressBackend{
										ServiceName: serviceName,
										ServicePort: servicePort,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	meta_v1.SetMetaDataAnnotation(&ingress.ObjectMeta, annotationIngressClass, IngressClassDefault)

	service := v1.Service{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
		},
	}

	return tunnelItems{
		Certificate: v1.Secret{
			ObjectMeta: meta_v1.ObjectMeta{
				Name:      "cloudflared-cert",
				Namespace: namespace,
			},
			Data: map[string][]byte{
				"cert.pem": []byte("not actually a certificate"),
			},
		},
		Ingress: ingress,
		Service: service,
		Pods:    nil,
	}
}

func TestControllerLookups(t *testing.T) {
	t.Parallel()
	fakeClient := &fake.Clientset{}

	serviceNamespace := "acme"
	items := getTunnelItems(serviceNamespace)

	wc := NewArgoController(fakeClient,
		SecretNamespace("cloudflare"),
	)

	// broken for now
	// assert.Equal(t, "fooservice", wc.getServiceNameForIngress(&items.Ingress))
	lbpool, _ := parseIngressLoadBalancer(&items.Ingress)
	assert.Equal(t, "test.example.com", wc.getHostNameForIngress(&items.Ingress))
	assert.Equal(t, int32(80), wc.getServicePortForIngress(&items.Ingress).IntVal)
	assert.Equal(t, "testpool", lbpool)
	// assert.Equal(t, "test.example.com", wc.getLBPoolForIngress(&items.Ingress))
}

func TestTunnelInitialization(t *testing.T) {
	t.Parallel()
	fakeClient := &fake.Clientset{}

	serviceNamespace := "acme"
	controllerNamespace := "cloudflare"
	items := getTunnelItems(serviceNamespace)

	fakeClient.Fake.AddReactor("list", "ingresses", func(action ktesting.Action) (bool, runtime.Object, error) {
		glog.Infof("list is called on ingresses")
		return true, &v1beta1.IngressList{
			Items: []v1beta1.Ingress{items.Ingress},
		}, nil
	})
	// service is not listed initially

	fakeClient.Fake.AddReactor("get", "ingresses", func(action ktesting.Action) (bool, runtime.Object, error) {
		glog.Infof("get is called on ingresses")
		return true, &items.Ingress, nil
	})

	fakeClient.Fake.AddReactor("get", "secrets", func(action ktesting.Action) (bool, runtime.Object, error) {
		glog.Infof("get is called on secrets")
		return true, &items.Certificate, nil
	})

	fakeClient.Fake.AddWatchReactor("ingresses", ktesting.DefaultWatchReactor(watch.NewFake(), nil))
	fakeClient.Fake.AddWatchReactor("ingresses", ktesting.DefaultWatchReactor(watch.NewFake(), nil))
	fakeClient.Fake.AddWatchReactor("ingresses", ktesting.DefaultWatchReactor(watch.NewFake(), nil))

	wc := NewArgoController(fakeClient,
		SecretNamespace("cloudflare"),
	)

	stopCh := make(chan struct{})
	defer close(stopCh)
	go wc.Run(stopCh)

	cache.WaitForCacheSync(stopCh,
		wc.ingressInformer.HasSynced,
		wc.serviceInformer.HasSynced,
	)

	wait.Poll(100*time.Millisecond, 10*time.Second, func() (done bool, err error) {
		done = wc.tunnels.Len() > 0
		return
	})

	assert.Equal(t, wc.options.secretNamespace, controllerNamespace)
	assert.Equal(t, 1, wc.tunnels.Len())

	key := constructIngressKey(&items.Ingress)
	fooTunnel, ok := wc.tunnels.Load(key)
	if !ok {
		t.Fatalf("failing, tunnel is nil for %s", key)
	}
	assert.False(t, fooTunnel.Active())
	assert.Equal(t, "test.example.com", fooTunnel.Config().ExternalHostname)
	assert.Equal(t, "fooservice", fooTunnel.Config().ServiceName)
	assert.Equal(t, int32(80), fooTunnel.Config().ServicePort.IntVal)

}

func TestTunnelServiceInitialization(t *testing.T) {
	t.Parallel()
	fakeClient := &fake.Clientset{}

	controllerNamespace := "cloudflare"
	serviceNamespace := "acme"

	items := getTunnelItems(serviceNamespace)

	fakeClient.Fake.AddReactor("list", "ingresses", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, &v1beta1.IngressList{
			Items: []v1beta1.Ingress{items.Ingress},
		}, nil
	})
	fakeClient.Fake.AddReactor("get", "ingresses", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, &items.Ingress, nil
	})
	fakeClient.Fake.AddReactor("get", "services", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, &items.Service, nil
	})

	fakeClient.Fake.AddReactor("get", "secrets", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, &items.Certificate, nil
	})

	fakeClient.Fake.AddWatchReactor("*", ktesting.DefaultWatchReactor(watch.NewFake(), nil))

	wc := NewArgoController(fakeClient,
		SecretNamespace("cloudflare"),
	)

	stopCh := make(chan struct{})
	defer close(stopCh)
	go wc.Run(stopCh)

	cache.WaitForCacheSync(stopCh,
		wc.ingressInformer.HasSynced,
		wc.serviceInformer.HasSynced,
	)

	// add the service now
	fakeClient.Fake.AddReactor("list", "services", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, &v1.ServiceList{
			Items: []v1.Service{items.Service},
		}, nil
	})
	//  does invoking a create action trigger the watch?
	serviceResource := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}
	getServiceAction := ktesting.NewCreateAction(serviceResource, serviceNamespace, &items.Service)
	_, _ = fakeClient.Invokes(getServiceAction, &v1.Service{})

	wait.Poll(100*time.Millisecond, 10*time.Second, func() (done bool, err error) {
		done = wc.tunnels.Len() > 0
		return
	})

	assert.Equal(t, wc.options.secretNamespace, controllerNamespace)
	assert.Equal(t, 1, wc.tunnels.Len())

	key := constructIngressKey(&items.Ingress)
	fooTunnel, ok := wc.tunnels.Load(key)
	if !ok {
		t.Fatalf("failing, tunnel is nil for %s", key)
	}
	assert.False(t, fooTunnel.Active())
	assert.Equal(t, "test.example.com", fooTunnel.Config().ExternalHostname)
	assert.Equal(t, "fooservice", fooTunnel.Config().ServiceName)
	assert.Equal(t, int32(80), fooTunnel.Config().ServicePort.IntVal)

}

func TestTunnelServicesTwoNS(t *testing.T) {
	t.Parallel()
	fakeClient := &fake.Clientset{}

	controllerNamespace := "cloudflare"

	items := []tunnelItems{getTunnelItems("target"), getTunnelItems("walmart")}

	fakeClient.Fake.AddReactor("list", "ingresses", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, &v1beta1.IngressList{
			Items: []v1beta1.Ingress{items[0].Ingress, items[1].Ingress},
		}, nil
	})
	fakeClient.Fake.AddReactor("get", "ingresses", func(action ktesting.Action) (bool, runtime.Object, error) {
		switch action.GetNamespace() {
		case items[0].Ingress.GetNamespace():
			return true, &items[0].Ingress, nil
		case items[1].Ingress.GetNamespace():
			return true, &items[1].Ingress, nil
		default:
			return true, nil, fmt.Errorf("bad test namespace %s", action.GetNamespace())
		}
	})
	fakeClient.Fake.AddReactor("get", "services", func(action ktesting.Action) (bool, runtime.Object, error) {
		switch action.GetNamespace() {
		case items[0].Ingress.GetNamespace():
			return true, &items[0].Service, nil
		case items[1].Ingress.GetNamespace():
			return true, &items[1].Service, nil
		default:
			return true, nil, fmt.Errorf("bad test namespace %s", action.GetNamespace())
		}
	})

	fakeClient.Fake.AddReactor("get", "secrets", func(action ktesting.Action) (bool, runtime.Object, error) {
		switch action.GetNamespace() {
		case controllerNamespace:
			return true, &items[0].Certificate, nil
		default:
			return true, nil, fmt.Errorf("bad test namespace %s", action.GetNamespace())
		}
	})

	fakeClient.Fake.AddWatchReactor("*", ktesting.DefaultWatchReactor(watch.NewFake(), nil))

	wc := NewArgoController(fakeClient,
		SecretNamespace("cloudflare"),
	)

	stopCh := make(chan struct{})
	defer close(stopCh)
	go wc.Run(stopCh)

	cache.WaitForCacheSync(stopCh,
		wc.ingressInformer.HasSynced,
		wc.serviceInformer.HasSynced,
	)

	// add the service now
	fakeClient.Fake.AddReactor("list", "services", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, &v1.ServiceList{
			Items: []v1.Service{items[0].Service},
		}, nil
	})
	//  does invoking a create action trigger the watch?
	serviceResource := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}
	getServiceAction := ktesting.NewCreateAction(serviceResource, items[0].Ingress.GetNamespace(), &items[0].Service)
	_, _ = fakeClient.Invokes(getServiceAction, &v1.Service{})

	wait.Poll(100*time.Millisecond, 10*time.Second, func() (done bool, err error) {
		done = wc.tunnels.Len() > 1
		return
	})

	assert.Equal(t, wc.options.secretNamespace, controllerNamespace)
	assert.Equal(t, 2, wc.tunnels.Len())

	for _, item := range items {
		key := constructIngressKey(&item.Ingress)
		tunnel, ok := wc.tunnels.Load(key)
		if !ok {
			t.Fatalf("failing, tunnel is nil for %s", key)
		}
		assert.False(t, tunnel.Active())
		assert.Equal(t, "test.example.com", tunnel.Config().ExternalHostname)
		assert.Equal(t, "fooservice", tunnel.Config().ServiceName)
		assert.Equal(t, int32(80), tunnel.Config().ServicePort.IntVal)
	}
}
