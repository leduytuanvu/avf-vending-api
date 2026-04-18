# Local dependency stack (Docker Compose)

Compose is for **local development and lab** only. Services **without** a `profiles` entry always start:

- **PostgreSQL** — required by all Go processes for persistence.
- **Redis** — used when API/config enables Redis-backed features.
- **NATS (JetStream)** — broker for `cmd/worker` outbox publish and (when enabled) reconciler refund routing.

Optional **profiles** (combine as needed):

| Profile | Services | When to use |
| ------- | -------- | ------------ |
| *(default, none)* | `postgres`, `redis`, `nats` | Minimal backend for API + worker + reconciler on the host |
| `broker` | `emqx`, `minio` | `cmd/mqtt-ingest` against EMQX; MinIO for S3-style artifacts when `API_ARTIFACTS_ENABLED=true` |
| `observability` | `prometheus`, `loki`, `grafana`, `otel-collector` | Dashboards and sample scrapes (see `ops/prometheus/prometheus.yml`, `ops/grafana/provisioning/`) |
| `experimental` | `clickhouse`, `temporal`, `temporal-ui` | ClickHouse: optional analytics mirror in `cmd/worker` when `ANALYTICS_*` is set (see [ops/ANALYTICS_CLICKHOUSE.md](../../ops/ANALYTICS_CLICKHOUSE.md)). Temporal: optional service only; no worker in this repo yet. |

**Important:** Neither ClickHouse nor Temporal is required for core OLTP. ClickHouse stays off by default; Temporal has no client in `cmd/*` unless you add one.

## Usage

From the **repository root** (parent of `deployments/`):

```powershell
# Minimal core only
docker compose -f deployments/docker/docker-compose.yml up -d

# Core + EMQX/MinIO + observability stack
docker compose -f deployments/docker/docker-compose.yml --profile broker --profile observability up -d
```

## Prometheus scraping host processes

Prometheus (in Docker) scrapes `host.docker.internal` for Go metrics. For each process set `METRICS_ENABLED=true` and bind metrics on **all interfaces**, e.g.:

- `WORKER_METRICS_LISTEN=0.0.0.0:9091`
- `RECONCILER_METRICS_LISTEN=0.0.0.0:9092`
- `MQTT_INGEST_METRICS_LISTEN=0.0.0.0:9093`

The API serves `/metrics` on `HTTP_ADDR` (default `:8080`); ensure the host port is published if the API runs in another container.

## Other notes

- PostgreSQL init creates `avf_vending`, Temporal DBs, and the `temporal` role (see `postgres-init/01-init.sql`) even if you never start Temporal.
- Grafana defaults: `http://localhost:3000` as `admin` / `admin` when the `observability` profile is enabled.
- Temporal UI (`experimental`): `http://localhost:8233`.
- **`cmd/reconciler` with `RECONCILER_ACTIONS_ENABLED=true`:** set `RECONCILER_PAYMENT_PROBE_URL_TEMPLATE` with exactly one `%s` (payment id), `NATS_URL`, and `RECONCILER_REFUND_REVIEW_SUBJECT`. Use `RECONCILER_DRY_RUN=true` only together with actions enabled: probes still run, but payment rows are not updated and refund/duplicate NATS publishes are skipped.

See also: [ops/PROCESSES.md](../../ops/PROCESSES.md), [ops/METRICS.md](../../ops/METRICS.md), [ops/ANALYTICS_CLICKHOUSE.md](../../ops/ANALYTICS_CLICKHOUSE.md), [ops/RUNBOOK.md](../../ops/RUNBOOK.md).
