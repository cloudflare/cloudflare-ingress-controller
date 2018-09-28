package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/cloudflare/cloudflare-ingress-controller/internal/controller"
	"github.com/cloudflare/cloudflare-ingress-controller/internal/tunnel"
	"github.com/oklog/run"
	"github.com/sirupsen/logrus"
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

	log := logrus.StandardLogger()
	log.SetLevel(loglevel(flag.CommandLine))

	kclient, err := kubeclient(*kubeconfig)
	if err != nil {
		log.Fatalf("failed to create kubernetes client: %v", err)
		os.Exit(1)
	}

	var g run.Group
	{
		ctx, cancel := context.WithCancel(context.Background())
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

		g.Add(func() error {
			select {
			case s := <-sig:
				log.Infof("received signal=%s, exiting gracefully...\n", s.String())
				cancel()
			case <-ctx.Done():
			}
			return ctx.Err()
		}, func(_ error) {
			cancel()
		})
	}
	{
		tunnel.EnableMetrics(5 * time.Second)
		ctx, cancel := context.WithCancel(context.Background())
		argo := controller.NewTunnelController(kclient, log,
			controller.IngressClass(*ingressClass),
			controller.SecretNamespace(*namespace),
			controller.Version(version),
		)

		g.Add(func() error {
			argo.Run(ctx.Done())
			return nil
		}, func(error) {
			cancel()
		})
	}

	if err := g.Run(); err != nil {
		log.Fatalf("received fatal error, err=%v\n", err)
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

// Bridge the glog verbosity flag into a logrus.Level
func loglevel(flagset *flag.FlagSet) (l logrus.Level) {
	l = logrus.InfoLevel
	if f := flagset.Lookup("v"); f != nil {
		if v, err := strconv.Atoi(f.Value.String()); err == nil {
			if v >= 0 && v <= 5 {
				l = logrus.AllLevels[v]
			} else if v > 5 {
				l = logrus.DebugLevel
			} else {
				l = logrus.PanicLevel
			}
		}
	}
	return
}
