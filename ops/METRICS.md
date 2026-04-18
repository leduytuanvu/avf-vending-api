# Metrics and signals

## Prometheus (`METRICS_ENABLED=true`)

Set `METRICS_ENABLED=true` for any process that should expose `/metrics` on the default Prometheus registry.

| Process | Listen address env | Default bind when env empty | Primary series |
| ------- | ------------------- | -----------------------------| -------------- |
| `cmd/api` | `HTTP_ADDR` (same server as API) | `:8080` | `avf_http_request_duration_seconds_*` (Chi routes + status), Go/process defaults via `promhttp` |
| `cmd/worker` | `WORKER_METRICS_LISTEN` | `127.0.0.1:9091` | `avf_worker_outbox_*` (pending gauges, publish counters, publish lag histogram) — see `internal/app/background/outboxmetrics`; optional ClickHouse mirror: `avf_analytics_mirror_*` — `internal/platform/clickhouse` |
| `cmd/reconciler` | `RECONCILER_METRICS_LISTEN` | `127.0.0.1:9092` | `avf_reconciler_*` (`reconciler_job` label names the five tickers; avoids clashing with scrape `job`) — `internal/observability/reconcilerprom` |
| `cmd/mqtt-ingest` | `MQTT_INGEST_METRICS_LISTEN` | `127.0.0.1:9093` | `avf_mqtt_ingest_dispatch_total{kind,result}` — `internal/observability/mqttingestprom` |

**Docker tip:** Prometheus in `deployments/docker` (`--profile observability`) scrapes `host.docker.internal`. Bind metrics listeners with `0.0.0.0:<port>` so the scraper can reach them (see `deployments/docker/README.md`).

## Grafana

Provisioned dashboards (file provider) live under `ops/grafana/provisioning/dashboards/json/`:

- `avf-api-http.json` — API scrape health + HTTP latency histogram
- `avf-worker-outbox.json` — outbox backlog, publish rates, lag
- `avf-mqtt-ingest.json` — ingest dispatch throughput and errors by topic kind
- `avf-reconciler.json` — reconciler cycle duration, row volume, completion results

Datasource UIDs: `prometheus`, `loki` (see `ops/grafana/provisioning/datasources/datasource.yml`).

## Logs (always)

Structured JSON logs (`zap`) remain the cross-cutting signal: `worker_job_summary`, `reconciler_job_summary`, `mqtt ingest failed`, `background_cycle_*`, etc. Loki in Compose is optional; shipping app logs to Loki requires your own agent or driver (not bundled in this repo).

## ClickHouse / Temporal / OTEL

- **ClickHouse:** optional cold-path only. When `ANALYTICS_CLICKHOUSE_ENABLED=true` and `ANALYTICS_MIRROR_OUTBOX_PUBLISHED=true`, `cmd/worker` exposes:
  - `avf_analytics_mirror_enqueue_dropped_total` — enqueue dropped under max concurrent pressure
  - `avf_analytics_mirror_insert_ok_total` / `avf_analytics_mirror_insert_failed_total` — per-row insert outcomes after retries
  - `avf_analytics_mirror_marshal_failed_total` — JSON marshal failures before insert
  See `ops/ANALYTICS_CLICKHOUSE.md`. Compose `experimental` profile runs ClickHouse for local trials.
- **Temporal:** optional `experimental` Compose service; no Temporal worker/client in `cmd/*` here.
- **OTEL:** `cmd/api` can export traces/metrics to the sample collector (`ops/otel/otel-collector.yaml`); pipelines are debug + Prometheus self-scrape on `:8889` unless you extend the config.

## Sample Prometheus config

[`prometheus/prometheus.yml`](prometheus/prometheus.yml) lists jobs `avf_api`, `avf_worker`, `avf_reconciler`, `avf_mqtt_ingest`, and `otel_collector`. Adjust targets for your namespace or bind addresses.
