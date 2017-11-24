# cloudflare-warp-ingress

Implements an ingress controller using cloudflare-warp tunnel
to connect a cloudflare-managed url to a kubernetes service.

## Configuration

#### Cloudflare credentials

The cloudflare _cert.pem_ file is inserted wholely into a configmap and
mounted as a file into the pod that creates the tunnel.

#### Ingress configuration

The ingress must have the annotation
_kubernetes.io/ingress.class: cloudflare-warp_ in order to be managed
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


