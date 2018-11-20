# Updating 0.5.3 to 0.6.0
A guide tracking changes between 0.5.3 and 0.6.0.

### Overview
Version 0.6.0 is largely a refactor of 0.5.3 to alter resource handling to better track transitions (adds, updates, deletes).

A tunnel requires all of the criteria to be met
- an Ingress definition
- a Secret that covers the domain defined by the Ingress host
- a Service with active Endpoints matching the Ingress port

### Features
- resource changes are reflected more reliably and quickly 
- Ingress definitions now support multiple `rules` and `paths`
  - routing by url path is still not supported
- default tunnel secret is defined by a command-line flag
  - `--default-origin-secret=namespace/name`
- upgraded go to version 1.10.5
- upgraded cloudflared to version 2018.11.0

### Migration
- allow list, watch, and get on secrets
  - updated RBAC has been included 
- add `tls` sections to Ingress definitions
  - secrets are no longer linked to domains by label
  - secrets are linked though the Ingress definition
- colocate Secrets with tied Ingress (same namespace)
  - secrets are no longer deployed into a single/shared namespace
- review command-line flags
  - all command-line options have changed
