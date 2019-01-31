# Origin Secret Configuration
Tunnel secret handling more closely follows the [Ingress][kubernetes-ingress] definition,
reusing the `TLS` section to map hosts to tunnel secrets. The revised handling restricts
tunnel secrets to be collocated with the [Ingress][kubernetes-ingress], a restriction not
present in previous versions.

To help with migration and to allow a set of reusable secrets to be located in an isolated
namepace, a configuration has been introduced to provide host specific default secrets.

The control `--origin-secret-config` sets the file path to origin specific secret
configuration.

### Format
The configuration is Yaml
```yaml
groups:
- hosts:
  - abc.test.com
  - cba.test.com
  secret:
    name: test-a
    namespace: test-a
- hosts:
  - xyz.test.com
  - zyx.test.com
  secret:
    name: test-b
    namespace: test-b
- hosts:
  - "*.test.com"
  secret:
    name: test-c
    namespace: test-c
```
> * wildcard hosts are allowed
> * specific hosts take precedence over wildcard hosts

[kubernetes-ingress]: https://kubernetes.io/docs/concepts/services-networking/ingress/
