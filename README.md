# Argo Tunnel Ingress Controller

### About
Argo Tunnel Ingress Controller provides Kubernetes Ingress via Argo Tunnels.
The controller establishes or destroys tunnels by monitoring changes to resources.

Argo Tunnel offers an easy way to expose web servers securely to the internet,
without opening up firewall ports and configuring ACLs. Argo Tunnel also ensures
requests route through Cloudflare before reaching the web server so you can be 
sure attack traffic is stopped with Cloudflare’s WAF and Unmetered DDoS mitigation
and authenticated with Access if you’ve enabled those features for your account.

- [Argo Smart Routing][agro-smart-routing]
- [Argo Tunnel][argo-tunnel-quick-start]

### Deploy
```bash
kubectl apply -f deploy/argo-tunnel.yaml
```

### Guides & Reference
- [Argo Tunnel: Reference][argo-tunnel-reference]
- [Argo Tunnel: Quick Start][argo-tunnel-quick-start]
- [Setup Your First Tunnel][guide-first-tunnel]
- [Setup High Availability with Load Balancers][guide-ha-tunnel]
- [Setup Tunnels to Subdomains][guide-subdomain-tunnel]
- [Supported Command-Line Options, Annotations & Labels][controls]
- [Monitoring & Analytics][observability]

### Contributing
Thanks in advance for any and all contributions!

- Before contributing, please familiarize yourself with the [Code of Conduct][conduct]
- See [contributing][contributing] to setup your environment.
- Checkout the [issues][issues] and [roadmap][roadmap].

### Join the community
The [Cloudflare community forum][cloudflare-community] is a place to discuss
Argo, Argo Tunnel, or any Cloudflare product.

[agro-smart-routing]: https://www.cloudflare.com/products/argo-smart-routing/
[argo-tunnel-reference]: https://developers.cloudflare.com/argo-tunnel/reference/
[argo-tunnel-quick-start]: https://developers.cloudflare.com/argo-tunnel/quickstart/
[cloudflare-community]: https://community.cloudflare.com
[conduct]: ./CODE_OF_CONDUCT.md
[contributing]: /docs/contributing.md
[controls]: /docs/controls.md
[guide-first-tunnel]: /docs/guide_first_tunnel.md
[guide-ha-tunnel]: /docs/guide_ha_tunnel.md
[guide-subdomain-tunnel]: /docs/guide_subdomain_tunnel.md
[issues]: https://github.com/cloudflare/cloudflare-ingress-controller/issues
[observability]: /docs/observability.md
[releases]: https://github.com/cloudflare/cloudflare-ingress-controller/releases
[roadmap]: /docs/roadmap.md