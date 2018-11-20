package argotunnel

import (
	"strconv"
	"time"

	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	annotationIngressClass              = "kubernetes.io/ingress.class"
	annotationIngressCompressionQuality = "argo.cloudflare.com/compression-quality"
	annotationIngressHAConnections      = "argo.cloudflare.com/ha-connections"
	annotationIngressHeartbeatCount     = "argo.cloudflare.com/heartbeat-count"
	annotationIngressHeartbeatInterval  = "argo.cloudflare.com/heartbeat-interval"
	annotationIngressLoadBalancer       = "argo.cloudflare.com/lb-pool"
	annotationIngressNoChunkedEncoding  = "argo.cloudflare.com/no-chunked-encoding"
	annotationIngressRetries            = "argo.cloudflare.com/retries"
)

func parseIngressTunnelOptions(ing *v1beta1.Ingress) (opts []tunnelOption) {
	if ingMeta, err := meta.Accessor(ing); err == nil {
		if val, ok := parseMetaUint64(ingMeta, annotationIngressCompressionQuality); ok {
			opts = append(opts, compressionQuality(val))
		}
		if val, ok := parseMetaInt(ingMeta, annotationIngressHAConnections); ok {
			opts = append(opts, haConnections(val))
		}
		if val, ok := parseMetaUint64(ingMeta, annotationIngressHeartbeatCount); ok {
			opts = append(opts, heartbeatCount(val))
		}
		if val, ok := parseMetaDuration(ingMeta, annotationIngressHeartbeatInterval); ok {
			opts = append(opts, heartbeatInterval(val))
		}
		if val, ok := ingMeta.GetAnnotations()[annotationIngressLoadBalancer]; ok {
			opts = append(opts, lbPool(val))
		}
		if val, ok := parseMetaBool(ingMeta, annotationIngressNoChunkedEncoding); ok {
			opts = append(opts, disableChunkedEncoding(val))
		}
		if val, ok := parseMetaUint(ingMeta, annotationIngressRetries); ok {
			opts = append(opts, retries(val))
		}
	}
	return
}

func parseMetaBool(obj metav1.Object, key string) (val bool, ok bool) {
	if s, in := obj.GetAnnotations()[key]; in {
		switch s {
		case "true":
			val, ok = true, true
		case "false":
			val, ok = false, true
		}
	}
	return
}

func parseMetaDuration(obj metav1.Object, key string) (val time.Duration, ok bool) {
	if s, in := obj.GetAnnotations()[key]; in {
		if d, err := time.ParseDuration(s); err == nil {
			val, ok = d, true
		}
	}
	return
}

func parseMetaInt(obj metav1.Object, key string) (val int, ok bool) {
	if s, in := obj.GetAnnotations()[key]; in {
		if v, err := strconv.ParseInt(s, 10, 32); err == nil {
			val, ok = int(v), true
		}
	}
	return
}

func parseMetaUint(obj metav1.Object, key string) (val uint, ok bool) {
	if s, in := obj.GetAnnotations()[key]; in {
		if v, err := strconv.ParseUint(s, 10, 32); err == nil {
			val, ok = uint(v), true
		}
	}
	return
}

func parseMetaUint64(obj metav1.Object, key string) (val uint64, ok bool) {
	if s, in := obj.GetAnnotations()[key]; in {
		if v, err := strconv.ParseUint(s, 10, 64); err == nil {
			val, ok = uint64(v), true
		}
	}
	return
}

func parseIngressClass(ing *v1beta1.Ingress) (val string, ok bool) {
	if ingMeta, err := meta.Accessor(ing); err == nil {
		val, ok = ingMeta.GetAnnotations()[annotationIngressClass]
	}
	return
}
