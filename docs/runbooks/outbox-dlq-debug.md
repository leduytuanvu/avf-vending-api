# Outbox and DLQ debugging (worker)

Durable **outbox** rows are published to JetStream (NATS) by `cmd/worker`. Failures increment DLQ-style counters and may leave rows unpublished until retried.

## Startup contract

- Local/dev may omit NATS only when `NATS_REQUIRED=false`, `OUTBOX_PUBLISHER_REQUIRED=false`, and `API_REQUIRE_NATS_RUNTIME=false`.
- Staging/production are fail-closed: `NATS_REQUIRED=true`, `OUTBOX_PUBLISHER_REQUIRED=true`, and `API_REQUIRE_NATS_RUNTIME=true` require a usable `NATS_URL` and JetStream internal streams at startup.
- Retry/DLQ knobs: `OUTBOX_MAX_ATTEMPTS`, `OUTBOX_BACKOFF_MIN`, `OUTBOX_BACKOFF_MAX`, and `OUTBOX_DLQ_ENABLED`.
- JetStream publishes require the outbox row contract: `aggregate_type`, `aggregate_id`, `event_type`, stable idempotency/event key, payload schema version, `created_at`, attempt metadata, and last-error/published timestamps when present.

## Where to look

- **Worker logs** — correlation via process logs; worker HTTP/gRPC may be minimal; focus on worker metrics and DB.
- **Postgres** — outbox tables (see migrations and `internal/app/background` / sqlc queries) for rows stuck in pending or dead-letter states.
- **NATS** — stream health, consumer lag, and publish errors.

## Prometheus (worker)

Registered in `internal/app/background/outboxmetrics` (namespace `avf`, subsystem `worker_outbox`), including:

- `avf_worker_outbox_pending_total`, `avf_worker_outbox_pending_due_now_total`, `avf_worker_outbox_oldest_pending_age_seconds`
- `avf_worker_outbox_failed_pending_total` — unpublished rows in `failed` status (retry scheduled; excludes terminal DLQ rows counted in `dead_lettered_total`).
- `avf_worker_outbox_publish_success_lag_seconds` (histogram, `created_at` → successful mark)
- `avf_worker_outbox_publish_jetstream_rpc_seconds` (histogram) — duration of one JetStream `Publish` RPC on the success path (narrower than lag; excludes queueing and Postgres work)
- `avf_worker_outbox_dispatch_publish_failed_total`, `avf_worker_outbox_dispatch_published_total`, `avf_worker_outbox_dispatch_dead_lettered_total`, `avf_worker_outbox_dispatch_dlq_publish_failed_total`

Exact metric names: scrape the **worker** `/metrics` listener (default `127.0.0.1:9091` when `METRICS_ENABLED` unless overridden).

## Operational playbook

1. **Spike in DLQ publish failures** — NATS connectivity, credentials, stream full, or message size; check broker dashboards and worker errors.
2. **`failed_pending_total` rises with elevated `publish_jetstream_rpc_seconds` p99** — broker slowness or oversized payloads; correlate with NATS dashboards. This counts **retryable** rows (`status='failed'`, still unpublished), not terminal Postgres DLQ rows (`dead_lettered_at` / `dead_letter` status), which show up in `dead_lettered_total` instead.
3. **Growing pending gauge** — worker not running, lease contention, or repeated publish failure; scale workers cautiously and fix root NATS issue.
4. **After incident** — replay or manual reconciliation only via documented admin/maintenance flows (do not bypass idempotency).

## Recovery steps

1. Confirm `cmd/worker` is running the same release as the API and has `NATS_URL` plus `OUTBOX_PUBLISHER_REQUIRED=true` in staging/production.
2. Confirm JetStream streams `AVF_INTERNAL_OUTBOX` and `AVF_INTERNAL_DLQ` exist and are writable by the worker credentials.
3. Inspect pending rows ordered by oldest `created_at` / next-attempt time; do not update payloads in place.
4. For poison messages, capture `event_type`, aggregate identifiers, attempts, and `last_error`, then decide whether to fix data and retry through an admin/ops flow or leave in DLQ for manual reconciliation.
5. **Break-glass CLI:** `cmd/outbox-replay` can list pending rows, **requeue** (clear lease/backoff), or **replay-dlq** with explicit confirmation — see [outbox-replay.md](./outbox-replay.md).
6. After NATS recovery, watch `avf_worker_outbox_pending_due_now_total`, `avf_worker_outbox_dispatch_published_total`, and DLQ counters until lag drains.

Local read-only smoke can include an optional NATS/ops read only if you expose a safe authenticated GET and set `SMOKE_AUTH_TOKEN` plus `SMOKE_AUTH_READ_PATH`. It must not publish or mutate outbox rows.

## Related

- [outbox-replay.md](./outbox-replay.md)
- [telemetry-jetstream-resilience.md](./telemetry-jetstream-resilience.md)
- [production-readiness.md](./production-readiness.md)

## Prometheus signals (canonical)

Worker forwards to **`outbox_*`** in `productionmetrics` (pending gauge, publish success/lag histograms, DLQ counters, dispatch failures). Names align with [`docs/observability/production-metrics.md`](../observability/production-metrics.md); legacy `avf_worker_outbox_*` may still appear during migration.
