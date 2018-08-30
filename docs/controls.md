# Argo Tunnel Ingress Controls

### Ingress Annotations
- `kubernetes.io/ingress.class`: the Ingress class that should interpret and serve the Ingress.
  - defaults to `argo-tunnel`
  - override with the command-line option `--ingressClass=`
- `argo.cloudflare.com/lb-pool`: attach a Cloudflare loadbalancer for high-availability
  - load-balancing must be enabled for the Cloudflare account
  - allows balancing traffic across clusters

### Secret Labels
- `cloudflare-argo/domain`: the label is required to pair a secret with a domain
