# High Availability with Argo Tunnels
A guide to setting up high availability with argo tunnels to span replicas,
Kubernetes clusters, or even cloud providers.

- Spanning Clusters

Creating ingresses in other clusters with matching domains
will add more origin tunnels to the Cloudflare load balancer, splitting the
traffic across all tunnels.

> The guide builds on [Setup Your First Tunnel][guide-first-tunnel].

### Requirements
- Load balancing enabled, [enable here][cloudflare-dashboard-traffic]

> A load-balancer is required to set the controller replica count > 1.

### Step 1: Create a Load Balancer
On the Cloudflare dashboard, browse to [Traffic][cloudflare-dashboard-traffic]
and create a load balancer named `echo-lb-pool`. Additional details can be found
at [Argo Tunnel: Load-Balancing][cloudflare-reference-load-balancing]
- Browse to [Argo Tunnel: Load-Balancing][cloudflare-reference-load-balancing].

### Step 2: Annotate the Ingress
Add the annotation `argo.cloudflare.com/lb-pool=echo-lb-pool` the `echo` Ingress.
```bash
kubectl annotate ing echo "argo.cloudflare.com/lb-pool=echo-lb-pool"
```


[cloudflare-dashboard-traffic]: https://www.cloudflare.com/a/traffic/
[cloudflare-reference-load-balancing]: https://developers.cloudflare.com/argo-tunnel/reference/load-balancing/
[guide-first-tunnel]: ./guide_first_tunnel.md
