# Argo Tunnels for Subdomains

A guiding to setting up subdomain tunnels.

[Setup Your First Tunnel][guide-first-tunnel] generates and installs a certificate
matching `mydomain.com` and `*.mydomain.com`.  To configure tunnels for subdomains,
we'll need to generate additional certificates.

> The guide builds on [Setup Your First Tunnel][guide-first-tunnel].

> `subdomain.mydomain.com` is a place holder. Updated the value to match your Cloudflare domain.

### Step 1: Create a Subdomain Certificate
Certificates are located under [Crypto][cloudflare-dashboard-crypto] on the Cloudflare dashboard.
- Browse to [Crypto][cloudflare-dashboard-crypto].
- Click `Create Certificate`.
- Select the private key type `ECDSA`.
- Set domains `subdomain.mydomain.com` and `*.subdomain.mydomain.com`.
- Click `Next`.
- Save both the `Private Key` and `Certificate` to a file `cert.pem`.

> Save the entire contents with-in and including the section tags. 

### Step 2: Append the Tunnel Token
```bash
awk '/BEGIN.*TUNNEL/{mark=1}/END.*TUNNEL/{print;mark=0}mark' ~/.cloudflared/cert.pem >> cert.pem
```

### Step 3: Deploy the Tunnel Secret
```bash
kubectl create secret generic subdomain.mydomain.com --from-file="cert.pem"
```
> Create the secret in the same namespace as the service deployment.
> Adjust `subdomain.mydomain.com` to match your Cloudflare domain.

### Step 4: Attach a Tunnel
When the controller observes the creation of an ingress, it verifies that
the referenced service, endpoints, and secret exists and opens a tunnel
between the Cloudflare receiver and the kubernetes virtual service ip.

```bash
kubectl apply -f deploy/echo.yaml
```
> Adjust the Ingress host `echo.subdomain.mydomain.com` to match your Cloudflare domain.

### Step 5: Verify the Tunnel
The tunnel will be visible under [DNS][cloudflare-dashboard-dns] on the Cloudflare dashboard.
- Browse to [Cloudflare: DNS][cloudflare-dashboard-dns].
- Browse to `echo.subdomain.mydomain.com`.

> Adjust the Ingress host `echo.subdomain.mydomain.com` to match your Cloudflare domain.

[cloudflare-dashboard-crypto]: https://www.cloudflare.com/a/crypto/
[cloudflare-dashboard-dns]: https://www.cloudflare.com/a/dns/
[cloudflare-reference-subdomains]: https://developers.cloudflare.com/argo-tunnel/reference/tiered-subdomains/
[guide-first-tunnel]: ./guide_first_tunnel.md