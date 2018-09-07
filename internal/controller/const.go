package controller

const (
	MaxRetries                = 5
	IngressClassKey           = "kubernetes.io/ingress.class"
	IngressAnnotationLBPool   = "argo.cloudflare.com/lb-pool"
	SecretLabelDomain         = "cloudflare-argo/domain"
	SecretName                = "cloudflared-cert"
	CloudflareArgoIngressType = "argo-tunnel"
)
