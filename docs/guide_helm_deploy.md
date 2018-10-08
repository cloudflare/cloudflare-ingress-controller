# Deploying with Helm
A guide to deploying your ingress controller with Helm, a Kubernetes package manager.

### Requirements
- [Helm][helm]

### Step 1: Setup the Repository
```console
helm repo add cloudflare https://cloudflare.github.io/helm-charts
helm repo update
```
> * [Cloudflare Helm Charts][helm-charts]

### Step 2: Deploy
```console
helm install --name anydomain cloudflare/argo-tunnel
```

A useful trick to debug issues with ingress is to increase the logLevel.
```console
helm install --name anydomain cloudflare/argo-tunnel --set controller.logLevel=6
```
> **Tip:** see the [argo-tunnel][helm-chart-argo-tunnel] helm chart for details

[helm]: https://helm.sh/
[helm-charts]: https://cloudflare.github.io/helm-charts/
[helm-chart-argo-tunnel]: https://github.com/cloudflare/helm-charts/tree/master/charts/argo-tunnel