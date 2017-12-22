# cloudflare-warp-ingress

Implements a Kubernetes ingress controller using cloudflare-warp tunnel
to connect a cloudflare-managed URL to a Kubernetes service.


## Getting started

The Warp controller will manage ingress tunnels in a single
namespace of the cluster.  Multiple controllers can exist
in different namespaces, with different credentials for
each namespace.


#### Pre-requirements

To use this, you need:
- a [Cloudflare](https://www.cloudflare.com/) account (free tier is OK)
- a zone (domain name) in your Cloudflare account 
- a [Kubernetes](https://kubernetes.io/) cluster

The deployment instructions below use a few YAML files that
will be used with `kubectl` to create the appropriate resources
in Kubernetes. You might want to checkout that repository
to have them handy!


#### Get Warp credentials

The first step is to obtain the credentials that will
be used by the controller to authenticate with Cloudflare.

First, install the Warp client.

```
# On Linux
curl https://bin.equinox.io/c/2ovkwS9YHaP/warp-stable-linux-amd64.tgz \
     | tar -zxC /usr/local/bin

# On macOS
curl https://warp.cloudflare.com/dl/warp-stable-darwin-amd64.tgz \
    | tar -zxC /usr/local/bin
```

(See [here](https://warp.cloudflare.com/downloads/) for further
installation information and links.)

Then, use the client to log in.

```
cloudflare-warp login
```

This will open a browser page (or show you an URL to open in your
browser) to complete the login process. After this, the credentials
will be available in file `.cloudflare-warp/cert.pem`.


#### Push credentials to Kubernetes

The cloudflare _cert.pem_ file is saved as a Kubernetes secret and
will be mounted as a file into the pod that creates the tunnel.

```
kubectl create secret generic cloudflare-warp-cert \
   --from-file=${HOME}/.cloudflare-warp/cert.pem
```


#### Deploy the Warp controller

This will create a service account for the Warp controller, and
create a Kubernetes "deployment" resource for the controller
(just like `kubectl run` would).

```
kubectl create -f deploy/cloudflare-warp-serviceaccount.yaml
kubectl create -f deploy/warp-controller-deployment.yaml
```


#### RBAC configuration

If your cluster has RBAC enabled, then the Warp controller must be configured
with sufficient rights to observe ingresses, services and endpoints.

```
kubectl create -f deploy/cloudflare-warp-role.yaml
kubectl create -f deploy/cloudflare-warp-rolebinding.yaml
```


#### Create ingress

An example of an ingress resource can be found in `deploy/nginx-ingress.yaml`.

Edit it to change:
- the `host` to be used (this can be any name under the zone that you
  picked during the `cloudflare-warp login` process earlier)
- the `serviceName` that you want to expose (and the port number if
  it is different)

Then create the ingress resource.

```
kubectl create -f deploy/nginx-ingress.yaml
```

You should now be able to access that service using the specified URL.


#### Troubleshooting

If things don't work as expected, check the logs of the controller.

```
kubectl logs deploy/warp-controller
```


## Notes


#### Ingress configuration

The ingress must have the annotation
_kubernetes.io<span>/</span>ingress.class: cloudflare-warp_ in order to be managed
by the Warp controller.


#### Namespaces

Most ingress controllers are deployed to be globally available to the
Kubernetes cluster (e.g. in the `kube-system` namespace). The Warp
controller is a bit different. Since it holds the credentials for a
specific DNS zone, you may want to deploy different instances with
different credentials in different namespaces. The example above
will create the ingress in your default namespace.

If you want to deploy the controller to a different namespace, you
need to:
- point `kubectl` to the right namespace (using `--namespace`, or
  `set-context`, or whatever suits your fancfy)
- edit `deploy/warp-controller-deployment.yaml` to specify the
  namespace you want to use on the controller command line
  
The `command:` section should look like the one below:

```yaml
- command:
  - /warp-controller
  - -v=6
  - -namespace=blue
```


## Design

There is a one-to-one relationship between a cloudflare url, a Warp
tunnel, and a kubernetes service.  The controller watches the creation,
update and deletion of ingresses, services and endpoints.  When an
ingress with matching annotation is created, a tunnel-management
object is created to match it. The life-cycle of this tunnel-management
object matches the life-cycle of the ingress.

When a service and at least one endpoint exist to match that ingress,
the Warp tunnel is created to route traffic though to the kubernetes
service, using kubernetes service-load-balancing to distribute traffic to
the endpoints.

The controller manages ingresses and services only in its own namespace.
This restriction matches the normal kubernetes security boundary, along
with the assumption that a cloudflare account is associated
with a namespace.

There are two implementiations of the Tunnel interface.  The
TunnelPodManager manages a pod that runs the Warp client code.  This
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

