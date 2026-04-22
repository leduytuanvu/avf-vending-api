# Production observability stack

This directory holds the reproducible production observability assets for the hardened 2-VPS deployment.

## Deployment model

Use the stack in two parts:

1. **Core** on one trusted node (commonly app node A or a small private observability VM):
   - `prometheus`
   - `loki`
   - `grafana`
   - `blackbox-exporter`

2. **Per-node agents** on every node you want host metrics or logs from:
   - `promtail`
   - `node-exporter`

The current repo-packaged compose entrypoint is an **app-node overlay**:

- layer `deployments/prod/docker-compose.observability.yml` with `deployments/prod/app-node/docker-compose.app-node.yml`
- `deployments/prod/docker-compose.observability.yml` also validates on its own for CI/config checks, but the intended runtime use is still as an app-node overlay

Validation example:

```bash
docker compose --env-file deployments/prod/app-node/.env.app-node --env-file deployments/prod/observability/.env.observability -f deployments/prod/app-node/docker-compose.app-node.yml -f deployments/prod/docker-compose.observability.yml config
```

Data-node coverage is currently provided by:

- Prometheus blackbox probes for NATS and EMQX
- optional `node-exporter` / `promtail` deployment using the same images and configs if you want host metrics and local logs from the data node

## Metrics sources

- app node A/B:
  - `cmd/api` on `:8081`
  - `cmd/worker` on `:9091`
  - `cmd/reconciler` on `:9092`
  - `cmd/mqtt-ingest` on `:9093`
  - `node-exporter` on `:9100`
- data node:
  - `node-exporter` on `:9100`
  - NATS health on `:8222/healthz`
  - EMQX status on `:18083/api/v5/status`
  - TCP reachability for `:4222` and `:8883`

## Logs

`promtail` tails Docker JSON logs locally on each node and pushes them to Loki.

Set these per-node values in `.env.observability`:

- `PROMTAIL_NODE_NAME`
- `PROMTAIL_NODE_ROLE`
- `LOKI_PUSH_URL`

## Targets and alerts

- scrape and probe inventory: `prometheus/prometheus.yml`
- alert rules: `prometheus/alerts.yml`
- Grafana provisioning: `grafana/provisioning/`

Replace the example private DNS names in `prometheus/prometheus.yml` with the real node names or private IPs used in production.

If you run `cmd/temporal-worker` in production, add an explicit scrape target and readiness probe for `:9094` in `prometheus/prometheus.yml`; the current sample does not do that by default.
