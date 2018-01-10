package controller

import (
	"flag"
	"fmt"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/golang/glog"
	"github.com/stretchr/testify/assert"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	ktesting "k8s.io/client-go/testing"
)

func init() {
	flag.Set("alsologtostderr", fmt.Sprintf("%t", true))
	var logLevel string
	flag.StringVar(&logLevel, "logLevel", "6", "test")
	flag.Lookup("v").Value.Set(logLevel)

	flag.Parse()
	glog.V(2).Infof("initializing test")
}

func TestSimple(t *testing.T) {
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
	namespace := "test-namespace"
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
	getPodAction := ktesting.NewGetAction(podResource, "pods", namespace)
	podObject, err := fakeClient.Invokes(getPodAction, &v1.Pod{})

	assert.True(t, called)

	assert.Nil(t, err)
	assert.NotNil(t, podObject)
	pod := podObject.(*v1.Pod)
	assert.NotNil(t, pod)
	assert.Equal(t, "forreal", pod.ObjectMeta.Name)

}

func TestNewWarpController(t *testing.T) {
	namespace := "test-namespace"
	fakeClient := &fake.Clientset{}
	wc := NewWarpController(fakeClient, namespace)

	stopCh := make(chan struct{})
	defer close(stopCh)
	go wc.Run(stopCh)
	// wait for cache sync
	time.Sleep(time.Second)

	assert.Equal(t, wc.namespace, namespace)
	assert.NotNil(t, wc.tunnels)
	assert.Equal(t, 0, len(wc.tunnels))
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
			Annotations: map[string]string{ingressAnnotationLBPool: poolName},
		},
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				v1beta1.IngressRule{
					Host: "test.example.com",
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: []v1beta1.HTTPIngressPath{
								v1beta1.HTTPIngressPath{
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
	meta_v1.SetMetaDataAnnotation(&ingress.ObjectMeta, ingressClassKey, cloudflareWarpIngressType)

	service := v1.Service{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      ingressName,
			Namespace: namespace,
		},
	}

	return tunnelItems{
		Certificate: v1.Secret{
			ObjectMeta: meta_v1.ObjectMeta{
				Name:      "cloudflare-warp-cert",
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
	fakeClient := &fake.Clientset{}

	namespace := "test-namespace"
	items := getTunnelItems(namespace)

	wc := NewWarpController(fakeClient, namespace)

	// broken for now
	// assert.Equal(t, "fooservice", wc.getServiceNameForIngress(&items.Ingress))

	assert.Equal(t, "test.example.com", wc.getHostNameForIngress(&items.Ingress))
	assert.Equal(t, int32(80), wc.getServicePortForIngress(&items.Ingress).IntVal)

	// assert.Equal(t, "fooservice.test-namespace", wc.getLBPoolForIngress(&items.Ingress))
	assert.Equal(t, "test.example.com", wc.getLBPoolForIngress(&items.Ingress))
}

func TestTunnelInitialization(t *testing.T) {
	fakeClient := &fake.Clientset{}

	namespace := "test-namespace"
	items := getTunnelItems(namespace)

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

	wc := NewWarpController(fakeClient, namespace)
	// wc.EnableMetrics()cw

	stopCh := make(chan struct{})
	defer close(stopCh)
	go wc.Run(stopCh)

	// wait for cache sync
	time.Sleep(time.Second)

	assert.Equal(t, wc.namespace, namespace)
	assert.NotNil(t, wc.tunnels)
	assert.Equal(t, 1, len(wc.tunnels))

	fooTunnel := wc.tunnels["fooservice"]
	assert.NotNil(t, fooTunnel)
	assert.False(t, fooTunnel.Active())
	assert.Equal(t, "test.example.com", fooTunnel.Config().ExternalHostname)
	assert.Equal(t, "fooservice", fooTunnel.Config().ServiceName)
	assert.Equal(t, int32(80), fooTunnel.Config().ServicePort.IntVal)

}

func TestTunnelServiceInitialization(t *testing.T) {
	fakeClient := &fake.Clientset{}

	namespace := "test-namespace"
	items := getTunnelItems(namespace)

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

	wc := NewWarpController(fakeClient, namespace)
	// wc.EnableMetrics()cw

	stopCh := make(chan struct{})
	defer close(stopCh)
	go wc.Run(stopCh)

	// wait for cache sync
	time.Sleep(time.Second)

	// add the service now
	fakeClient.Fake.AddReactor("list", "services", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, &v1.ServiceList{
			Items: []v1.Service{items.Service},
		}, nil
	})
	//  does invoking a create action trigger the watch?
	serviceResource := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}
	getServiceAction := ktesting.NewCreateAction(serviceResource, namespace, &items.Service)
	_, _ = fakeClient.Invokes(getServiceAction, &v1.Service{})

	time.Sleep(5 * time.Second)

	assert.Equal(t, wc.namespace, namespace)
	assert.NotNil(t, wc.tunnels)
	assert.Equal(t, 1, len(wc.tunnels))

	fooTunnel := wc.tunnels["fooservice"]
	assert.NotNil(t, fooTunnel)
	assert.False(t, fooTunnel.Active())
	assert.Equal(t, "test.example.com", fooTunnel.Config().ExternalHostname)
	assert.Equal(t, "fooservice", fooTunnel.Config().ServiceName)
	assert.Equal(t, int32(80), fooTunnel.Config().ServicePort.IntVal)

}
