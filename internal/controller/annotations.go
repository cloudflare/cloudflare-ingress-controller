package controller

import (
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
)

const (
	annotationIngressClass        = "kubernetes.io/ingress.class"
	annotationIngressLoadBalancer = "argo.cloudflare.com/lb-pool"
)

func parseIngressClass(ing *v1beta1.Ingress) (val string, ok bool) {
	if ingMeta, err := meta.Accessor(ing); err == nil {
		val, ok = ingMeta.GetAnnotations()[annotationIngressClass]
	}
	return
}

func parseIngressLoadBalancer(ing *v1beta1.Ingress) (val string, ok bool) {
	if ingMeta, err := meta.Accessor(ing); err == nil {
		val, ok = ingMeta.GetAnnotations()[annotationIngressLoadBalancer]
	}
	return
}
