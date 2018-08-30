package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudflare/cloudflare-ingress-controller/pkg/controller"
	"github.com/golang/glog"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {

	config, exitNow, _ := parseFlags()

	if exitNow {
		os.Exit(0)
	}

	var kclient *kubernetes.Clientset
	var kconfig *rest.Config
	var err error

	if config.KubeconfigPath != "" {
		kconfig, err = clientcmd.BuildConfigFromFlags("", config.KubeconfigPath)
	} else {
		kconfig, err = rest.InClusterConfig()
	}
	if err != nil {
		glog.Fatalf("Failed to get config: %v", err)
	}

	kclient, err = kubernetes.NewForConfig(kconfig)
	if err != nil {
		glog.Fatalf("Failed to create kubernetes client: %v", err)
	}

	argo := controller.NewArgoController(kclient, config)
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
