package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/cloudflare/cloudflare-ingress-controller/internal/controller"
	"github.com/golang/glog"
	"github.com/oklog/run"
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

	var g run.Group
	{
		ctx, cancel := context.WithCancel(context.Background())
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

		g.Add(func() error {
			select {
			case s := <-sig:
				glog.Infof("Received signal=%s, exiting gracefully...\n", s.String())
				cancel()
			case <-ctx.Done():
			}
			return ctx.Err()
		}, func(_ error) {
			cancel()
		})
	}
	{
		ctx, cancel := context.WithCancel(context.Background())
		argo := controller.NewArgoController(kclient, config)
		argo.EnableMetrics()

		g.Add(func() error {
			argo.Run(ctx.Done())
			return nil
		}, func(error) {
			cancel()
		})
	}

	if err := g.Run(); err != nil {
		glog.Errorf("Received error, err=%v\n", err)
		os.Exit(1)
	}
}
