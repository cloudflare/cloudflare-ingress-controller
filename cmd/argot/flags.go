package main

import (
	"flag"
	"fmt"

	"github.com/cloudflare/cloudflare-ingress-controller/pkg/controller"
	"github.com/cloudflare/cloudflare-ingress-controller/pkg/version"
)

func parseFlags() (*controller.Config, bool, error) {

	exitEarly := false

	kubeconfig := flag.String("kubeconfig", "", "Path to a kubeconfig file")
	namespace := flag.String("namespace", "default", "Namespace to run in")
	ingressClass := flag.String("ingressClass", controller.CloudflareArgoIngressType, "Name of ingress class, used in ingress annotation")
	printVersion := flag.Bool("version", false, "prints application version")

	flag.Set("logtostderr", "true")
	flag.Parse()

	if *printVersion {
		fmt.Printf("%s %s\n", version.APP_NAME, version.VERSION)
		exitEarly = true
	}

	return &controller.Config{
		IngressClass:   *ingressClass,
		KubeconfigPath: *kubeconfig,
		Namespace:      *namespace,
		MaxRetries:     controller.MaxRetries,
	}, exitEarly, nil
}
