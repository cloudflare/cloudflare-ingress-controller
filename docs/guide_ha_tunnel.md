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

> A load-balancer is required when the controller replica count > 1.

### Step 1: Annotate the Ingress
Simply annotate the ingress with a load-balancer pool.  If the pool does not exist, 
it will be created for you.

```console
kubectl annotate ing echo "argo.cloudflare.com/lb-pool=echo-lb-pool"
```

The annotation can also be added directly to the Ingress definition, for example:
```yaml
apiVersion: v1
kind: Service
metadata:
  labels:
    app: echo
  name: echo
spec:
  ports:
  - name: http
    port: 80
    protocol: TCP
    targetPort: 8080
  selector:
    app: echo
---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  labels:
    app: echo
  name: echo
spec:
  replicas: 1
  selector:
    matchLabels:
      app: echo
  template:
    metadata:
      labels:
        app: echo
    spec:
      containers:
      - name: echo
        image: k8s.gcr.io/echoserver:1.10
        imagePullPolicy: Always
        ports:
        - containerPort: 8080
        resources:
          limits:
            cpu: 10m
            memory: 20Mi
          requests:
            cpu: 10m
            memory: 20Mi
      terminationGracePeriodSeconds: 60
---
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  annotations:
    kubernetes.io/ingress.class: argo-tunnel
    argo.cloudflare.com/lb-pool: echo-lb-pool
  labels:
    ingress: argo-tunnel
  name: echo
spec:
  rules:
  - host: echo.mydomain.com
    http:
      paths:
      - backend:
          serviceName: echo
          servicePort: http
```

[cloudflare-dashboard-traffic]: https://www.cloudflare.com/a/traffic/
[cloudflare-reference-load-balancing]: https://developers.cloudflare.com/argo-tunnel/reference/load-balancing/
[guide-first-tunnel]: ./guide_first_tunnel.md
