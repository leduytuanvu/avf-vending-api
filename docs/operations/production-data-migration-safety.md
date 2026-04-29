# Production data migration safety checklist

Use this checklist before any production schema or data migration. It complements the backup evidence gate and does not replace production environment approvals.

## Required preflight

- Confirm the migration is necessary for the release and cannot be deferred.
- Confirm the migration is additive or backward-compatible. Destructive operations require explicit exception approval and rollback reasoning.
- Confirm the app image and goose image are digest-pinned.
- Confirm staging or preprod has run the same migration path.
- Confirm backup evidence is valid per `docs/runbooks/backup-evidence-for-production-migrations.md`.
- Confirm restore drill evidence is valid enough for the risk of the migration.
- Confirm current `APP_REGION`, `APP_NODE_NAME`, and `APP_INSTANCE_ID` values are documented for every production process.

## Data safety review

- New columns must be nullable or have safe defaults unless all writers are updated first.
- New constraints must not scan or lock hot tables without an approved window.
- New indexes on large tables should be created concurrently when supported by the migration tooling.
- Backfills must be bounded, resumable, and observable.
- Idempotency keys, replay ledgers, outbox rows, audit rows, and payment/provider identifiers must not be rewritten.
- Redis must not be used as source data for a migration.
- JetStream replay must be paused or rate-limited if a backfill increases write pressure.

## Execution guardrails

- Freeze unrelated deploys during the migration window.
- Run one migration executor only.
- Keep app nodes on a compatible version while the migration runs.
- Monitor Postgres locks, connection pool usage, outbox lag, worker errors, and API readiness.
- Stop on unexpected lock waits, deadlocks, or row-count mismatches.

## Post-migration validation

- Verify schema version.
- Verify representative tenant reads and writes.
- Verify outbox publishing is not silently stalled.
- Verify payment webhook idempotency still replays safely.
- Verify machine offline replay with a duplicate key does not double-apply.
- Verify `/version` still reports the expected region/node/instance identity.

## Rollback posture

Application rollback is image-based. Database rollback is not automatic.

- If a migration is additive, prefer rolling back the app image and leaving additive schema in place.
- If data was backfilled incorrectly, write a new corrective migration instead of manually editing rows without evidence.
- If restore is required, follow `docs/runbooks/multi-region-dr-readiness.md` and record actual RPO/RTO.

