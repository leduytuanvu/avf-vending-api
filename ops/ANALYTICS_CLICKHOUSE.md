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
| `ANALYTICS_CLICKHOUSE_TABLE` | `avf_outbox_mirror` | Table name in that database; only `[A-Za-z0-9_]`. |
| `ANALYTICS_MIRROR_MAX_CONCURRENT` | `8` | Bounded async inserts; additional enqueue attempts increment `avf_analytics_mirror_enqueue_dropped_total` and log a warning. |
| `ANALYTICS_INSERT_TIMEOUT` | `5s` | Per HTTP insert attempt. |
| `ANALYTICS_INSERT_MAX_ATTEMPTS` | `3` | Retries per row; exhausted attempts increment `avf_analytics_mirror_insert_failed_total`. |

If `ANALYTICS_MIRROR_OUTBOX_PUBLISHED=true` but ClickHouse is disabled, **config load fails** with an explicit error.

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

## Failure behavior

- Publish + Postgres mark success **does not** depend on the mirror.
- Mirror runs **only** inside the `marked == true` branch after a successful mark.
- Shutdown: worker calls mirror sink `Shutdown()` before closing the HTTP client so in-flight inserts get a chance to finish (bounded by timeouts).

## Metrics

See `ops/METRICS.md` (`avf_analytics_mirror_*`).
