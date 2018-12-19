# Argo Tunnel Ingress Controls

### Ingress Annotations
- `kubernetes.io/ingress.class`: the Ingress class that should interpret and serve the Ingress
  - defaults to `argo-tunnel`
  - override with the command-line option `--ingressClass=`
- `argo.cloudflare.com/compression-quality`: Use cross-stream compression instead HTTP compression.
  - defaults to `"0"`
  - quality:
    - 0 - off
    - 1 - low
    - 2 - medium
    - 3 - high
- `argo.cloudflare.com/ha-connections`: the number of high-availability connections to establish
  - defaults to `"4"`
- `argo.cloudflare.com/heartbeat-count`: minimum number of unacknowledged heartbeats to send before closing the connection
  - defaults to `"5"`
- `argo.cloudflare.com/heartbeat-interval`: minimum idle time before sending a heartbeat
  - defaults to `"5s"`
- `argo.cloudflare.com/lb-pool`: attach a Cloudflare loadbalancer for high-availability
  - load-balancing must be enabled for the Cloudflare account
  - allows balancing traffic across clusters
  - **required** if replicas > 1
- `argo.cloudflare.com/no-chunked-encoding`: disables chunked transfer encoding; useful if you are running a WSGI server
  - defaults to `"false"`
- `argo.cloudflare.com/retries`: maximum number of retries for connection/protocol errors.
  - defaults to `"3"`

### Command-Line Options
- `--default-origin-secret`: the default certificate used to establish tunnels
  - any tunnel that does not specify a secret will use this default.
- `--origin-secret-config`: the default certificate used for specific hosts
  - any matching host that does not specify a secret will use this default.
  - see [origin-secret-config][guide-origin-secret-config]

[guide-origin-secret-config]: ./guide_origin_secret_config.md