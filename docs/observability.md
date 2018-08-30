# Argo Tunnel Ingress Observability 

### Analytics
- [Cloudflare Analytics][cloudflare-dashboard-analytics]

### Logs
Following is only allowed for a single pod.
```bash
POD_NAME=$(kubectl get pods -l "app=argo-tunnel" -o jsonpath="{.items[0].metadata.name}"); echo $POD_NAME
kubectl logs $POD_NAME --since=10m -f
```

All logs can be view by label,
```bash
kubectl logs -l "app=argo-tunnel" --since=10m
```

### Health Checks
Custom Health Checks can be defined under the [Traffic][cloudflare-dashboard-traffic] tab
on the Cloudflare dashboard.

- Navigate to [Traffic][cloudflare-dashboard-traffic].
- Create a health check monitor.
- Verify a status code of "2xx" and insert a Host header for the domain.

[cloudflare-dashboard-analytics]: https://www.cloudflare.com/a/analytics
[cloudflare-dashboard-traffic]: https://www.cloudflare.com/a/traffic/