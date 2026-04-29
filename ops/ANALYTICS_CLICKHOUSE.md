# Optional ClickHouse analytics mirror

This path is **cold / best-effort**: PostgreSQL and JetStream outbox semantics stay authoritative. ClickHouse is **disabled by default** (`ANALYTICS_CLICKHOUSE_ENABLED=false`).

## When to use

- Long-retention analytics, audit trails, or ad-hoc SQL over published outbox metadata.
- **Not** for enforcing idempotency, billing, or delivery guarantees.

## Enablement (`cmd/worker` only)

| Variable | Default | Meaning |
| -------- | ------- | ------- |
| `ANALYTICS_CLICKHOUSE_ENABLED` | `false` | If `true`, worker opens an HTTP client and **pings** ClickHouse at startup (fail fast if unreachable). |
| `ANALYTICS_CLICKHOUSE_HTTP_URL` | empty | e.g. `http://user:pass@host:8123/dbname` — scheme `http`/`https`, non-empty host, **path** = database name. |
| `ANALYTICS_MIRROR_OUTBOX_PUBLISHED` | `false` | If `true` (requires CH enabled), enqueue one JSONEachRow insert per outbox row **after** Postgres `MarkOutboxPublished` returns `marked=true`. |
| `ANALYTICS_PROJECT_OUTBOX_EVENTS` | `false` | If `true` (requires CH enabled), derive typed fleet/sales projection rows from successfully published outbox events. |
| `ANALYTICS_CLICKHOUSE_TABLE` | `avf_outbox_mirror` | Table name in that database; only `[A-Za-z0-9_]`. |
| `ANALYTICS_PROJECTION_TABLE` | `avf_analytics_projection_events` | Table name for typed projection rows; only `[A-Za-z0-9_]`. |
| `ANALYTICS_MIRROR_MAX_CONCURRENT` | `8` | Bounded async inserts; additional enqueue attempts increment `avf_analytics_mirror_enqueue_dropped_total` and log a warning. |
| `ANALYTICS_INSERT_TIMEOUT` | `5s` | Per HTTP insert attempt. |
| `ANALYTICS_INSERT_MAX_ATTEMPTS` | `3` | Retries per row; exhausted attempts increment `avf_analytics_mirror_insert_failed_total`. |

If `ANALYTICS_MIRROR_OUTBOX_PUBLISHED=true` or `ANALYTICS_PROJECT_OUTBOX_EVENTS=true` but ClickHouse is disabled, **config load fails** with an explicit error.

## Example DDL (analytics DB)

Column names match the JSON emitted by `internal/platform/clickhouse` (`mirror_row.go`):

```sql
CREATE TABLE IF NOT EXISTS avf_outbox_mirror
(
    outbox_id Int64,
    topic String,
    event_type String,
    aggregate_type String,
    aggregate_id UUID,
    organization_id String,
    payload_base64 String,
    published_at String,
    ingested_at String,
    idempotency_key String
)
ENGINE = MergeTree
ORDER BY (published_at, outbox_id);
```

`published_at` / `ingested_at` are RFC3339Nano strings for simplicity; you can add materialized columns to parse to `DateTime64` if needed.

## Typed projection DDL

`ANALYTICS_PROJECT_OUTBOX_EVENTS=true` writes one row per classified published outbox event. Classification is intentionally conservative and based on `event_type`, `aggregate_type`, and topic:

- `sales_event`
- `payment_event`
- `vend_event`
- `inventory_delta`
- `machine_telemetry_summary`
- `command_lifecycle_event`

```sql
CREATE TABLE IF NOT EXISTS avf_analytics_projection_events
(
    projection_id String,
    projection_type LowCardinality(String),
    source LowCardinality(String),
    outbox_id Int64,
    topic String,
    event_type String,
    aggregate_type String,
    aggregate_id UUID,
    organization_id String,
    occurred_at String,
    published_at String,
    ingested_at String,
    idempotency_key String,
    payload_base64 String
)
ENGINE = ReplacingMergeTree(ingested_at)
ORDER BY (projection_type, projection_id);
```

`projection_id` is deterministic from the outbox row (`outbox:<id>:<projection_type>`), so replay/backfill can safely re-insert and let `ReplacingMergeTree` collapse duplicates at query time.

## Failure behavior

- Publish + Postgres mark success **does not** depend on the mirror.
- Mirror/projection runs **only** inside the `marked == true` branch after a successful mark.
- Worker recovers panics from the analytics hook; analytics bugs must not fail outbox dispatch.
- Shutdown: worker calls mirror/projection sink `Shutdown()` before closing the HTTP client so in-flight inserts get a chance to finish (bounded by timeouts).

## Backfill design

Backfill should read already-published Postgres `outbox_events` in stable `id` order and feed the same projection mapper used by `cmd/worker`. Keep batches small, use `projection_id` for idempotency, and never mutate OLTP rows. A future CLI/backfill job should expose:

```powershell
go run ./cmd/worker -analytics-backfill -from-outbox-id 1 -to-outbox-id 100000 -batch-size 1000
```

That flag is not implemented yet; the current implementation covers online projection from newly published outbox rows.

## Metrics

Metrics:

- `avf_analytics_mirror_*` for raw outbox mirror health.
- `avf_analytics_projection_insert_ok_total`, `avf_analytics_projection_insert_failed_total`, `avf_analytics_projection_enqueue_dropped_total`, `avf_analytics_projection_marshal_failed_total`, `avf_analytics_projection_skipped_total`, and `avf_analytics_projection_lag_seconds` for typed projections.
