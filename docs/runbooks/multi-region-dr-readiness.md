# Multi-region and DR readiness

This runbook makes the current DR posture explicit. It prepares the repo for future multi-region operation without enabling active-active writes.

## Current topology

- PostgreSQL is the source of truth for business state, command ledgers, idempotency ledgers, audit, machine state, and finance data.
- Redis is non-authoritative. It may hold cache, rate limit, session, revocation, lock, and catalog cache data, but it must not be the only copy of a business fact.
- NATS/JetStream is a durable transport/replay buffer for outbox and telemetry flows, not the system of record.
- Object storage holds artifact/media/diagnostic blobs. PostgreSQL stores metadata and references.
- App nodes are stateless and may be rebuilt from image digests plus environment/secrets.

## Runtime identity

Every production process must expose stable runtime identity in logs and `/version`:

- `APP_REGION`: logical deployment region, for example `ap-southeast-1`.
- `APP_NODE_NAME`: stable node name, for example `app-node-a`.
- `APP_INSTANCE_ID`: unique process instance id, for example `app-node-a-api`, `app-node-a-worker`, or `app-node-b-mqtt-ingest`.
- `APP_RUNTIME_ROLE`: optional process role override; normally the binary name is enough.

Rules:

- All processes writing to the same production PostgreSQL primary must use the same `APP_REGION` label.
- `APP_INSTANCE_ID` must be unique per running process and stable enough for logs and incident timelines.
- Do not include secrets, IPs that reveal private topology unnecessarily, or random per-restart values.
- Future cross-region standby nodes must not perform writes unless there is an explicit single-writer failover decision.

## ID and idempotency safety

The repo already uses UUIDs, provider event ids, command sequences scoped by machine, and explicit idempotency keys. For DR and replay safety:

- Idempotency keys must be deterministic for the business action, not for the node that processed it.
- Do not prefix keys with `APP_REGION`, `APP_NODE_NAME`, or `APP_INSTANCE_ID`; a replay after failover must hit the same logical key.
- Machine offline replay identity remains `idempotency_key`, `event_id`, or `boot_id` plus `seq_no` plus event type.
- Outbox events must keep stable event/idempotency keys so JetStream replay or worker restart does not double-apply side effects.
- Payment provider webhook identity remains provider plus webhook event id/provider reference. Region failover must not rewrite provider ids.

## PostgreSQL RPO/RTO

Production target:

- RPO: 15 minutes or better when PITR is enabled; if only snapshots exist, RPO is the snapshot interval.
- RTO: 4 hours for provider-managed restore into an isolated recovery environment; lower only after a measured drill proves it.

Minimum expectations:

- PITR enabled with a documented retention window.
- Daily snapshots retained according to compliance policy.
- Restore drill at least quarterly.
- Backup evidence retained before every production migration.

Restore validation must check:

- schema migration version;
- row counts for organizations, machines, orders/payments, command ledger, audit events, outbox, machine replay/idempotency tables;
- read-only API health and representative tenant-scoped reads;
- no connection from the recovery environment back to production Redis/NATS/object buckets unless explicitly intended.

## Redis recovery

Redis is rebuildable. After Redis loss:

- restart affected processes after provider connectivity is restored;
- expect cache misses, rate limit counters, login failure counters, revocation cache, session cache, locks, and catalog/media cache epochs to be lost according to enabled features;
- rely on PostgreSQL for authoritative account status, machine credentials, payments, orders, audit, inventory, and command state;
- if a revocation/session feature is configured fail-closed in production, recover Redis before admitting traffic.

Do not backfill business state from Redis into PostgreSQL.

## NATS and JetStream recovery

JetStream streams are replay buffers. Postgres remains authoritative.

- Outbox publishing: replay from PostgreSQL outbox rows when possible. Dead-lettered rows require operator review before retry.
- Telemetry projection: JetStream redelivery may reprocess messages. Stable idempotency keys and payload-hash guards must suppress duplicate applies.
- Retention: size `TELEMETRY_STREAM_MAX_BYTES` and `TELEMETRY_STREAM_MAX_AGE` to cover expected outage windows. If retention expires, recover from PostgreSQL state and device reconciliation where possible.
- DLQ: preserve DLQ subjects/streams during incident analysis, but do not treat them as permanent backup.

After rebuilding NATS, validate:

- readiness of streams and consumers;
- outbox pending/dead-letter counts;
- telemetry consumer lag;
- no replay storm against PostgreSQL connection pool.

## Object storage recovery

Object storage must support recovery of media, OTA artifacts, and diagnostic bundles:

- enable bucket versioning in production;
- enable encryption at rest;
- restrict app credentials to least privilege;
- document retention/lifecycle rules so recent versions are not purged before RPO;
- keep bucket and provider region in backup evidence, without committing secrets or presigned URLs.

PostgreSQL metadata restore is not enough if object versions are gone. A restore drill must validate at least one product/media URL, one OTA artifact metadata row if OTA is enabled, and one diagnostic bundle object if present.

## Restore drill validation checklist

Use a staging, preprod, or disposable recovery target.

1. Select a backup or PITR timestamp and record expected RPO.
2. Restore PostgreSQL to an isolated instance.
3. Point app env at the restored database with a non-production `APP_REGION` such as `restore-drill`.
4. Keep Redis empty or isolated unless testing Redis recovery specifically.
5. Recreate NATS/JetStream empty first; only restore stream data if the drill explicitly tests replay.
6. Configure object storage read-only for validation, or use a copied recovery bucket.
7. Run:
   - `go run ./cmd/cli -validate-config`
   - `/health/live`
   - `/health/ready`
   - `/version` and confirm `region`, `node_name`, and `instance_id`
   - representative admin read-only queries
   - object download/presign smoke if artifacts are enabled
8. Run a controlled replay test with a known idempotency key and confirm duplicate replay is not double-applied.
9. Record actual RTO, observed RPO, failures, and manual fixes.

## Production failover guardrails

- Active-active writes are not supported.
- Promote only one PostgreSQL writer.
- Freeze migrations during DR until the writer and migration history are confirmed.
- Update `DATABASE_URL`, `NATS_URL`, Redis URL, object storage endpoint, and DNS as separate audited changes.
- Bring app nodes back gradually and watch database pool saturation, outbox lag, and JetStream lag.

