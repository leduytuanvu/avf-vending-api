# Telemetry JetStream resilience and worker projection

This runbook covers bounded JetStream streams/consumers, worker-side Postgres projection throttling, duplicate handling for metrics rollups, and how to interpret lag vs database pressure.

For **fleet rollout**, overload playbooks, rollback, and bursty validation procedures, see [telemetry-production-rollout.md](./telemetry-production-rollout.md).

## What to monitor (Prometheus)

When `METRICS_ENABLED=true`, scrape the worker listen address (`WORKER_METRICS_LISTEN`, default `127.0.0.1:9091`).

| Metric | Meaning |
| --- | --- |
| `avf_telemetry_consumer_lag{stream,durable}` | JetStream `NumPending` for each telemetry durable consumer (broker-side backlog). |
| `avf_telemetry_projection_backlog` | In-flight messages inside the worker (projection semaphore slots). |
| `avf_telemetry_projection_failures_total{reason}` | `fetch_err`, `malformed_json`, `handler_err` counts. |
| `avf_telemetry_projection_batch_size` | Pull `Fetch` batch sizes. |
| `avf_telemetry_projection_flush_seconds` | Time to process a full pull batch. |
| `avf_telemetry_duplicate_total{reason}` | Skipped duplicate applies for metrics (`stream_seq`, `idempotency_replay`, `idempotency_conflict`). |
| `avf_telemetry_idempotency_conflict_total` | Same `idempotency_key`, different payload hash (apply skipped). |
| `avf_telemetry_projection_db_fail_consecutive_max` | Max consecutive Nak/handler-failure streak across durables. |

## HTTP probes (worker)

On the metrics listener: `GET /health/live` always returns `200 ok`. `GET /health/ready` returns `503` when:

- `TELEMETRY_READINESS_MAX_PENDING > 0` and max consumer `NumPending` exceeds it, or
- `TELEMETRY_READINESS_MAX_PROJECTION_FAIL_STREAK > 0` and projection failure streak reaches it.

If NATS is not configured for the worker, `/health/ready` returns `200` (telemetry projection disabled).

## Thresholds that suggest overload

- **Rising `telemetry_consumer_lag` with flat projection backlog**: the broker is accumulating faster than the worker pulls, or fetch batch / timeout is too conservative. Consider increasing `TELEMETRY_CONSUMER_BATCH_SIZE` slightly or `TELEMETRY_CONSUMER_PULL_TIMEOUT`, or scaling out another worker instance (same durable set — coordinate carefully).
- **High `telemetry_projection_backlog` with moderate lag**: Postgres or handler work is slow; bounded concurrency is doing its job. Check Postgres CPU, locks, and `telemetry_projection_flush_seconds` p99.
- **Nonzero `telemetry_duplicate_total`**: expected under redelivery; spikes after broker restarts or consumer upgrades are normal. Persistent high duplicates with rising rollups may indicate mis-tuned dedupe at publish time.
- **`telemetry_projection_db_fail_consecutive_max` climbing**: failing handlers or DB timeouts; inspect worker logs for `telemetry_handler_error`.

## First actions when lag rises

1. Confirm whether lag is **broker** (`telemetry_consumer_lag`) or **processing** (`telemetry_projection_backlog`, slow `flush_seconds`).
2. Check NATS server disk / JetStream store and stream `TELEMETRY_STREAM_MAX_BYTES` / `TELEMETRY_STREAM_MAX_AGE` (discard-old policy).
3. If Postgres-bound: lower `TELEMETRY_PROJECTION_MAX_CONCURRENCY` only after verifying pool saturation (avoid piling on failing queries); fix underlying SQL or indexes out of band.
4. Avoid raising `TELEMETRY_CONSUMER_MAX_ACK_PENDING` without understanding memory: higher pending increases broker and client exposure to bursts.

## Environment reference

See `deployments/prod/app-node/.env.app-node.example` for the active split-topology env contract (`TELEMETRY_STREAM_*`, `TELEMETRY_CONSUMER_*`, `TELEMETRY_PROJECTION_*`, `TELEMETRY_READINESS_*`, `TELEMETRY_CONSUMER_LAG_POLL_INTERVAL`).
Use `deployments/prod/.env.production.example` only if you are intentionally reviewing the rollback-only legacy single-host path.

Stream max ages scale from `TELEMETRY_STREAM_MAX_AGE` (baseline = longest stream) using fixed ratios (heartbeat shortest, diagnostic longest).
