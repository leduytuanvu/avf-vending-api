# Runbooks index

Operational procedures live under **`docs/runbooks/`**. Use this index to jump to incident-style guides referenced by architecture docs (**[`../architecture/data-flow.md`](../architecture/data-flow.md)**).

## Connectivity / dependency outages

| Scenario | Runbook |
| -------- | ------- |
| Redis unavailable | [`redis-outage-behavior.md`](redis-outage-behavior.md) |
| PostgreSQL unavailable | [`postgres-outage.md`](postgres-outage.md) |
| MQTT broker unavailable | [`mqtt-broker-outage.md`](mqtt-broker-outage.md) |
| Object storage / media CDN degraded | [`object-storage-outage.md`](object-storage-outage.md) |

## Machine / MQTT / telemetry

| Scenario | Runbook |
| -------- | ------- |
| Machine offline / degraded connectivity | [`machine-offline.md`](machine-offline.md) |
| Command stuck / not ACK’d | [`mqtt-command-stuck.md`](mqtt-command-stuck.md), [`mqtt-command-debug.md`](mqtt-command-debug.md) |
| MQTT ingest limits / telemetry pressure | [`mqtt-ingest-telemetry-limits.md`](mqtt-ingest-telemetry-limits.md), [`telemetry-jetstream-resilience.md`](telemetry-jetstream-resilience.md) |

## Payments / commerce

| Scenario | Runbook |
| -------- | ------- |
| Payment mismatch / reconciliation queue | [`payment-reconciliation.md`](payment-reconciliation.md) |
| Webhook verification failures | [`payment-webhook-debug.md`](payment-webhook-debug.md) |

## Async pipeline / worker

| Scenario | Runbook |
| -------- | ------- |
| Outbox publish / DLQ | [`outbox.md`](outbox.md), [`outbox-dlq-debug.md`](outbox-dlq-debug.md) |

## Deploy / rollback

| Scenario | Runbook |
| -------- | ------- |
| Principal **production readiness** signoff (P0–P2 checklist, commands, blockers) | [`final-production-readiness-signoff.md`](final-production-readiness-signoff.md) |
| Roll back a deployment | [`rollback-production.md`](rollback-production.md), [`production-cutover-rollback.md`](production-cutover-rollback.md) |

## Local development

| Scenario | Runbook |
| -------- | ------- |
| Docker Compose / migrations / Swagger | [`local-dev.md`](local-dev.md) |

Full narrative inventory remains in **[`../README.md`](../README.md)** (Documentation → Operations).
