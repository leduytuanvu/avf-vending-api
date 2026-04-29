# Transactional outbox (operators)

AVF persists asynchronous integration events in Postgres (`outbox_events`) alongside domain mutations (classic transactional outbox). `cmd/worker` leases eligible rows with `SKIP LOCKED`, publishes to JetStream (when wired), retries with backoff, and dead-letters poison rows after **max attempts**. **JetStream publish never runs inside the OLTP transaction** — only `InsertOutboxEvent` (and related row state) is committed with the aggregate; the worker publishes later. Operators inspect and intervene via admin APIs.

## HTTP (platform_admin JWT)

Stable aliases under **`/v1/admin/system/outbox`** mirror legacy **`/v1/admin/ops/outbox`** listings:

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/v1/admin/system/outbox/stats` | Pipeline counters only |
| GET | `/v1/admin/system/outbox` | Paginated rows + stats |
| GET | `/v1/admin/system/outbox/{eventId}` | Single row |
| POST | `/v1/admin/system/outbox/{eventId}/replay` | Reset Postgres DLQ quarantine so the worker can retry publish |
| POST | `/v1/admin/system/outbox/{eventId}/mark-dlq` | Manual DLQ (`dead_letter`) with optional `{ "note": "..." }` body |

Replay and manual DLQ emit **`audit_events`** in the **same PostgreSQL transaction** as the outbox row update (fail-closed via `RecordCriticalTx`).

## CLI (break-glass)

Minimal operations: [outbox-replay.md](./outbox-replay.md) (`cmd/outbox-replay`). Use only when admin HTTP is unavailable; dead-letter replay still requires explicit confirmation flags.

Audit rows require a valid **`organization_id`** FK. Resolution order for the audit event:

1. `outbox_events.organization_id` when present
2. otherwise the interactive principal's `organization_id` JWT scope
3. otherwise **`PLATFORM_AUDIT_ORGANIZATION_ID`** (env) for platform-scoped rows

Without a resolvable organization, replay/DLQ returns **`503`** with `platform_audit_org_unresolved`.

Legacy endpoints **`GET /v1/admin/ops/outbox`** and **`POST /v1/admin/ops/outbox/{outboxId}/retry`** remain supported.

OpenAPI schemas: `V1AdminOutboxRow`, `V1AdminOutboxOpsEnvelope`, `V1AdminOutboxStatsEnvelope`, `V1AdminOutboxRetryEnvelope`, `V1AdminOutboxMarkDLQEnvelope`.

## Worker and DLQ drill-down

Broker failures, backoff tuning, Prometheus gauges, and NATS/JetStream expectations are documented in [outbox-dlq-debug.md](./outbox-dlq-debug.md).

## Observability

Worker metrics under `avf_worker_outbox_*` include pending totals, dead-letter counts, lease pressure, **`failed_pending_total`** (unpublished rows in `failed` status), **publish-success lag** (`created_at`→mark `published_at`), and **JetStream RPC latency** per successful publish (`publish_jetstream_rpc_seconds`). See [outbox-dlq-debug.md](./outbox-dlq-debug.md).

## JetStream telemetry (separate)

Device telemetry ingestion uses **`AVF_TELEMETRY_*`** streams into Postgres projections. That path does **not** replace transactional outbox for payments, commerce, inventory, MQTT command dispatch, or audit. Critical side effects MUST be modeled as `outbox_events` rows in the OLTP transaction. See [telemetry-jetstream-resilience.md](./telemetry-jetstream-resilience.md).

## Safety notes

- Do not edit **`payload`** in place for poison messages; fix upstream data or replay after verifying idempotency keys.
- Scaling multiple worker replicas relies on Postgres leases (`locked_by`, `locked_until`); expect JetStream dedupe (`Nats-Msg-Id`) as a secondary guard if `published_at` lags behind a successful publish.
