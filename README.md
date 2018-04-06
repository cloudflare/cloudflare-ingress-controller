# Cloudflare Argo Tunnel Ingress Controller

### About

Cloudflare Argo Tunnel is an easy way to expose web servers securely to the
public internet, and routes traffic between the endpoints through an encrypted
tunnel and Cloudflare infrastructure. It is integrated with Cloudflare Argo
routing to provide efficient routing through Cloudflare's datacenters. The
`cloudflared` application is used to establish the tunnel wehn the web server is
running on a host directly.

The Argo Tunnel Ingress Controller uses kubernetes tools to provide routing for
services running in a cluster. The ingress controller is a native kubernetes
component and works the same way on any cluster: on a cloud provider, on bare
metal, or minikube.

Cloudflare Argo Links:

- [Argo Tunnel](https://developers.cloudflare.com/argo-tunnel/)
- [Argo Routing](https://www.cloudflare.com/products/argo-smart-routing/)
- [Ingress Controller](https://github.com/cloudflare/cloudflare-ingress-controller)

Argo Tunnel was previously known as Cloudflare Warp, and not all components have
been renamed.

### Installation and Configuration

In order to use the ingress controller you must have

- a cloudflare account with argo enabled, associated with a domain
- a kubernetes cluster
- a service in the cluster you want to expose

#### Configuring the Cloudflare account

Instructions to configure the account are found at
https://developers.cloudflare.com/argo-tunnel/quickstart/quickstart/

The `cloudflared` executable is required to obtain the argo token and
certificate. The token and certificate must be saved as a secret in the
kubernetes cluster.

To retrieve the certificate

- obtain the [cloudflared executable](https://developers.cloudflare.com/argo-tunnel/downloads/)
- run `cloudflared login`
- select the domain
- save the file locally (by default, to $HOME/.cloudflared/cert.pem)

#### Running a Kubernetes cluster

The easiest way to get started with Kubernetes and Argo Tunnel is with
[StackPointCloud](https://stackpoint.io), where you can run a kubernetes cluster
on any of several cloud providers, and install the Argo Tunnel immediately or as
an added solution.

Using the StackPointCloud interface, save the tunnel certificate for your
organization as Solution Credentials.  Then, when creating the cluster (or
adding as solution later), select the _Cloudflare_ solution, and the ingress
controller components will be added automatically.

#### Installing the controller components with helm

If you are using a different kubernetes cluster, [Helm](http://helm.sh) is the
simplest way to get started.  [Helm](http://helm.sh) is a package manager for
kubernetes which defines an application as a set of templates and make it easy
to install and update the application in a kubernetes cluster.

The Helm chart that describes all the components is found in controller [github
repository](https://github.com/cloudflare/cloudflare-ingress-controller) and is
also hosted at the StackPointCloud [trusted charts
repository](http://trusted-charts.stackpoint.io/)

To install the ingress controller with an downloaded certificate, you must
define a few variables, one of which is the base64-encoded contents of the
certificate

```bash
DOMAIN=mydomain.com
CERT_B64=$(base64 $HOME/.cloudflare-warp/cert.pem)
NS="warp"
USE_RBAC=true

RELEASE_NAME="warp-$DOMAIN"

helm install --name $RELEASE_NAME --namespace $NS \
   --set rbac.install=$USE_RBAC \
   --set secret.install=true,secret.domain=$DOMAIN,secret.certificate_b64=$CERT_B64 \
   tc/cloudflare-warp-ingress
```

Helm can install the ingress controller _without_ a certificate, in which case
you must follow the Helm chart instructions to inject the secret into the
cluster. The ingress controller will not be able to create connections without
the correct certificate.

### Creating an ingress

As a simple example of a service, let's use the https://httpbin.org service. It
is a python flask application, and there's a container available for it. To
install it into kubernetes, we use this manifest to install a deployment and a
service:

```yaml
apiVersion: v1
kind: List
metadata:
  resourceVersion: ""
  selfLink: ""
items:
- kind: Deployment
  apiVersion: apps/v1beta1
  metadata:
    name: httpbin
  spec:
    replicas: 1
    template:
      metadata:
        labels:
          app: warp-service-app
      spec:
        containers:
        - name: httpbin
          image: kennethreitz/httpbin:latest
          ports:
          - containerPort: 8080
- kind: Service
  apiVersion: v1
  metadata:
    name: httpbin
  spec:
    selector:
      app: warp-service-app
    ports:
    - protocol: TCP
      port: 80
      targetPort: 8080
```

The [kubernetes
ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/) is a
spec for external connectivity to a kubernetes service. Typically, the ingress
will contain an annotation, _kubernetes.io/ingress.class_, to hint at the type
of controller that should implement the connection.

Our ingress manifest contains

- the cloudflare-warp annotation
- a host url which belongs to the cloudflare domain we own
- the name of the service -- in the same namespace -- that should be exposed.

```yaml
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
name: httpbin
annotations:
    kubernetes.io/ingress.class: cloudflare-warp
spec:
rules:
- host: httpbin.anthopleura.net
    http:
    paths:
    - path: /
        backend:
        serviceName: httpbin
        servicePort: 80
```

When the controller observes the creation of an ingress, it verifies that

- the service exists
- the endpoints supporting the service (pods of the deployment exist)
- the cloudflare-warp-cert secret exists

and opens a tunnel between the cloudflare receiver and the kubernetes virtual
service ip.

### Monitoring

- Using the cloudflare UI

https://www.cloudflare.com/a/analytics/anthopleura.net

Navigate to the traffic tab, where the load-balancers, pools and origins are
listed. From here it is possible to manually create a health check monitor.
Generally, the health check should verify a status code of "2xx" and, using the
advanced options, insert a Host header of the desired domain.

- Examining Logs

The ingress controller pods can be listed by label with

```bash
kubectl get pod -l app=cloudflare-warp-ingress
kubectl logs -f [POD_NAME]
```
The ingress controller stdout log is verbose.

### High availability

- Spanning clusters

Creating ingresses in other clusters with matching hostnames will simply add
more origin tunnels to the cloudflare loadbalancer. From the cloudflare
perspective, each tunnel contributes equally to the pool and so traffic will be
routed across all the instances.

- Minikube

The ingress controller runs within minikube just as it does in a cloud-provider
cluster, and so allows easy routing of internet traffic into a development
environment.


### Technical details and roadmap

#### Kubernetes components

The full controller installation comprises the following kubernetes objects:

- _Deployment_:
    Manages one or more instances of the controller, each establishing independent tunnels.
- _Secret_:
    Contains the cloudflare and tls credentials to establish and manage the tunnels.
- _ClusterRole_:
    Defines the RBAC rights for the controller, to read secrets in its own namespace and watch pods, services and ingresses in other namespaces.
- _ServiceAccount_:
    Defines an identity for the controller.
- _ClusterRoleBinding_:
    Maps the serviceaccount identity to the role.

#### Roadmap

- Istio

Istio offers a useful set of tools for routing, managing and monitoring traffic
within the kubernetes cluster. By ci connecting argo tunnel traffic into the
istio mesh, we want to combine the internal traffic management tools with the
security of the argo tunnel. This engagement is ongoing.

- Prometheus metrics

Currently the _cloudflared_ application, when running standalone, will expose a
set of connection metrics for a single tunnel, making them available for
scraping by Prometheus. We intend to expand these single-tunnel metrics to the
multiple tunnels managed by an ingress controller.

- Deeper integration with Cloudflare

The cloudflare api exposes details of load-balancers, pools and origins, so
additional annotations on the ingress could signal additional actions to the
controller.

### Contributing

The following commands are a starting point for building the warp-controller code:

```
mkdir -p workspace/cloudflare-warp/src/github.com/cloudflare
export GOPATH=$(pwd)/workspace/cloudflare-warp/

cd workspace/cloudflare-warp/src/github.com/cloudflare
git clone https://github.com/cloudflare/cloudflare-warp-ingress/

cd cloudflare-warp-ingress/
dep ensure
make container
```

This process should retrieve all the necessary dependencies, build the binary,
and package it as a docker image.  Given that some of the github repositories
are private, there may or may not be issues retrieving the code. In order to run
the application in a kubernetes cluster, the image must be pushed to a
repository.  It is currently being pushed to a quay<span>.</span>io repository,
and this can be changed editing the references in the Makefile and in the
_deploy_ manifest.

### Engage with us

- Engage with us section
- Community forum link for CF
