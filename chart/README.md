# Argo Tunnel Ingress Controller

### TL;DR;
```console
$ helm install --name argo-mydomain chart/
```
> **Tip**: See [Your First Tunnel][guide-first-tunnel].

### About
Argo Tunnel Ingress Controller provides Kubernetes Ingress via Argo Tunnels.
The controller establishes or destroys tunnels by monitoring changes to resources.

Argo Tunnel offers an easy way to expose web servers securely to the internet,
without opening up firewall ports and configuring ACLs. Argo Tunnel also ensures
requests route through Cloudflare before reaching the web server so you can be 
sure attack traffic is stopped with Cloudflare’s WAF and Unmetered DDoS mitigation
and authenticated with Access if you’ve enabled those features for your account.

- [Argo Smart Routing][argo-smart-routing]
- [Argo Tunnel: Reference][argo-tunnel-reference]
- [Argo Tunnel: Quick Start][argo-tunnel-quick-start]
- [Argo Tunnel Ingress: Quick Start][argo-tunnel-ingress-quick-start]

### Installing the Chart
To install the chart with the release name `argo-mydomain`:
```console
$ helm install --name argo-mydomain chart/
```
> **Tip**: See [Your First Tunnel][guide-first-tunnel].

The command deploys the controller on the Kubernetes cluster in the default configuration.
The [configuration](#configuration) section lists the parameters that can be configured
during installation.

> **Tip**: List all releases using `helm list`

### Uninstalling the Chart
To uninstall/delete the `argo-mydomain` deployment:
```console
$ helm delete argo-mydomain
```

### Configuration
The following table lists the configurable parameters of the chart and their default values.

Parameter | Description | Default
--- | --- | ---
`controller.name` | name of the controller component | `controller`
`controller.image.repository` | controller container image repository | `gcr.io/stackpoint-public/argot`
`controller.image.tag` | controller container image tag | `0.5.2`
`controller.image.pullPolicy` | controller container image pull policy | `Always`
`controller.ingressClass` | name of the ingress class to route through this controller | `argo-tunnel`
`controller.logLevel` | log-level for this controller | `2`
`controller.replicaCount` | desired number of controller pods (load-balancers are required for values larger than 1). | `1`
`loadBalancing.enabled` | if `true`, replicaCount may be >1, requires load balancing enabled on account | `false`
`rbac.create` | if `true`, create & use RBAC resources | `true`
`serviceAccount.create` | if `true`, create a service account | `true`
`serviceAccount.name` | The name of the service account to use. If not set and `create` is `true`, a name is generated using the fullname template. | ``

A useful trick to debug issues with ingress is to increase the logLevel.
```console
$ helm install chart/ --set controller.logLevel=6
```

[argo-smart-routing]: https://www.cloudflare.com/products/argo-smart-routing/
[argo-tunnel-reference]: https://developers.cloudflare.com/argo-tunnel/reference/
[argo-tunnel-quick-start]: https://developers.cloudflare.com/argo-tunnel/quickstart/
[argo-tunnel-ingress-quick-start]: https://github.com/cloudflare/cloudflare-ingress-controller/
[guide-first-tunnel]: ../docs/guide_first_tunnel.md
