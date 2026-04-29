# Data retention architecture (PostgreSQL)

Operational telemetry, append-only ledgers, command traces, webhook evidence, and outbox publishes accumulate continuously. Retention keeps OLTP tables bounded without deleting authoritative commerce facts (`payments`, `orders`) directly — evidence tables (`payment_provider_events`) prune only under safe predicates (terminal payment state, reconciliation matched / not required, `legal_hold=false`).

## Layers

1. **Telemetry (`RunTelemetryRetention`)** — device telemetry projections, rollups, incidents, diagnostics metadata. Horizons: `TELEMETRY_RETENTION_DAYS` (standard) vs `TELEMETRY_CRITICAL_RETENTION_DAYS` (critical-linked rows / coarse evidence).
2. **Enterprise (`RunEnterpriseRetention`)** — published outbox rows, resolved PSP events, terminal command ledger/receipts, dedupe, expired tokens, audit (`legal_hold=false`), inventory ledger trim, terminal offline replay rows.
3. **Scheduling** — `cmd/worker` runs daily ticks when `ENABLE_RETENTION_WORKER=true` and subsystem cleanup toggles are enabled. `RETENTION_DRY_RUN` forces candidate counting / metrics without deletes.

## Partitioning / archive

Primary strategy remains **indexed batch DELETE** plus additive indexes (e.g. terminal offline replay retention index). Monthly partitioning is optional when vacuum pressure dominates; introduce via additive migrations without dropping FK-backed audit trails.

## Safety

- Development/test refuse destructive deletes unless `RETENTION_ALLOW_DESTRUCTIVE_LOCAL=true`.
- Admin APIs (`GET|POST /v1/admin/system/retention/*`) expose stats and explicit dry-run/run with enterprise audit (**retention.dry_run**, **retention.run**).
