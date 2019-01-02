# Your First Argo Tunnel
A guide to setting up your first argo-tunnel. If this is NOT your first argo-tunnel,
skip to step 3 or refer to other guides.

> `mydomain.com` is a place holder. Updated the value to match your Cloudflare domain.

### Requirements
- A [Cloudflare account][cloudflare-login]
- An [active zone on Cloudflare][cloudflare-quick-start-step-2]
- An active subscription to Argo, [enable here][cloudflare-dashboard-traffic]
- The [cloudflared daemon][argo-tunnel-daemon]

### Step 1: Enable Argo
If it’s your first time using Argo, navigate to the 
[Traffic tab of the Cloudflare dashboard][cloudflare-dashboard-traffic],
click the ‘Enable’ button, and follow the steps on the screen for setting up
usage-based billing.
- Browse to [Cloudflare: Traffic][cloudflare-dashboard-traffic].
- Locate `Argo` and click `Enable`.

> Enterprise customers who have enabled Argo will need to contact their Cloudflare
> representative to have Smart Routing enabled for their account as it is necessary
> for Argo Tunnel to work.

### Step 2: Install cloudflared
```cloudflared``` provides a mechanism to login, configure zones, and access
zone credentials.

[Follow these instructions to install cloudflared][argo-tunnel-daemon]

Once installed, verify cloudflared has installed properly by checking the version.
```console
cloudflared --version
```

### Step 3: Login to your Cloudflare account
```console
cloudflared login
```
> _If the browser fails to open automatically, copy and paste the URL into your 
browser’s address bar and press enter._

Once you login, you will see a list of domains associated with your account.
Locate the domain you wish to connect a tunnel to and click its name in the 
table row. Once you select the domain, Cloudflare will issue a certificate 
which will be downloaded automatically by your browser. This certificate will
be used to authenticate your machine to the Cloudflare edge.

Move the certificate to the ```.cloudflared``` directory on your system.
```console
mv cert.pem ~/.cloudflared/cert.pem
```

The certificate and domain will be used to define an Ingress to your system.

### Step 4: Deploy a Tunnel Secret
```console
kubectl create secret generic mydomain.com --from-file="$HOME/.cloudflared/cert.pem"
```
> Create the secret in the **same namespace** as your service deployment.
> Adjust `mydomain.com` to match your Cloudflare domain.

A single controller can configure tunnels for multiple domains. An [Ingress][kubernetes-ingress] definition will be used to defined tunnels to Services and link Secrets by external hostname.

### Step 5: Attach a Tunnel
When the controller observes the creation of an [Ingress][kubernetes-ingress], it verifies that
the referenced service, endpoints, and secret exists and opens a tunnel
between the Cloudflare receiver and the kubernetes virtual service ip.

```console
kubectl apply -f deploy/echo.yaml
```
> Adjust the Ingress host `echo.mydomain.com` to match your Cloudflare domain.
> Adjust the Ingress `tls` section to link the host with a secret.

**Caveats**:
- routing by path is not supported (`Ingress.spec.rules[*].host.http.paths[*].path`)

> This caveat will be addressed in future releases.

### Step 6: Verify the Tunnel
The tunnel will be visible under [DNS][cloudflare-dashboard-dns] on the Cloudflare dashboard.
- Browse to [Cloudflare: DNS][cloudflare-dashboard-dns].
- Browse to `echo.mydomain.com`.

> Adjust the Ingress host `echo.mydomain.com` to match your Cloudflare domain.


[argo-tunnel-daemon]: https://developers.cloudflare.com/argo-tunnel/downloads/
[cloudflare-dashboard-dns]: https://www.cloudflare.com/a/dns/
[cloudflare-dashboard-traffic]: https://www.cloudflare.com/a/traffic/
[cloudflare-login]: http://cloudflare.com/a/login
[cloudflare-quick-start-step-2]: https://support.cloudflare.com/hc/en-us/articles/201720164-Step-2-Create-a-Cloudflare-account-and-add-a-websit
[kubernetes-ingress]: https://kubernetes.io/docs/concepts/services-networking/ingress/
