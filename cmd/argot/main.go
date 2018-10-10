package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/cloudflare/cloudflare-ingress-controller/internal/controller"
	"github.com/cloudflare/cloudflare-ingress-controller/internal/k8s"
	"github.com/cloudflare/cloudflare-ingress-controller/internal/tunnel"
	"github.com/oklog/run"
	"github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var version = "UNKNOWN"

func main() {
	name := filepath.Base(os.Args[0])
	app := kingpin.New(name, "Cloudflare Argo-Tunnel Kubernetes ingress controller.")
	verbose := app.Flag("v", "enable logging at specified level").Default("3").Int()

	// variant (print version information)
	variant := app.Command("version", "print version")

	// couple (build tunnels to services/endpoints)
	couple := app.Command("couple", "Couple services with argo tunnels")
	incluster := couple.Flag("incluster", "use in-cluster configuration.").Bool()
	kubeconfig := couple.Flag("kubeconfig", "path to kubeconfig (if not in running inside a cluster)").Default(filepath.Join(os.Getenv("HOME"), ".kube", "config")).String()
	ingressclass := couple.Flag("ingress-class", "ingress class name").Default(controller.IngressClassDefault).String()
	originsecret := k8s.ObjMixin(couple.Flag("default-origin-secret", "default origin certificate secret <namespace>/<name>").Default(controller.SecretNamespaceDefault + "/" + controller.SecretNameDefault))

	args := os.Args[1:]
	switch kingpin.MustParse(app.Parse(args)) {
	// variant (print version information)
	case variant.FullCommand():
		fmt.Printf("%s %s %s/%s\n", name, version, runtime.GOOS, runtime.GOARCH)

	// couple (build tunnels to services/endpoints)
	case couple.FullCommand():
		// mirror verbosity between glog and logrus
		flag.Set("logtostderr", "true")
		flag.Set("v", strconv.Itoa(*verbose))
		flag.Parse()

		log := logrus.StandardLogger()
		log.SetLevel(logruslevel(*verbose))
		log.Out = os.Stderr

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
			kclient, err := kubeclient(*kubeconfig, *incluster)
			if err != nil {
				log.Fatalf("failed to create kubernetes client: %v", err)
				os.Exit(1)
			}

			tunnel.EnableMetrics(5 * time.Second)
			ctx, cancel := context.WithCancel(context.Background())
			argo := controller.NewTunnelController(kclient, log,
				controller.IngressClass(*ingressclass),
				controller.SecretNamespace(originsecret.Namespace),
				controller.SecretName(originsecret.Name),
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
}

// select a kubernetes client
func kubeclient(kubeconfigpath string, incluster bool) (*kubernetes.Clientset, error) {
	kubeconfig, err := func() (*rest.Config, error) {
		if kubeconfigpath != "" && !incluster {
			return clientcmd.BuildConfigFromFlags("", kubeconfigpath)
		}
		return rest.InClusterConfig()
	}()

	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(kubeconfig)
}

// bridge verbose flag into a logrus.Level
func logruslevel(v int) (l logrus.Level) {
	if v >= 0 && v <= 5 {
		l = logrus.AllLevels[v]
	} else if v > 5 {
		l = logrus.DebugLevel
	} else {
		l = logrus.PanicLevel
	}
	return
}
