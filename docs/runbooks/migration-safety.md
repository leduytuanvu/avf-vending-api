# Migration safety (goose)

The AVF vending API uses **goose** SQL migrations under `migrations/` (`NNNNN_description.sql` with `-- +goose Up` / `-- +goose Down`).

CI and deploy workflows run **`scripts/ci/verify_migrations.sh`** (implemented by `tools/verify_migrations.py`) to:

- Validate filenames, strict version ordering, duplicate versions, and non-empty goose sections.
- Flag **destructive** patterns in SQL (see below).
- Write **`migration-evidence/migration-safety-report.json`** for evidence.

No database connection or credentials are required for these checks.

---

## Safe migration guidelines

1. Prefer **additive** changes: new tables/columns/indexes, `ADD COLUMN IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`.
2. Use **multi-step** releases for breaking changes (deploy code that supports both schemas, then shrink).
3. Avoid **data deletes** in `Up`; use batch jobs or application-level cleanup when needed.
4. Keep **`Down`** reversibility in mind; **`Down` is not scanned for destructive policy** (it is expected to reverse `Up`, often with `DROP` / `DROP COLUMN`).
5. **All targets** (CI, staging, production) evaluate **goose `Up` only** for destructive rules — matching what runs on `goose up` during deploy.

---

## Destructive patterns (blocked by default)

The verifier flags (case-insensitive, after stripping `--` and `/* */` comments):

| Pattern | Notes |
|--------|--------|
| `DROP TABLE` | |
| `DROP COLUMN` | |
| `TRUNCATE` | |
| `DROP INDEX` | Only if **not** `DROP INDEX IF EXISTS` (optional `CONCURRENTLY`); safe index replacement is allowed. |
| `ALTER TYPE` … `DROP` | Heuristic window |
| `ALTER TABLE` … `ALTER COLUMN` … `TYPE` | Type changes |
| `DELETE FROM` without `WHERE` | Per `;`-split statement (best effort) |
| `UPDATE` without `WHERE` | Per `;`-split statement (best effort) |

---

## Approval mechanics

| Context | Behavior |
|---------|----------|
| **CI / PR** (`DEPLOY_TARGET=ci`) | Any destructive hit **fails** the check. There is no bypass in CI. |
| **Staging deploy** | Fails if destructive unless repository variable **`ALLOW_DESTRUCTIVE_MIGRATIONS`** is set to `true` (workflow passes it into the gate). |
| **Production deploy** | Fails if destructive unless **`ALLOW_PROD_DESTRUCTIVE_MIGRATIONS`** is `true` **and** the job still passes normal **GitHub Environment** protections (reviewers, wait timer, etc.). |

**Production cannot run destructive migrations silently:** without the repo variable, the deploy job exits before SSH regardless of environment approval wording.

---

## Rollback expectations

- **Automatic production rollback** (image-only) **does not run `goose down`**. Database schema may remain ahead of rolled-back app images.
- Plan manual recovery (restore from backup + migration plan) if a migration + deploy combination is bad.

See also manifest notes in `deploy-prod.yml` (`migration_rollback_policy`, `auto_rollback_scope`).

---

## Database backup before production migration

- **`release_app_node.sh`** runs goose when `RUN_MIGRATION=1`; it does **not** take a database backup first.
- The **GitHub Actions** `deploy-prod.yml` workflow does **not** invoke `backup_postgres.sh` / `backup_managed_postgres.sh` automatically, but with **`run_migration: true`** on **Deploy Production** the operator **must** supply a non-empty `backup_evidence_id` in `workflow_dispatch` — the workflow fails in validation before Security Release artifact download and again before SSH if it is empty.
- For **image-only** rollouts, set `run_migration: false` (no backup fields required).

See **[backup-evidence-for-production-migrations.md](backup-evidence-for-production-migrations.md)** and the step summary on each production deploy.

---

## Commands

```bash
# Same as CI / pre-commit
DEPLOY_TARGET=ci bash scripts/ci/verify_migrations.sh

# Staging policy (simulated)
ALLOW_DESTRUCTIVE_MIGRATIONS=true DEPLOY_TARGET=staging bash scripts/ci/verify_migrations.sh

# Production policy (simulated; real runs use migration_preflight.sh from Actions)
ALLOW_PROD_DESTRUCTIVE_MIGRATIONS=true DEPLOY_TARGET=production bash scripts/ci/verify_migrations.sh
```

---

## Related

- `scripts/check_migrations.sh` — thin wrapper; use `scripts/ci/verify_migrations.sh` for the full gate.
- `docs/architecture/migration-strategy.md` — broader migration strategy.

## Prometheus signals during migration windows

- Watch **`http_requests_total` / `http_errors_total`** and **`grpc_requests_total` / `grpc_errors_total`** on API instances for spikes while goose runs or connections churn.
- **`outbox_pending_total`** / **`outbox_dlq_total`** (worker) — backlog growth or poison-message drain after schema changes affecting publishers.
- Catalog: [`docs/observability/production-metrics.md`](../observability/production-metrics.md).
