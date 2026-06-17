# Local Docker observability stack

Grafana, Loki, and dashboard provisioning for **local** `deployments/docker/docker-compose.yml`.

## Intentional duplication with production

`deployments/prod/observability/` contains **parallel copies** of the same dashboard/datasource YAML and JSON (not symlinks). Reasons:

- Docker Compose on Windows and Ubuntu VPS expects stable, path-local files per profile.
- Local stack may use different datasource URLs than production nodes.

When updating dashboards, update **both** trees unless the change is environment-specific (document which tree in the commit message).
