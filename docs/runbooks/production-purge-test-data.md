# Production purge — test data wipe (keep admin@avf.com)

Clear all production **test** data while preserving:

- Schema and goose migration version (no `DROP SCHEMA`, no `goose down`)
- Single admin account `admin@avf.com` (password unchanged)

Removes fleet, catalog, commerce, telemetry, activation leftovers, Redis cache, uploaded media, and EMQX **machine** MQTT credentials.

**Maintenance window:** ~30–60 minutes. Real machines lose binding until re-activated.

## Components

| Layer | Location | Action |
|-------|----------|--------|
| PostgreSQL | Supabase (`DATABASE_URL` on app-node A) | Selective purge SQL |
| Redis | Managed `REDIS_URL` | `FLUSHDB` |
| Media | Cloudinary (+ optional S3) | Delete product folder / bucket prefix |
| MQTT | EMQX on VPS B (`187.127.99.153`) | Delete machine users, keep `MQTT_USERNAME` |
| Web | `admin.ldtv.dev` | Re-login; fleet/catalog pages empty |

## Scripts (repository)

| File | Purpose |
|------|---------|
| `scripts/ops/production-purge-dry-run.sql` | Row counts before/after |
| `scripts/ops/production-purge-keep-admin.sql` | Gated purge (requires session guard) |
| `scripts/ops/run-production-purge.sh` | Full orchestration on app-node A |
| `scripts/ops/emqx-purge-machine-users.sh` | EMQX cleanup on VPS B |

## Phase 0 — Preflight (mandatory)

1. **Supabase snapshot** — Dashboard → Database → Backups / PITR. Record timestamp.

2. **Logical backup** on app-node A (`72.62.244.94`):

```bash
ssh root@72.62.244.94
cd /opt/avf-vending-api
bash deployments/prod/shared/scripts/backup_managed_postgres.sh \
  execute /var/backups/avf-pre-wipe-$(date -u +%Y%m%dT%H%M%SZ).dump
```

3. **Confirm admin exists:**

```bash
docker run --rm --env-file deployments/prod/app-node/.env.app-node \
  postgres:17-alpine psql "$DATABASE_URL" -c \
  "SELECT id, email, status FROM platform_auth_accounts WHERE lower(email)='admin@avf.com';"
```

If zero rows → **stop**. Do not purge.

4. **Stop workers** (reduce race during bulk delete):

```bash
docker compose -f deployments/prod/app-node/docker-compose.app-node.yml \
  --env-file deployments/prod/app-node/.env.app-node \
  stop worker mqtt-ingest reconciler
```

## Phase 1 — PostgreSQL

### Dry-run

```bash
cd /opt/avf-vending-api
docker run --rm --env-file deployments/prod/app-node/.env.app-node \
  -v "$(pwd)/scripts/ops:/ops:ro" postgres:17-alpine \
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f /ops/production-purge-dry-run.sql
```

### Purge (destructive)

Guard must be set in the same session:

```bash
docker run --rm --env-file deployments/prod/app-node/.env.app-node \
  -v "$(pwd)/scripts/ops:/ops:ro" postgres:17-alpine \
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 \
  -c "SET avf.confirm_production_purge='I_UNDERSTAND_THIS_WIPES_PRODUCTION';" \
  -f /ops/production-purge-keep-admin.sql
```

### Expected post-purge counts

```sql
SELECT count(*) FROM machines;                 -- 0
SELECT count(*) FROM products;                -- 0
SELECT count(*) FROM platform_auth_accounts;  -- 1
SELECT email FROM platform_auth_accounts;     -- admin@avf.com
SELECT version FROM goose_db_version ORDER BY version DESC LIMIT 1;  -- unchanged
```

**Note:** Migration `00003_seed_dev.sql` was applied on prod. The purge script includes explicit deletes for those deterministic UUIDs.

## Phase 2 — Redis

```bash
redis-cli -u "$REDIS_URL" FLUSHDB
# or: docker run --rm redis:7-alpine redis-cli -u "$REDIS_URL" FLUSHDB
```

Admin must **log in again** after this step.

## Phase 3 — Media

### Cloudinary

Read `CLOUDINARY_FOLDER` from `deployments/prod/app-node/.env.app-node` (typically `avf-vending/products`):

```bash
curl -X DELETE \
  "https://api.cloudinary.com/v1_1/${CLOUDINARY_CLOUD_NAME}/folders/${CLOUDINARY_FOLDER}" \
  -u "${CLOUDINARY_API_KEY}:${CLOUDINARY_API_SECRET}"
```

Or delete via Cloudinary Console → Media Library.

### S3 (if `OBJECT_STORAGE_BUCKET` is set)

```bash
aws s3 rm "s3://${OBJECT_STORAGE_BUCKET}/" --recursive
```

## Phase 4 — EMQX machine credentials

On VPS B (`187.127.99.153`):

```bash
cd /opt/avf-vending-api
CONFIRM_PRODUCTION_PURGE=I_UNDERSTAND_THIS_WIPES_PRODUCTION \
  bash scripts/ops/emqx-purge-machine-users.sh
```

Do **not** delete the service user `MQTT_USERNAME` (app ingest).

## Phase 5 — Restart and verify

```bash
cd /opt/avf-vending-api
docker compose -f deployments/prod/app-node/docker-compose.app-node.yml \
  --env-file deployments/prod/app-node/.env.app-node \
  start worker mqtt-ingest reconciler

curl -fsS https://api.ldtv.dev/health/live
curl -fsS https://api.ldtv.dev/version
```

1. Log in at https://admin.ldtv.dev/login as `admin@avf.com`
2. Confirm `/machines`, `/sites`, `/catalog/products`, `/planograms` are empty
3. Optional smoke:

```bash
ADMIN_EMAIL=admin@avf.com ADMIN_PASSWORD='...' \
  bash scripts/e2e/production-readonly-smoke.sh
```

## One-shot orchestration

On app-node A after syncing repo:

```bash
cd /opt/avf-vending-api
CONFIRM_PRODUCTION_PURGE=I_UNDERSTAND_THIS_WIPES_PRODUCTION \
  bash scripts/ops/run-production-purge.sh all
```

Phases can run individually: `preflight`, `postgres`, `redis`, `media`, `emqx`, `restart`, `verify`.

## GitHub Actions (optional)

Workflow **Production Purge Test Data** (`production-purge-test-data.yml`) runs the same steps via SSH when local access is blocked. Requires:

- `confirm_purge` input = `I_UNDERSTAND_THIS_WIPES_PRODUCTION`
- Production environment secrets (SSH key, hosts)

```bash
gh workflow run production-purge-test-data.yml \
  -f confirm_purge=I_UNDERSTAND_THIS_WIPES_PRODUCTION \
  -f phase=all
```

## Rollback

1. **Postgres:** Supabase PITR to pre-wipe timestamp **or**:

```bash
CONFIRM_MANAGED_RESTORE=managed-production-restore \
  bash deployments/prod/shared/scripts/restore_managed_postgres.sh \
  /var/backups/avf-pre-wipe-*.dump execute
```

2. **Redis:** no rollback — sessions rebuild on login
3. **Media:** Cloudinary/S3 versioning if enabled; otherwise re-upload
4. **EMQX:** re-bootstrap or re-activate each machine

## Risks

- Real machines offline until setup wizard / activation runbook
- Orders, payments, audit test data permanently lost (unless restored from backup)
- Purge SQL reviewed against schema v19; run dry-run on staging if available

## After purge

Start fresh machine setup via web `/machines/[id]/setup` or [production-setup-AVF111111-first-install.md](./production-setup-AVF111111-first-install.md).

**Never commit** `.env`, passwords, or backup dumps to git.
