# Cloudflare Argo Tunnel ingress controller Helm chart

## Cloudflare Argo Tunnel

The Cloudflare Argo Tunnel Ingress Controller makes connections between a Kubernetes
service and the Cloudflare edge, exposing an application in your cluster to the
internet at a hostname of your choice. A quick description of the details can be
found at https://developers.cloudflare.com/argo-tunnel/quickstart/ and
https://github.com/cloudflare/cloudflare-ingress-controller.

**Note:** Before installing Cloudflare Argo Tunnel you need to obtain Cloudflare
credentials for your domain zone. The credentials are obtained wtih use of the cloudflared
application, available from https://developers.cloudflare.com/argo-tunnel/downloads/


To deploy Cloudflare Argo Tunnel Ingress Controller:

```
### set these variables to match your situation
DOMAIN=mydomain.com
CERT_B64=$(base64 $HOME/.cloudflared/cert.pem)
NS="argo"
USE_RBAC=true
###

RELEASE_NAME=$( echo "argo-$DOMAIN" | sed -e 's/\./-/g')

helm install --name $RELEASE_NAME --namespace $NS \
   --set rbac.install=$USE_RBAC \
   --set secret.install=true,secret.domain=$DOMAIN,secret.certificate_b64=$CERT_B64 \
   chart/
```


Check that pods are running:

```bash
kubectl -n argo get pods
NAME                                                    READY     STATUS    RESTARTS   AGE
cloudflare-argo-ingress-cloudflare-argo-ingress-3061065498-v6mw5   1/1       Running   0          1m
```

## Testing external access

Now you should be able to check argo at https://argo.mydomain.com/
And if you noticed Cloudflare Argo Tunnel creates `https` connection by default :-)

## Remove

The release can be cleaned up with helm:

```bash
helm delete --purge  $RELEASE_NAME
```
