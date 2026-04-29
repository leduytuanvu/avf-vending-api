# Cost optimization runbook

Operational knobs introduced under **P2.3 / P2.5** to reduce repeated database load and bound abusive clients without weakening auth or audit guarantees.

## Quick checklist

| Area | Control | Notes |
|------|---------|------|
| Postgres pool | `DATABASE_*`, process overrides | Overrides must stay ≤ global max |
| Slow queries | `DATABASE_SLOW_QUERY_LOG_MS` | Warn-only logging; tune threshold from P95 |
| Redis catalog | `CACHE_ENABLED`, `SALE_CATALOG_CACHE_TTL` | Invalidate via media epoch bump + `config_revision` in cache key |
| Telemetry gRPC | `CAPACITY_MAX_TELEMETRY_GRPC_BATCH_*` | Rejects oversized protobuf payloads |
| Offline replay | `CAPACITY_MAX_OFFLINE_EVENTS_PER_REQUEST` | Caps events per `PushOfflineEvents` RPC |
| Reporting | `REPORTING_SYNC_MAX_SPAN_DAYS`, `REPORTING_EXPORT_MAX_SPAN_DAYS` | Sync caps JSON; exports allow a wider window but remain bounded |
| Admin abuse | `RATE_LIMIT_ENABLED`, `RATE_LIMIT_REPORTS_READ_PER_MIN` | Protects hot reporting reads |
| Worker | `WORKER_*` tick + scan limits; `WORKER_OUTBOX_DISPATCH_MAX_ITEMS` | Raise scans gradually; watch `avf_worker_job_cycle_seconds` |

## Telemetry retention

Telemetry retention jobs (`TelemetryDataRetention`, NATS JetStream limits) remain the authoritative durability strategy for high-frequency paths—see **`docs/runbooks/data-retention.md`** and **`TelemetryJetStream`** env vars.

## Redis outages

Catalog caching is opportunistic: on Redis miss the API rebuilds from Postgres. Enterprise deployments should monitor **`avf_sale_catalog_cache_miss_total`** spikes correlated with Postgres latency.

## Further reading

- [capacity-planning.md](../architecture/capacity-planning.md)
