# Production reset with bootstrap admin (design only — DO NOT RUN on production without gates)

This runbook describes a **full production data reset** that wipes all business data **and all auth accounts**, then bootstraps a fresh `platform_admin` via `cmd/bootstrap-admin`. It is the successor to `production-purge-keep-admin.sql`, which preserved `admin@avf.com`.

**Status:** design and tooling only. Execution requires explicit business approval and retention sign-off (see [production-reset-retention-signoff-blocker.md](./production-reset-retention-signoff-blocker.md)).

## Preconditions

- Written retention sign-off or archived exports of financial/audit records
- Maintenance window agreed with operations
- Supabase snapshot + `backup_managed_postgres.sh execute` drill validated on a disposable target
- `goose status` recorded; image digest pinned for rollback

## Gates (in order)

| Gate | Action |
|------|--------|
| 1 — PRECHECK | Verify Supabase project, `APP_ENV`, `PAYMENT_ENV`, redacted `DATABASE_URL`, `goose status` |
| 2 — BACKUP | Dashboard snapshot + `backup_managed_postgres.sh execute` to `/var/backups/avf/prod/` |
| 3 — BACKUP VERIFY | `restore_managed_postgres.sh validate` + drill restore to disposable DB |
| 4 — DRY RUN | `production-purge-dry-run.sql` archived as evidence |
| 5 — HUMAN CONFIRM | `CONFIRM_PRODUCTION_RESET=I_UNDERSTAND_THIS_WIPES_PRODUCTION` **and** SQL session `SET avf.confirm_production_reset=...` |
| 6 — WRITE FREEZE | Stop `worker`, `mqtt-ingest`, `reconciler`; announce window |
| 7 — RESET | `production-reset-bootstrap-admin.sql`, Redis `FLUSHDB`, media purge, EMQX machine users |
| 8 — SCHEMA VERIFY | `goose status` unchanged; `/app/migrate validate` |
| 9 — ADMIN SEED | `/app/bootstrap-admin` with `BOOTSTRAP_ADMIN_USERNAME` / `BOOTSTRAP_ADMIN_PASSWORD` from env |
| 10 — LOGIN + RBAC | `POST /v1/auth/login` with username; `/v1/auth/me` shows `platform_admin`; outbox route returns 200 |
| 11 — POST-RESET COUNTS | Exactly one `platform_auth_accounts` row; no pre-reset sessions/tokens |
| 12 — UNFREEZE | Restart workers; begin fleet re-provisioning |

## Orchestrator

```bash
CONFIRM_PRODUCTION_RESET=I_UNDERSTAND_THIS_WIPES_PRODUCTION \
  bash scripts/ops/run-production-reset-bootstrap-admin.sh all
```

Phases: `preflight | postgres | redis | media | emqx | bootstrap | verify | all`

## Bootstrap admin

Run inside the app container (same digest as API):

```bash
docker compose exec -e BOOTSTRAP_ADMIN_USERNAME=admin -e BOOTSTRAP_ADMIN_PASSWORD='<supplied at runtime>' api /app/bootstrap-admin
```

- Deliberately bypasses password policy (bootstrap-only); rotate immediately via `POST /v1/auth/change-password`
- Idempotent: skips insert when username already exists
- Refuses when any account exists unless `-force`

## Rollback

Postgres: Supabase PITR or `restore_managed_postgres.sh execute`. Redis, media, and EMQX users are **not** covered by the DB backup.

## Fleet impact

Every machine credential, session, MQTT user, and activation state is destroyed. The fleet must be re-provisioned machine-by-machine.
