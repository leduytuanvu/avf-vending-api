# Capacity planning (100–1000 machines)

This document summarizes OLTP guardrails implemented under **P2.3 / P2.5** so operators can size Postgres, Redis, NATS JetStream telemetry, and worker pools without rewriting the modular monolith.

## PostgreSQL

- **Pool sizing**: `DATABASE_MAX_CONNS` is the global ceiling (validated with a hard upper bound); per-process overrides (`API_DATABASE_MAX_CONNS`, `WORKER_DATABASE_MAX_CONNS`, …) must be **≤ `DATABASE_MAX_CONNS`**.
- **Slow query logging**: optional `DATABASE_SLOW_QUERY_LOG_MS` (milliseconds). When > 0, queries slower than the threshold emit a structured **`postgres_slow_query`** log line (SQL truncated). Disabled when unset/`0`.

## Hot indexes

Migration **`00066_p25_capacity_indexes.sql`** adds **additive** btree/partial indexes for:

- Orders by **organization + machine + time**
- Payment attempts by **provider_reference** (non-empty)
- Outbox pending rows by **status + next_publish_after**
- Audit events by **organization + occurred_at + action**
- Command attempts by **machine + status + sent_at**
- Device telemetry by **machine + event_type + received_at**

Existing indexes on overlapping columns remain; new indexes complement reporting and ops queries.

Migration **`00071_p23_capacity_cost_indexes.sql`** adds further **additive** indexes for:

- **`payment_provider_events`** by **provider + provider_ref** (partial, non-empty ref)
- **`audit_logs`** by **organization + created_at + action**
- **`command_ledger`** by **machine + created_at + command_type**

## Redis catalog cache

- Sale catalog snapshots may be cached under **`CACHE_ENABLED`** with **`SALE_CATALOG_CACHE_TTL`**.
- Cache keys include **machine shadow version**, **per-organization media epoch** (`BumpOrganizationMedia` on catalog/media mutations), and **machine `config_revision`** so planogram/config publishes invalidate cached bootstrap/catalog rows without waiting for TTL expiry.
- Prometheus: **`avf_sale_catalog_cache_hits_total`** / **`avf_sale_catalog_cache_miss_total`**.

## Machine ingress (gRPC)

Configurable via **`CAPACITY_*`** env vars (see `internal/config/capacity.go`):

- Telemetry batch **event count** and **serialized protobuf size**
- Offline replay batch **event count**
- Media manifest **maximum entry count** before **`RESOURCE_EXHAUSTED`**

## Workers (`cmd/worker`)

- **`WORKER_RECOVERY_SCAN_MAX_ITEMS`** bounds reliability scan batches (outbox, payments, stuck commands).
- **`WORKER_OUTBOX_DISPATCH_MAX_ITEMS`** (optional) overrides the outbox lease/list batch for a single tick; when unset the worker uses the recovery scan limit.
- Tick intervals: **`WORKER_TICK_OUTBOX_DISPATCH`**, **`WORKER_TICK_PAYMENT_TIMEOUT_SCAN`**, **`WORKER_TICK_STUCK_COMMAND_SCAN`**.
- **`WORKER_CYCLE_BACKOFF_AFTER_FAILURE`** adds sleep after a failed cycle before the next tick.
- Prometheus: **`avf_worker_job_cycle_seconds`** histogram by job and result.

## Reporting HTTP

- **`REPORTING_SYNC_MAX_SPAN_DAYS`** caps synchronous JSON reporting windows.
- **`REPORTING_EXPORT_MAX_SPAN_DAYS`** caps CSV / export-style requests (`format=csv` or `/export` paths). If unset, the export horizon defaults to at least the sync horizon (and up to **730** days) so operators can widen sync without an extra env var. Validation requires **export ≥ sync**.

## Abuse protection (admin REST)

When **`RATE_LIMIT_ENABLED`** is on, **`RATE_LIMIT_REPORTS_READ_PER_MIN`** bounds **GET** traffic to **`/reports`** paths per authenticated subject + organization.

See also [cost-optimization.md](../runbooks/cost-optimization.md).
