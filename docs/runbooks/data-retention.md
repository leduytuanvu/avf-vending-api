# Enterprise Data Retention

PostgreSQL remains the source of truth. Retention cleanup is a bounded worker job, not part of local API startup.
Destructive cleanup defaults off for local/test unless `RETENTION_ALLOW_DESTRUCTIVE_LOCAL=true` is set.

## Configuration

Telemetry cleanup uses `TELEMETRY_RETENTION_DAYS`, `TELEMETRY_CRITICAL_RETENTION_DAYS`, `TELEMETRY_CLEANUP_ENABLED`, `TELEMETRY_CLEANUP_BATCH_SIZE`, and `TELEMETRY_CLEANUP_DRY_RUN`. It covers raw/derived telemetry evidence such as device telemetry events, check-ins, state transitions, incidents, rollups, and diagnostic bundle manifests. `TELEMETRY_CLEANUP_ENABLED` defaults off in development/test and on in staging/production.

Enterprise operational cleanup is disabled by default and runs only from `cmd/worker` when `ENTERPRISE_RETENTION_CLEANUP_ENABLED=true`:

- **Aliases**: `PAYMENT_EVENT_RETENTION_DAYS` applies when `PAYMENT_WEBHOOK_EVENT_RETENTION_DAYS` is unset; `OUTBOX_RETENTION_DAYS` applies when `OUTBOX_PUBLISHED_RETENTION_DAYS` is unset.

- `COMMAND_RETENTION_DAYS`: command ledger rows become eligible only when old and no related command attempts are `pending` or `sent`.
- `COMMAND_RECEIPT_RETENTION_DAYS`: old `device_command_receipts` rows are removed after the command receipt evidence horizon.
- `PAYMENT_WEBHOOK_EVENT_RETENTION_DAYS`: payment provider events are eligible only when linked to terminal, reconciled payments and `legal_hold=false`.
- `OUTBOX_PUBLISHED_RETENTION_DAYS`: only `outbox_events.status='published'` rows with `published_at` older than the horizon are deleted.
- `AUDIT_RETENTION_DAYS`: audit tables are pruned only when cleanup is explicitly enabled and `legal_hold=false`. Choose a compliance-safe value.
- `PROCESSED_MESSAGE_RETENTION_DAYS`: removes old `messaging_consumer_dedupe` rows after the replay window.
- `REFRESH_TOKEN_RETENTION_DAYS`: removes expired server-side refresh-token rows after the configured horizon.
- `OFFLINE_EVENT_RETENTION_DAYS`: prunes **terminal** `machine_offline_events` (`processed`, `succeeded`, `failed`, `duplicate`, `replayed`, `rejected`) by `received_at`; never deletes pending/processing rows.
- `INVENTORY_EVENT_RETENTION_DAYS`: trims aged append-only `inventory_events` ledger rows by `occurred_at` (bounded batches).
- `ENTERPRISE_RETENTION_CLEANUP_BATCH_SIZE`: max rows per table per delete batch.
- `ENTERPRISE_RETENTION_CLEANUP_DRY_RUN`: skips deletes.
- `RETENTION_ALLOW_DESTRUCTIVE_LOCAL`: must be true to run destructive retention cleanup in development/test.

Object storage artifacts are not deleted directly by this worker. Treat PostgreSQL artifact/diagnostic metadata as the retention index, and configure lifecycle rules in the object store for media, OTA artifacts, diagnostic bundles, and evidence buckets to match legal/compliance requirements. Do not delete object versions that are referenced by records under legal hold.

## Legal Hold

`audit_events`, legacy `audit_logs`, and `payment_provider_events` include a `legal_hold` flag. Retention SQL explicitly skips rows with this flag set. Use legal hold for disputes, investigations, chargebacks, security incidents, or regulatory preservation orders.

There is currently no public API to set legal hold in bulk; apply it through an audited operator/admin database procedure until an admin legal-hold workflow is designed.

## Privacy Controls

Audit/event metadata is sanitized before persistence. Keys containing password, token, secret, authorization, webhook/HMAC, PAN/card, CVV/CVC, expiry, account number, or IBAN are stored as `[REDACTED]`. JSON string values that contain Luhn-valid card numbers are also redacted. Payment webhook payload and provider metadata pass through this sanitizer before they are stored, so the system does not retain full PAN or card secrets from PSP callbacks.

### Worker master switch / global dry-run

- `ENABLE_RETENTION_WORKER`: master gate for `cmd/worker` telemetry + enterprise retention tickers (defaults **on** outside development/test).
- `RETENTION_DRY_RUN`: global override — merges with subsystem dry-run flags so **no Postgres deletes** run when enabled.

### Admin APIs (`platform_admin`)

- `GET /v1/admin/system/retention/stats` — configured horizons + enterprise footprint tables + runtime flags (`destructiveRetentionAllowed`, worker enabled).
- `POST /v1/admin/system/retention/dry-run` — candidate counts without deletes (audited **retention.dry_run**).
- `POST /v1/admin/system/retention/run` — executes bounded deletes when cleanup toggles are on; returns **403** in development/test unless `RETENTION_ALLOW_DESTRUCTIVE_LOCAL=true` (audited **retention.run**).

Optional query **`organization_id`** scopes enterprise audit attribution when present.

The cleanup job uses repeated `DELETE ... WHERE id IN (SELECT ... ORDER BY ... LIMIT $batch)` style statements, so one cycle never performs an unbounded table delete. It does not delete pending outbox rows, failed/dead-letter rows, unresolved payment events, active command attempts, or rows under legal hold.

## Partitioning Strategy

At 100-1000 machines, start with batch cleanup plus indexes already present on timestamp columns. Consider monthly partitions only when retention deletes become a measurable source of vacuum pressure or table bloat. Candidate tables are `device_telemetry_events`, `audit_events`, `payment_provider_events`, `outbox_events`, and high-volume command trace tables.

Partitioning should be introduced with a migration plan that preserves existing indexes and foreign key behavior. Do not detach/drop partitions for compliance-sensitive audit or payment evidence unless the configured retention policy and legal requirements allow it.
