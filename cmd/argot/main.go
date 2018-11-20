package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/cloudflare/cloudflare-ingress-controller/internal/argotunnel"
	"github.com/cloudflare/cloudflare-ingress-controller/internal/k8s"
	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/netutil"
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
	ingressclass := couple.Flag("ingress-class", "ingress class name").Default(argotunnel.IngressClassDefault).String()
	originsecret := k8s.ObjMixin(couple.Flag("default-origin-secret", "default origin certificate secret <namespace>/<name>"))
	debugaddr := couple.Flag("debug-address", "profiling bind address").Default("127.0.0.1:8081").String()
	debugenable := couple.Flag("debug-enable", "enable profiling handler").Bool()
	metricsaddr := couple.Flag("metrics-address", "metrics bind address").Default("0.0.0.0:8080").String()
	metricsenable := couple.Flag("metrics-enable", "enable metrics handler").Bool()
	connlimit := couple.Flag("connection-limit", "profiling bind address").Default("512").Int()
	resyncperiod := couple.Flag("resync-period", "period between synchronization attempts").Default(argotunnel.ResyncPeriodDefault.String()).Duration()
	workers := couple.Flag("workers", "number of workers processing updates").Default(strconv.Itoa(argotunnel.WorkersDefault)).Int()

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
		if *debugenable {
			debugServerMux := http.NewServeMux()
			debugServerMux.HandleFunc("/debug/pprof/", pprof.Index)
			debugServerMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
			debugServerMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
			debugServerMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
			debugServerMux.HandleFunc("/debug/pprof/trace", pprof.Trace)

			debugListener, err := net.Listen("tcp", *debugaddr)
			if err != nil {
				log.Fatalf("cannot open debug listener: %v", err)
				os.Exit(1)
			}

			debugListener = netutil.LimitListener(debugListener, *connlimit)
			debugServer := &http.Server{
				Handler:      debugServerMux,
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 5 * time.Second,
			}
			log.Debugf("debug listener on address: %s", *debugaddr)

			g.Add(func() error {
				return debugServer.Serve(debugListener)
			}, func(_ error) {
				debugServer.Shutdown(context.Background())
			})
		}
		if *metricsenable {
			// TODO: replace cloudflared metrics with go-kit metrics
			// cloudflared metrics currently assumes prometheus, uses the global registry
			// and does not differential by tunnel (e.g. assumes a daemon per tunnel)
			promregistry := prometheus.NewRegistry()
			promregistry.MustRegister(
				prometheus.NewProcessCollector(os.Getpid(), ""),
				prometheus.NewGoCollector(),
			)

			metricServerMux := http.NewServeMux()
			metricServerMux.Handle("/metrics", promhttp.HandlerFor(promregistry, promhttp.HandlerOpts{}))

			metricsListener, err := net.Listen("tcp", *metricsaddr)
			if err != nil {
				log.Fatalf("cannot open metrics listener: %v", err)
				os.Exit(1)
			}

			metricsListener = netutil.LimitListener(metricsListener, *connlimit)
			metricsServer := &http.Server{
				Handler:      metricServerMux,
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 5 * time.Second,
			}
			log.Debugf("metrics listener on address: %s", *metricsaddr)

			g.Add(func() error {
				return metricsServer.Serve(metricsListener)
			}, func(_ error) {
				metricsServer.Shutdown(context.Background())
			})
		}
		{
			kclient, err := kubeclient(*kubeconfig, *incluster)
			if err != nil {
				log.Fatalf("failed to create kubernetes client: %v", err)
				os.Exit(1)
			}

			argotunnel.EnableMetrics(5 * time.Second)
			argotunnel.SetVersion(version)

			ctx, cancel := context.WithCancel(context.Background())
			argo := argotunnel.NewController(kclient, log,
				argotunnel.IngressClass(*ingressclass),
				argotunnel.Secret(originsecret.Name, originsecret.Namespace),
				argotunnel.ResyncPeriod(*resyncperiod),
				argotunnel.Workers(*workers),
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
