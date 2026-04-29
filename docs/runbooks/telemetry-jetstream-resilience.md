# Telemetry JetStream resilience and worker projection

This runbook covers bounded JetStream streams/consumers, worker-side Postgres projection throttling, duplicate handling for metrics rollups, and how to interpret lag vs database pressure.

**Transactional outbox (`outbox_events`, `AVF_INTERNAL_OUTBOX`, `cmd/worker`) is a different pipeline** — it carries payment, commerce, inventory, command dispatch, and audit integration events written in the same database transaction as OLTP changes. Do not route those side effects through telemetry streams; use the outbox docs ([outbox.md](./outbox.md), [outbox-dlq-debug.md](./outbox-dlq-debug.md)).

For **fleet rollout**, overload playbooks, rollback, and bursty validation procedures, see [telemetry-production-rollout.md](./telemetry-production-rollout.md).

Prometheus alert names, SLO-style thresholds, and incident steps for lag / replay / projection failure live in [production-observability-alerts.md](./production-observability-alerts.md).

## JetStream capacity by fleet tier

Telemetry uses **six** JetStream streams (`AVF_TELEMETRY_*`). Each stream gets the **same** `MaxBytes` value from **`TELEMETRY_STREAM_MAX_BYTES`** (per-stream cap). Retention is **limits** with **`DiscardOld`**: when a stream hits its byte or age limit, the broker drops the **oldest** messages — so undersizing discards data before the worker can catch up after a replay storm.

**Disk planning:** worst-case footprint is on the order of **6 × TELEMETRY_STREAM_MAX_BYTES** (all streams near cap) plus metadata and filesystem overhead. Size the **`nats_data`** volume (data node) or JetStream store accordingly and keep **≥ ~30%** free space under normal load so bursts do not fill the disk.

| Approx. fleet | `TELEMETRY_STREAM_MAX_BYTES` (example) | Notes |
| --- | --- | --- |
| Pilot / up to ~100 machines | `536870912` (512 MiB) | Minimum **recommended** in production examples; values at the legacy 256 MiB default log a **startup warning** in `mqtt-ingest` / `worker`. |
| ~500 machines | `2147483648` (2 GiB) | Increase if storm tests or `nats stream info` show `Bytes` pinned near `MaxBytes` during replay drills. |
| ~1000 machines | `4294967296` (4 GiB) | Validate with [telemetry storm load test](./telemetry-production-rollout.md#bursty-load-validation-lab--maintenance-window) and monitor stream byte usage. |

`TELEMETRY_STREAM_MAX_AGE` (baseline, default `168h`) scales shorter-lived streams by fixed ratios; see `internal/platform/nats/telemetry_stream_plan.go`. **Do not** rely on age alone to protect bursts if `MaxBytes` is tiny — bytes cap binds first under load.

## Pull consumer tuning (explicit env vars)

These are loaded by **mqtt-ingest** (stream ensure) and **worker** (pull + projection). Defaults are conservative; tune only with metrics and evidence.

| Variable | Role | Safe starting value (examples) |
| --- | --- | --- |
| `TELEMETRY_CONSUMER_BATCH_SIZE` | Max messages per `Fetch` batch | `32` |
| `TELEMETRY_CONSUMER_MAX_ACK_PENDING` | Unacked messages allowed per durable | `1024` |
| `TELEMETRY_CONSUMER_ACK_WAIT` | Time before unacked message is redelivered | `30s` |
| `TELEMETRY_CONSUMER_MAX_DELIVER` | Max delivery attempts before giving up | `12` |
| `TELEMETRY_CONSUMER_PULL_TIMEOUT` | Long-poll wait for batch (`Fetch` max wait) | `2s` |

**`TELEMETRY_PROJECTION_MAX_CONCURRENCY`** is **not** a JetStream consumer setting but bounds concurrent Postgres projection work in the worker. **Do not increase** it to chase lag without confirming **Postgres pool headroom**, **CPU**, and **lock behavior** — raising concurrency on a saturated database increases contention and failure rates. Verify `postgres_pool_effective`, query latency, and `avf_telemetry_projection_flush_seconds` before changing.

Higher **`TELEMETRY_CONSUMER_MAX_ACK_PENDING`** increases in-flight work and memory on the broker and client; review upstream JetStream consumer documentation and validate changes with load tests.

## Startup logging

`mqtt-ingest` and `worker` emit structured logs after normalizing limits:

- `jetstream_telemetry_retention_effective` — effective `stream_max_bytes_per_stream`, `stream_max_age_baseline`, consumer fetch/ack settings.
- `jetstream_telemetry_stream_limits` — one line per stream (`max_bytes`, `max_age`, subjects).

In **`APP_ENV=production`**, if `stream_max_bytes_per_stream` is still at the **legacy 256 MiB** default, they also log **`jetstream_telemetry_stream_max_bytes_at_or_below_legacy_default`** (warning).

## Operational commands (stream bytes, lag, pending, redeliveries)

With the **`nats`** CLI (NATS.io) and server reachable:

```bash
# Stream byte usage and limits (repeat per AVF_TELEMETRY_* stream)
nats stream info AVF_TELEMETRY_METRICS

# Consumer backlog, pending ack, redelivery stats
nats consumer info AVF_TELEMETRY_METRICS avf-w-telemetry-metrics
```

Relevant **Stream** fields: `Bytes`, `Messages`, `Max Bytes`, `Max Age`, `Discard Old`. **Consumer** fields: `Unprocessed Messages` / pending, `Redelivered Messages`, `Ack Pending` (exact names vary by CLI version).

**Prometheus (worker):** `avf_telemetry_consumer_lag{stream,durable}` tracks broker-side **`NumPending`** (lag). **`avf_telemetry_projection_failures_total{reason}`** counts fetch/handler/Nak failures. **`avf_telemetry_projection_backlog`** tracks in-flight projection concurrency (semaphore). **`avf_telemetry_projection_flush_seconds`** observes end-to-end batch processing. Use them alongside stream `Bytes` to see whether the broker is retaining a large backlog versus the consumer falling behind on acks.

**NATS monitoring HTTP** (often `8222` on the data node): `http://127.0.0.1:8222/jsz` (JetStream summary) when monitoring is enabled on the server process.

Discarded messages: with **`DiscardOld`**, drops may appear as stream **deleted** sequence advancement without a DLQ entry; prefer preventing discard by sizing **`TELEMETRY_STREAM_MAX_BYTES`** and **`TELEMETRY_STREAM_MAX_AGE`** and by draining replay storms at the device (see production rollout runbook).

## Automated telemetry storm load test

The repo ships `deployments/prod/scripts/telemetry_storm_load_test.sh` to publish many valid `{prefix}/{machine_id}/telemetry` envelopes (deterministic `event_id`, `boot_id`, `seq_no`, `occurred_at`, `dedupe_key`) for offline-replay storm drills. Defaults: **dry-run** (`DRY_RUN=true`), **staging** (`LOAD_TEST_ENV=staging`). Production requires `CONFIRM_PROD_LOAD_TEST=true`.

The script prints copy-paste commands to scrape:

- mqtt-ingest: `avf_telemetry_ingest_*`, `avf_mqtt_ingest_dispatch_total`
- worker: `avf_telemetry_consumer_lag`, `avf_telemetry_projection_*`, `avf_telemetry_duplicate_total`, `avf_telemetry_idempotency_conflict_total`
- optional: `nats consumer report` when the NATS CLI is available
- Postgres session pressure via `psql` examples

After an execute run it can fail fast on rising non-droppable ingest drops, critical ingress rejects, container restarts for `mqtt-ingest` / `worker`, DB pool signatures in logs, OOM kills, or prolonged API `/health/ready` failure (when `API_READY_URL` / `WORKER_READY_URL` is set). **Duplicate business effects** are gated on **`avf_telemetry_idempotency_conflict_total`**: any increase after the run fails the gate (planned identical duplicate replays via `CRITICAL_DUPLICATE_REPLAY_PERCENT` must not increment this counter). Results are written to **`telemetry-storm-result.json`** (override with `RESULT_JSON_FILE`). See [telemetry-production-rollout.md](./telemetry-production-rollout.md) for exact commands.

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
3. If Postgres-bound: **do not** raise `TELEMETRY_PROJECTION_MAX_CONCURRENCY` blindly; verify pool saturation and fix underlying SQL or indexes. Lowering concurrency can reduce lock contention when the database is the bottleneck.
4. Avoid raising `TELEMETRY_CONSUMER_MAX_ACK_PENDING` without understanding memory: higher pending increases broker and client exposure to bursts.

## Environment reference

See `deployments/prod/app-node/.env.app-node.example` for the active split-topology env contract (`TELEMETRY_STREAM_*`, `TELEMETRY_CONSUMER_*`, `TELEMETRY_PROJECTION_*`, `TELEMETRY_READINESS_*`, `TELEMETRY_CONSUMER_LAG_POLL_INTERVAL`).
Use `deployments/prod/.env.production.example` only if you are intentionally reviewing the rollback-only legacy single-host path.

Stream max ages scale from `TELEMETRY_STREAM_MAX_AGE` (baseline = longest stream) using fixed ratios (heartbeat shortest, diagnostic longest).
