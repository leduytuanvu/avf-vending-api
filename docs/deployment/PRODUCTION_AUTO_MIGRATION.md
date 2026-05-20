# Production automatic database migration

Production deploy runs **database backup + goose migrations** from the **same digest-pinned app image** being rolled out. Migration failure fails the GitHub Actions job and leaves previously running containers serving traffic when possible.

## When migration runs

| Trigger | Behavior |
|---------|----------|
| **Deploy Production** (`deploy-prod.yml`) with `run_migration: true` (default) | App node A runs `production-migrate.sh` before Caddy drain and service recreate |
| `run_migration: false` | Image-only rollout; no schema change (emergency use only) |
| Rollback | Does **not** run goose down; redeploys prior digest-pinned images only |

GitHub Actions concurrency group **`production-deploy`** ensures only one production deploy/rollback runs at a time (`cancel-in-progress: false`).

## Deploy sequence (app node)

1. Validate prerequisites; write new `APP_IMAGE_REF` to `.env.app-node`
2. Pull new images (migrations are embedded at `/app/migrations` in the app image)
3. **`scripts/deploy/production-migrate.sh`** (if `RUN_MIGRATION=1`):
   - Validate `/app/migrations` via `/app/migrate validate`
   - `pg_dump` backup (custom format)
   - Verify backup with `pg_restore -l`
   - Acquire PostgreSQL advisory lock
   - `migrate status` → `migrate up` → `migrate status`
4. Drain traffic (stop Caddy)
5. Recreate api, worker, reconciler, mqtt-ingest (+ optional temporal-worker)
6. Container health checks; start Caddy
7. Workflow public smoke: `/health/live`, `/health/ready`, `/version`

If step 3 fails, step 5 does not run; old containers keep serving until drain (migration runs **before** drain).

## Migration source of truth

- SQL files: repository root `migrations/*.sql` (goose format)
- Baked into production image at build time: `/app/migrations`
- Runner: `/app/migrate` (`cmd/migrate`) — commands: `validate`, `status`, `version`, `up`
- **Not** used: server-side tar-synced `migrations/`, `/opt/avf-backups/**`, separate goose-only image bind mounts

## Backups

| Item | Location |
|------|----------|
| Inline deploy backup | `/opt/avf-vending-api/deployments/prod/logs/migrations/backup-YYYYMMDDTHHMMSSZ.dump` |
| Migration log | `/opt/avf-vending-api/deployments/prod/logs/migrations/migrate-YYYYMMDDTHHMMSSZ.log` |

Backups are **custom-format** (`pg_dump -Fc`). Verified with `pg_restore -l` before migration proceeds.

Optional workflow input `backup_evidence_id` validates supplementary restore-drill JSON from a prior GitHub Actions run; it is **not** required when inline backup is used.

### Manual restore from inline backup

**Only when you intentionally need to revert database state.** This does not undo application code; coordinate with an image rollback.

```bash
# On app node — replace TIMESTAMP and confirm target DB first
BACKUP="/opt/avf-vending-api/deployments/prod/logs/migrations/backup-TIMESTAMP.dump"
# Load DATABASE_URL from .env.app-node (do not echo it)
set -a && source /opt/avf-vending-api/deployments/prod/app-node/.env.app-node && set +a

docker run --rm \
  -e "DATABASE_URL=${DATABASE_URL}" \
  -v "$(dirname "${BACKUP}"):/backup:ro" \
  postgres:17-alpine \
  pg_restore -d "${DATABASE_URL}" --clean --if-exists "/backup/$(basename "${BACKUP}")"
```

Review [production backup restore drill](../operations/production-backup-restore-drill.md) before any production restore.

## Inspecting migration state

On the VPS (app node):

```bash
cd /opt/avf-vending-api/deployments/prod/app-node
docker compose --env-file .env.app-node -f docker-compose.app-node.yml \
  --profile migration run --rm migrate status
docker compose --env-file .env.app-node -f docker-compose.app-node.yml \
  --profile migration run --rm migrate version
```

Local image validation after build:

```bash
docker build -f deployments/prod/Dockerfile -t avf-vending-api:migration-check .
bash scripts/deploy/validate_migration_image.sh avf-vending-api:migration-check
bash scripts/deploy/production-migrate.sh --validate-only \
  COMPOSE_FILE=deployments/prod/app-node/docker-compose.app-node.yml \
  COMPOSE_ENV_FILE=deployments/prod/app-node/.env.app-node.example
```

## If migration fails

1. GitHub Actions job fails; deploy manifest records failure
2. Running app containers on that node were **not** recreated (migration runs before drain)
3. Backup file and log remain under `logs/migrations/` for investigation
4. Fix forward: new migration or hotfix commit → redeploy
5. **Do not** run goose down in production unless explicitly documented and approved
6. **Do not** auto-restore DB from backup in the deploy script; restore is manual and gated

## Adding migrations safely

1. Add goose SQL under `migrations/` with sequential version prefix (`00004_...sql`)
2. Prefer **expand/contract**: add columns/tables first; remove old shapes in a later deploy after app code no longer needs them
3. Avoid destructive changes (drop column/table, data rewrite) in the same deploy as app code that depends on the new shape
4. **No manual production DB edits** — schema changes go through goose only
5. **No reset/drop schema** in production deploy paths
6. Dev-only seeds (e.g. `00003_seed_dev.sql`) must already be applied or must not be pending on production; production DBs should have goose version ≥ that file

## Security and safety

- `DATABASE_URL` is masked in logs (`user:***@host/db`)
- `scripts/verify_database_environment.sh` blocks dev/test URLs when `APP_ENV=production`
- PostgreSQL advisory lock (`MIGRATION_ADVISORY_LOCK_ID`, default `90420520260520`) prevents concurrent migrate runs
- Manual runs require `CONFIRM_PRODUCTION_MIGRATION=true` outside GitHub Actions
- Idempotent: `goose up` with no pending migrations exits successfully

## Related files

| File | Role |
|------|------|
| `scripts/deploy/production-migrate.sh` | Backup + migrate orchestration |
| `cmd/migrate/main.go` | Goose wrapper binary |
| `deployments/prod/Dockerfile` | Embeds migrations + migrate binary |
| `deployments/prod/app-node/scripts/release_app_node.sh` | Calls migrate before drain |
| `.github/workflows/deploy-prod.yml` | `run_migration` default `true`, concurrency `production-deploy` |
| `docs/audits/PRODUCTION_AUTO_MIGRATION_DEPLOY_AUDIT.md` | Pre-implementation audit |
