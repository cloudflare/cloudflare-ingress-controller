# cloudflare-warp-ingress

Implements an ingress controller using cloudflare-warp tunnel
to connect a cloudflare-managed url to a kubernetes service.

## Deployment

The warp controller will manage ingress tunnels in a single
namespace of the cluster.  Multiple controllers can exist
in different namespaces, with different credentials for
each namespace.

#### Cloudflare certificate

The _cert.pem_ file must be available to the controller as a secret,
and must be configured in the cluster before the controller can start.

```
kubectl --namespace=$NAMESPACE create secret generic \
   cloudflare-warp-cert \
   --from-file=${HOME}/.cloudflare-warp/cert.pem
```

#### Warp controller deployment

```
kubectl --namespace=$NAMESPACE create -f deploy/cloudflare-serviceaccount.yaml
kubectl --namespace=$NAMESPACE create -f deploy/warp-controller-deployment.yaml
```

#### RBAC configuration

If your cluster has rbac enabled, then the warp controller must be configured
with sufficient rights to observe ingresses, services and endpoints.

```
kubectl --namespace=$NAMESPACE create -f deploy/cloudflare-warp-role.yaml
kubectl --namespace=$NAMESPACE create -f deploy/cloudflare-warp-rolebinding.yaml
```

#### Ingress deployment

An example of an ingress is found at _deploy/nginx-ingress.yaml_

## Configuration

#### Cloudflare credentials

The cloudflare _cert.pem_ file is saved as a kubernetes secret and
mounted as a file into the pod that creates the tunnel.

#### Ingress configuration

The ingress must have the annotation
_kubernetes.io<span>/</span>ingress.class: cloudflare-warp_ in order to be managed
by the warp controller.

## Design

There is a one-to-one relationship between a cloudflare url, a warp
tunnel, and a kubernetes service.  The controller watches the creation,
update and deletion of ingresses, services and endpoints.  When an
ingress with matching annotation is created, a tunnel-management
object is created to match it. The life-cycle of this tunnel-management
object matches the life-cycle of the ingress.

When a service and at least one endpoint exist to match that ingress,
the warp tunnel is created to route traffic though to the kubernetes
service, using kubernetes service-load-balancing to distribute traffic to
the endpoints.

The controller manages ingresses and services only in its own namespace.
This restriction matches the normal kubernetes security boundary, along
with the assumption that a cloudflare account is associated
with a namespace.

There are two implementiations of the Tunnel interface.  The
TunnelPodManager manages a pod that runs the warp client code.  This
pod has the cloudflare credentials configmap mounted into it, and
arguments passed to the commandline. When the ingress and service
and endpoints are present, the pod is created.  When the service or
endpoints are absent, the pod is destroyed.

The second implementation is the WarpManager, which runs the tunnel
connection in-process as a goroutine.  The tunel connection lifecycle is
matches the lifecycle of the service and endpoints, starting and stopping
when the backend service and endpoints are available or unavailable.

## Developing

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

This process should retrieve all the necessary dependencies, build the binary, and
package it as a docker image.  Given that some of the github repositories are private,
there may or may not be issues retrieving the code. In order to run the application in
a kubernetes cluster, the image must be pushed to a repository.  It is currently
being pushed to a quay<span>.</span>io repository, and this can be changed editing the references in
the Makefile and in the _deploy_ manifest.

