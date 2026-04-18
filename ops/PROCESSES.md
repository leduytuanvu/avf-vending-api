# Processes and capabilities

This repo ships four main Go binaries. Each is a separate process; together they form a modular monolith style deployment (not microservices).

| Process | Entry | Required for | Hard dependencies | Optional / feature flags |
| ------- | ----- | -------------- | ------------------- | ------------------------- |
| **API** | `cmd/api` | HTTP `/v1/*`, health, optional gRPC | Postgres; Redis if your config uses it; JWT secrets per `HTTP_AUTH_MODE` | `METRICS_ENABLED`, `GRPC_ENABLED`, `API_ARTIFACTS_ENABLED` (+ S3 env), MQTT publish/NATS when wiring flags demand it; `TEMPORAL_*` when `TEMPORAL_ENABLED=true` (dials Temporal at startup; see `internal/app/workfloworch`) |
| **Worker** | `cmd/worker` | Outbox dispatch, payment timeout scan, stuck command scan | Postgres | `NATS_URL` for JetStream publish; `METRICS_ENABLED` + `WORKER_METRICS_LISTEN` for Prometheus; optional ClickHouse mirror via `ANALYTICS_*` (see `ops/ANALYTICS_CLICKHOUSE.md`) |
| **MQTT ingest** | `cmd/mqtt-ingest` | Device telemetry / shadow / command receipts over MQTT | Postgres + MQTT broker env (`internal/platform/mqtt`) | `METRICS_ENABLED` + `MQTT_INGEST_METRICS_LISTEN` for Prometheus |
| **Reconciler** | `cmd/reconciler` | Commerce list passes; optional PSP probe + refund enqueue | Postgres | `RECONCILER_ACTIONS_ENABLED` (+ probe URL, NATS, applier wiring); `METRICS_ENABLED` + `RECONCILER_METRICS_LISTEN` for Prometheus |

**Not in this table:** Loki-only log pipelines are optional (`deployments/docker` observability profile). **ClickHouse** is optional cold-path analytics only (`cmd/worker` when `ANALYTICS_CLICKHOUSE_ENABLED=true`); Postgres remains authoritative—see `internal/platform/clickhouse` and `ops/ANALYTICS_CLICKHOUSE.md`. **Temporal** is optional: default off; when enabled, only `cmd/api` bootstrap dials the frontend and exposes `Runtime.Deps.WorkflowOrchestration` (no dedicated Temporal worker binary in-repo yet; `Start` returns `ErrWorkflowNotImplemented` until workflows are registered). Compose `experimental` profile can still run a Temporal server for local use.

**Observability:** With `METRICS_ENABLED=true`, bind `/metrics` to an address reachable from your scraper (e.g. `0.0.0.0:9091` when Prometheus runs in Docker; see `deployments/docker/README.md`).
