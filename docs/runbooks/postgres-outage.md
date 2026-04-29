# PostgreSQL outage / degraded availability

PostgreSQL is the **system of record**. Without it, **`cmd/api`** readiness fails when **`READINESS_STRICT=true`**, and transactional flows stop.

## Symptoms

- **`GET /health/ready`** returns **503** when Postgres probes fail.
- Logs show connection refused / timeout / TLS errors from **`DATABASE_URL`** poolers.
- Workers stop committing (`cmd/worker`, `cmd/reconciler`, `cmd/mqtt-ingest` ingest persistence paths).

## Mitigations

1. Confirm outage scope (single AZ vs regional) via provider dashboard / replicas.
2. Fail traffic only after validating replica lag **never** promotes stale writes—follow your provider’s HA playbook.
3. Restore service **before** replaying MQTT/outbox-heavy bursts; backlog drains via [`outbox.md`](outbox.md) and telemetry consumers [`telemetry-jetstream-resilience.md`](telemetry-jetstream-resilience.md).

## Related

- DR posture: [`production-backup-restore-dr.md`](production-backup-restore-dr.md), [`multi-region-dr-readiness.md`](multi-region-dr-readiness.md).
- Metrics/alerts: [`observability-alerts.md`](observability-alerts.md).
