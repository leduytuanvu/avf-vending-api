# Backup evidence for production migrations

Production deploys that run **goose `Up`** on the first app node (`run_migration: true` in **Deploy Production**) must record **verifiable** database backup information before the workflow reaches SSH, image pull, or remote migration.

The GitHub Actions workflow enforces this in the **Validate production deploy inputs** step (before **Gate production release before SSH**, migration policy checks, and any sync to hosts).

## Workflow inputs (manual production deploy)

| Input | When required | Description |
|-------|---------------|-------------|
| `run_migration` | Every deploy (default `true`) | If `true`, the first app wave runs goose **Up** on the new goose image. If `false`, the rollout is **image-only** (no goose on the first node); backup evidence is not required. |
| `backup_provider` | `run_migration: true` | How the database was protected (e.g. `hcloud-snapshot`, `rds-snapshot`, `s3-dump`, `backup_managed_postgres`, `gh-actions-artifact`). |
| `backup_evidence_id` | `run_migration: true` | Primary reference: snapshot id, S3 key, `host:/path` dump, GitHub `artifact:run:filename`, or change ticket. |
| `backup_completed_at_utc` | `run_migration: true` | When the backup finished, **ISO-8601 UTC** (e.g. `2026-04-26T12:00:00Z`). |
| `backup_approval_ref` | Optional | Approver, ticket URL, PagerDuty id, or runbook link. |

## What counts as acceptable evidence

- **VPS / volume snapshot** id and provider name.
- **Managed Postgres** script run or `pg_dump` path from `backup_managed_postgres.sh` (see `deployments/prod/shared/scripts/backup_managed_postgres.sh`) with a timestamp.
- **Cloud provider** automated backup or snapshot (RDS, Hetzner, etc.).
- **GitHub** artifact or release asset containing a dump (name + run id in `backup_evidence_id`).

The workflow does **not** create backups automatically; operators complete backup through normal procedures, then run **Deploy Production** with these fields.

## Artifacts and manifests

- Values are written to `deployment-evidence/production-deployment-manifest.json` and `production-deploy-evidence.json` under `run_migration_requested` and `db_backup_evidence` (see `scripts/release/write_deployment_manifest.py`). The same step also writes **`production-release-evidence.json`** and **`production-release-evidence.md`** (operator rollup including migration/backup, security gate ids, image digests, and LKG pointers) uploaded with the `production-deploy-evidence` artifact.
- Rollback mode does not require backup fields; production rollback remains **app/goose image only** and does **not** run `goose down`.

## Related

- [migration-safety.md](migration-safety.md) — destructive policy and operator expectations.
- [rollback-production.md](rollback-production.md) — no automatic database down on rollback.
