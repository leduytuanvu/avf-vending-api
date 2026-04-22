# Metrics and signals

## Prometheus (`METRICS_ENABLED=true`)

Set `METRICS_ENABLED=true` for any process that should expose `/metrics` on the default Prometheus registry.

| Process | Listen address env | Default bind when env empty | Primary series |
| ------- | ------------------- | -----------------------------| -------------- |
| `cmd/api` | `HTTP_ADDR` (and optional `HTTP_OPS_ADDR`) | `:8080` | `avf_http_request_duration_seconds_*` (Chi routes + status), Go/process defaults via `promhttp` |
| `cmd/worker` | `WORKER_METRICS_LISTEN` | `127.0.0.1:9091` | `avf_worker_outbox_*` (pending gauges, publish counters, publish lag histogram) — see `internal/app/background/outboxmetrics`; optional ClickHouse mirror: `avf_analytics_mirror_*` — `internal/platform/clickhouse` |
| `cmd/reconciler` | `RECONCILER_METRICS_LISTEN` | `127.0.0.1:9092` | `avf_reconciler_*` (`reconciler_job` label names the five tickers; avoids clashing with scrape `job`) — `internal/observability/reconcilerprom` |
| `cmd/mqtt-ingest` | `MQTT_INGEST_METRICS_LISTEN` | `127.0.0.1:9093` | `avf_mqtt_ingest_dispatch_total{kind,result}` — `internal/observability/mqttingestprom` |
| `cmd/temporal-worker` | `TEMPORAL_WORKER_METRICS_LISTEN` | `127.0.0.1:9094` | Go/process defaults via `promhttp`; workflow health is primarily log/runbook-driven today |

**Docker tip:** Prometheus in `deployments/docker` (`--profile observability`) scrapes `host.docker.internal`. Bind metrics listeners with `0.0.0.0:<port>` so the scraper can reach them (see `deployments/docker/README.md`).

## Health and readiness

- `cmd/api` serves `/health/live` and `/health/ready` on the main HTTP listener.
- `cmd/api` also serves `/version` on the main HTTP listener. When `HTTP_OPS_ADDR` is set, the same `/health/live`, `/health/ready`, `/version`, and optional `/metrics` surface is mirrored there for private probes and scrapes.
- `cmd/worker`, `cmd/reconciler`, `cmd/mqtt-ingest`, and `cmd/temporal-worker` always serve `/health/live` and `/health/ready` on their `*_METRICS_LISTEN` address, even when `METRICS_ENABLED=false`.
- `cmd/worker`, `cmd/reconciler`, `cmd/mqtt-ingest`, and `cmd/temporal-worker` also serve `/version` on the same ops listener.
- `/metrics` remains conditional on `METRICS_ENABLED=true`; health probes do not depend on Prometheus being enabled anymore.

`/version` returns operator-facing build/runtime metadata such as `version`, `git_sha`, `app_env`, `process`, `runtime_role`, `node_name`, `instance_id`, and configured public base URLs.

## Grafana

Provisioned dashboards (file provider) live under `ops/grafana/provisioning/dashboards/json/`:

- `avf-api-http.json` — API scrape health + HTTP latency histogram
- `avf-worker-outbox.json` — outbox backlog, publish rates, lag
- `avf-mqtt-ingest.json` — ingest dispatch throughput and errors by topic kind
- `avf-reconciler.json` — reconciler cycle duration, row volume, completion results
- `avf-platform-overview.json` in `deployments/prod/observability/grafana/provisioning/dashboards/json/` — 2-VPS production overview across app nodes and the fallback data node

Datasource UIDs: `prometheus`, `loki` (see `ops/grafana/provisioning/datasources/datasource.yml`).

## Logs (always)

Structured JSON logs (`zap`) remain the cross-cutting signal: `worker_job_summary`, `reconciler_job_summary`, `mqtt ingest failed`, `background_cycle_*`, etc.

For production 2-VPS:

- `promtail` in `deployments/prod/docker-compose.observability.yml` is packaged as an app-node observability overlay
- logs are labeled with per-node metadata (`PROMTAIL_NODE_NAME`, `PROMTAIL_NODE_ROLE`) and shipped to Loki
- Loki + Grafana provisioning is reproducible from `deployments/prod/observability/`
- data-node service health is covered by Prometheus blackbox probes by default; data-node host-log and host-metric agents remain an explicit operator deployment choice

## ClickHouse / Temporal / OTEL

- **ClickHouse:** optional cold-path only. When `ANALYTICS_CLICKHOUSE_ENABLED=true` and `ANALYTICS_MIRROR_OUTBOX_PUBLISHED=true`, `cmd/worker` exposes:
  - `avf_analytics_mirror_enqueue_dropped_total` — enqueue dropped under max concurrent pressure
  - `avf_analytics_mirror_insert_ok_total` / `avf_analytics_mirror_insert_failed_total` — per-row insert outcomes after retries
  - `avf_analytics_mirror_marshal_failed_total` — JSON marshal failures before insert
  See `ops/ANALYTICS_CLICKHOUSE.md`. Compose `experimental` profile runs ClickHouse for local trials.
- **Temporal:** optional `experimental` Compose service; `cmd/temporal-worker` executes registered workflows while `cmd/api`, `cmd/worker`, and `cmd/reconciler` may schedule follow-up work behind `TEMPORAL_SCHEDULE_*` flags.
- **OTEL:** all long-running binaries (`cmd/api`, `cmd/worker`, `cmd/reconciler`, `cmd/mqtt-ingest`, `cmd/temporal-worker`) initialize the OTEL tracer provider when telemetry is configured. HTTP tracing is mounted on the API path; background processes currently use OTEL mainly for process/resource identity and future span export.

Production observability in this phase keeps tracing lightweight:

- no tracing rewrite is introduced
- OTEL remains optional and can continue exporting to an external collector if you already use it
- the production observability stack focuses on Prometheus, Loki, and Grafana first

## Production 2-VPS scrape inventory

The production Prometheus config in `deployments/prod/observability/prometheus/prometheus.yml` is intended to scrape:

- app node A and B API ops metrics on `:8081`
- app node A and B worker metrics on `:9091`
- app node A reconciler metrics on `:9092`
- app node A mqtt-ingest metrics on `:9093`
- node-exporter on each app node and the data node
- public API HTTPS via blackbox probe
- NATS and EMQX fallback health/TCP probes on the data node

If you enable `cmd/temporal-worker` in production, add a dedicated `:9094` scrape target and readiness probe explicitly; the current sample does not include it by default.

## Sample Prometheus config

For local/lab Docker, see [`ops/prometheus/prometheus.yml`](prometheus/prometheus.yml).

For the 2-VPS production sample, see `deployments/prod/observability/prometheus/prometheus.yml`, which currently defines:

- `avf_api_metrics`
- `avf_worker_metrics`
- `avf_reconciler_metrics`
- `avf_mqtt_ingest_metrics`
- `avf_public_api_https`
- `avf_app_readiness`
- `avf_data_node_http`
- `avf_data_node_tcp`
