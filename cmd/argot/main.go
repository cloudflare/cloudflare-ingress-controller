package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/cloudflare/cloudflare-ingress-controller/internal/controller"
	"github.com/cloudflare/cloudflare-ingress-controller/internal/tunnel"
	"github.com/cloudflare/cloudflare-ingress-controller/internal/version"
	"github.com/golang/glog"
	"github.com/oklog/run"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func init() {
	flag.Set("logtostderr", "true")

	// Log as JSON instead of the default text.
	log.SetFormatter(&log.JSONFormatter{})

	// Output to stdout instead of the default stderr
	log.SetOutput(os.Stdout)

	log.SetLevel(log.DebugLevel)
}

func main() {

	kubeconfig := flag.String("kubeconfig", "", "Path to a kubeconfig file")
	printVersion := flag.Bool("version", false, "prints application version")

	config := &controller.Config{
		MaxRetries: controller.MaxRetries,
	}
	flag.StringVar(&config.Namespace, "namespace", "default", "Namespace to run in")
	flag.StringVar(&config.IngressClass, "ingressClass", controller.CloudflareArgoIngressType, "Name of ingress class, used in ingress annotation")

	flag.Set("logtostderr", "true")
	flag.Parse()

	if *printVersion {
		fmt.Printf("%s %s\n", version.APP_NAME, version.VERSION)
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
		argo := controller.NewArgoController(kclient, config)
		argo.EnableMetrics()

		g.Add(func() error {
			argo.Run(ctx.Done())
			return nil
		}, func(error) {
			cancel()
		})
	}
	{
		stopCh := make(chan struct{})
		g.Add(func() error {
			return tunnel.ServeMetrics(9090, stopCh, log.StandardLogger())
		}, func(error) {
			close(stopCh)
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
