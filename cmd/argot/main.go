package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/cloudflare/cloudflare-ingress-controller/internal/controller"
	"github.com/golang/glog"
	"github.com/oklog/run"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var version = "UNKNOWN"

func main() {

	kubeconfig := flag.String("kubeconfig", "", "Path to a kubeconfig file")
	printVersion := flag.Bool("version", false, "prints application version")
	namespace := flag.String("namespace", controller.SecretNamespaceDefault, "Namespace to run in")
	ingressClass := flag.String("ingressClass", controller.IngressClassDefault, "Name of ingress class, used in ingress annotation")

	flag.Set("logtostderr", "true")
	flag.Parse()

	if *printVersion {
		name := filepath.Base(os.Args[0])
		fmt.Printf("%s %s\n", name, version)
		os.Exit(0)
	}

	kclient, err := kubeclient(*kubeconfig)
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
		argo := controller.NewArgoController(kclient,
			controller.IngressClass(*ingressClass),
			controller.SecretNamespace(*namespace),
			controller.Version(version),
		)
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

func kubeclient(kubeconfigpath string) (*kubernetes.Clientset, error) {
	kubeconfig, err := func() (*rest.Config, error) {
		if kubeconfigpath != "" {
			return clientcmd.BuildConfigFromFlags("", kubeconfigpath)
		}
		return rest.InClusterConfig()
	}()

	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(kubeconfig)
}
