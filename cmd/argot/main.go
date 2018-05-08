package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudflare/cloudflare-ingress-controller/pkg/controller"
	"github.com/cloudflare/cloudflare-ingress-controller/pkg/version"
	"github.com/golang/glog"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {

	kubeconfig := flag.String("kubeconfig", "", "Path to a kubeconfig file")
	namespace := flag.String("namespace", "default", "Namespace to run in")
	v := flag.Bool("version", false, "prints application version")

	flag.Set("logtostderr", "true")
	flag.Parse()

	if *v {
		fmt.Printf("%s %s\n", version.APP_NAME, version.VERSION)
		os.Exit(0)
	}

	var client *kubernetes.Clientset
	var config *rest.Config
	var err error

	if *kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		glog.Fatalf("Failed to get config: %v", err)
	}

	client, err = kubernetes.NewForConfig(config)
	if err != nil {
		glog.Fatalf("Failed to create kubernetes client: %v", err)
	}

	argo := controller.NewArgoController(client, *namespace)
	argo.EnableMetrics()

	stopCh := make(chan struct{})
	// defer close(stopCh)

	// crude trap Ctrl^C for better cleanup in testing
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		close(stopCh)
		time.Sleep(10 * time.Second)
		os.Exit(1)
	}()

	glog.Info("Starting Controller")
	argo.Run(stopCh)
}
