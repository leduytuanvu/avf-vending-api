# Runbook: Production migration — product media offline cache

**Scope:** Apply **`00004_product_media_offline_cache`** (goose revision **4**) and verify catalog/media schema + runtime behavior **after** the application images that understand this schema are deployed.

**Topology:** Production **app-node** Docker Compose under:

`/opt/avf-vending-api/deployments/prod/app-node`

(or your documented `PRODUCTION_DEPLOY_ROOT`; adjust paths consistently).

**Secrets:** `DATABASE_URL` and broker credentials live in **`.env.app-node`** on the server. **Never paste credentials into tickets, chat logs, or this document.**

---

## Preconditions

| Check | Notes |
|-------|--------|
| Code merged | Feature merged to **`develop`** / release branch and promoted per **`docs/runbooks/release-process.md`**. |
| Images digest-pinned | **`APP_IMAGE_REF`** and **`GOOSE_IMAGE_REF`** in `.env.app-node` match the release that contains migration **`00004_*`** and compatible API binaries. |
| Maintenance window | Coordinators notified; optional traffic drain per **`deployments/prod/shared/scripts/traffic_drain_hook.sh`**. |
| Backup evidence | For GitHub-driven production deploys with migrations, follow **`docs/runbooks/backup-evidence-for-production-migrations.md`** and record **`backup_evidence_id`** when required by workflow inputs. |

---

## OPERATOR CONFIRMATION REQUIRED

Every block below that mutates production state, stops services, or runs **`goose up`** MUST be executed **only** after explicit operator approval and peer review of the diff/migration list.

---

## A. Pre-check current containers

**OPERATOR CONFIRMATION REQUIRED**

```bash
cd /opt/avf-vending-api/deployments/prod/app-node

docker compose --env-file .env.app-node -f docker-compose.app-node.yml ps -a
```

Confirm **`api`**, **`worker`**, **`reconciler`**, **`mqtt-ingest`**, and **`caddy`** are in the expected state (healthy / running). Note **`migrate`** is absent unless you invoke the **`migration`** profile.

---

## B. Load `DATABASE_URL` safely

**Do not** `cat` or log full `.env.app-node`.

Preferred: export only what you need in a subshell for one-off commands:

```bash
cd /opt/avf-vending-api/deployments/prod/app-node

set -a
# shellcheck source=/dev/null
source ./.env.app-node
set +a

# Sanity: redacted identity only (from repo helper)
export APP_ENV="${APP_ENV:-production}"
bash /opt/avf-vending-api/scripts/verify_database_environment.sh
```

Alternatively run read-only verification without exporting globally:

```bash
VERIFY_ENV_FILE=/opt/avf-vending-api/deployments/prod/app-node/.env.app-node \
  bash /opt/avf-vending-api/scripts/db/verify_product_media_migration.sh
```

---

## C. Verify `pg_isready`

**OPERATOR CONFIRMATION REQUIRED** (connectivity to managed Postgres)

Using host **`psql`** (if installed):

```bash
set -a && source /opt/avf-vending-api/deployments/prod/app-node/.env.app-node && set +a
pg_isready -d "$DATABASE_URL" -t 10
```

Using **`postgres:17-alpine`** (no local client):

```bash
set -a && source /opt/avf-vending-api/deployments/prod/app-node/.env.app-node && set +a

docker run --rm \
  -e DATABASE_URL \
  postgres:17-alpine \
  sh -c 'pg_isready -d "$DATABASE_URL" -t 10'
```

---

## D. `pg_dump` backup (`postgres:17-alpine`)

**OPERATOR CONFIRMATION REQUIRED**

Pick a **private** artifact directory on the server (example: `/var/backups/avf-postgres/`). Ensure sufficient disk space.

```bash
set -a && source /opt/avf-vending-api/deployments/prod/app-node/.env.app-node && set +a

BACKUP_DIR=/var/backups/avf-postgres
mkdir -p "${BACKUP_DIR}"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
BACKUP_BASENAME="pre_product_media_offline_cache_${STAMP}.dump"
export BACKUP_BASENAME

docker run --rm \
  -e DATABASE_URL \
  -e BACKUP_BASENAME \
  -v "${BACKUP_DIR}:/backup" \
  postgres:17-alpine \
  sh -c 'pg_dump -Fc --dbname="$DATABASE_URL" -f "/backup/${BACKUP_BASENAME}"'

BACKUP_FILE="${BACKUP_DIR}/${BACKUP_BASENAME}"
```

> **Note:** The repo also ships **`deployments/prod/shared/scripts/backup_managed_postgres.sh`** for validated managed-Postgres backups; prefer that script if your process already standardizes on it (set **`APP_NODE_ENV_FILE_PATH`** / env file path as documented there).

---

## E. Verify backup non-empty and `pg_restore -l`

**OPERATOR CONFIRMATION REQUIRED**

```bash
test -s "${BACKUP_FILE}"
ls -la "${BACKUP_FILE}"

docker run --rm \
  -v "$(dirname "${BACKUP_FILE}"):/backup:ro" \
  postgres:17-alpine \
  pg_restore -l "/backup/$(basename "${BACKUP_FILE}")" >/tmp/pg_restore_list.txt

wc -l /tmp/pg_restore_list.txt
head -n 20 /tmp/pg_restore_list.txt
```

Expect **non-zero** file size and a **non-empty** TOC listing.

---

## F. Stop workloads / migration mode

**OPERATOR CONFIRMATION REQUIRED**

Two supported patterns — pick **one** consistent with your rollout policy.

### F1. Integrated rollout (recommended default)

The node script **`deployments/prod/app-node/scripts/release_app_node.sh`** already:

1. Stops **`caddy`** (traffic drain).
2. Optionally runs **`migrate`** when **`RUN_MIGRATION=1`** and **`CONFIRM_PRODUCTION_MIGRATION=true`**.
3. Recreates **`api`**, **`worker`**, **`reconciler`**, **`mqtt-ingest`**, then brings **`caddy`** back.

See **`deployments/prod/app-node/README.md`** (`RUN_MIGRATION=1`).

### F2. Manual stop before isolated migration

If policy requires **`goose`** outside `release_app_node.sh`:

```bash
cd /opt/avf-vending-api/deployments/prod/app-node

docker compose --env-file .env.app-node -f docker-compose.app-node.yml stop \
  caddy api worker reconciler mqtt-ingest
```

(Optional) Stop **`temporal-worker`** if you run with **`--profile temporal`**.

---

## G. Run `goose` migration

**OPERATOR CONFIRMATION REQUIRED**

### G1. Compose `migrate` service (matches deployed tree)

Requires **`GOOSE_IMAGE_REF`** digest-pinned in `.env.app-node` and repo **`migrations/`** mounted at **`../../../migrations`** as in **`docker-compose.app-node.yml`**.

```bash
cd /opt/avf-vending-api/deployments/prod/app-node

set -a && source ./.env.app-node && set +a
export APP_ENV="${APP_ENV:-production}"

bash /opt/avf-vending-api/scripts/verify_database_environment.sh

docker compose --env-file .env.app-node -f docker-compose.app-node.yml \
  --profile migration run --rm migrate
```

### G2. Release wrapper (pull + migrate + restart)

From **`deployments/prod/app-node`** (paths per README):

```bash
cd /opt/avf-vending-api/deployments/prod/app-node

export CONFIRM_PRODUCTION_MIGRATION=true
export RUN_MIGRATION=1

bash scripts/release_app_node.sh \
  "${APP_IMAGE_REF}" \
  "${GOOSE_IMAGE_REF}"
```

Replace refs with the **new** digest-pinned values for this release if not already written into `.env.app-node` by your pipeline.

---

## H. Start services

If you used **manual stop (F2)** without **`release_app_node.sh`**, bring workloads back:

```bash
cd /opt/avf-vending-api/deployments/prod/app-node

docker compose --env-file .env.app-node -f docker-compose.app-node.yml up -d --remove-orphans \
  api worker reconciler mqtt-ingest caddy
```

Add **`--profile temporal`** and **`temporal-worker`** if applicable.

---

## I. Verify containers healthy

```bash
cd /opt/avf-vending-api/deployments/prod/app-node

docker compose --env-file .env.app-node -f docker-compose.app-node.yml ps

bash scripts/healthcheck_app_node.sh
```

Compose healthchecks hit **`/health/live`** and **`/health/ready`** on each service (see **`docker-compose.app-node.yml`**).

---

## J. Verify DB schema (read-only SQL)

Run the bundled verifier (recommended):

```bash
VERIFY_ENV_FILE=/opt/avf-vending-api/deployments/prod/app-node/.env.app-node \
  bash /opt/avf-vending-api/scripts/db/verify_product_media_migration.sh
```

See **§ Verification SQL** below for raw queries.

---

## K. Public health endpoints

Through **Caddy** / `API_DOMAIN` (example host **`api.ldtv.dev`** — use yours):

```bash
curl -fsS "https://${API_DOMAIN}/health/live"
curl -fsS "https://${API_DOMAIN}/health/ready"
curl -fsS "https://${API_DOMAIN}/version"
```

Expect **`live`** / **`ready`** body **`ok`** per existing probes.

---

## L. Smoke — login / `auth/me`

**OPERATOR CONFIRMATION REQUIRED** (uses real credentials from your password manager / break-glass admin).

Use admin email/password from `.env.app-node` (**never** commit). Example shape:

```bash
BASE="https://${API_DOMAIN}"

TOKEN_JSON="$(curl -fsS -X POST "${BASE}/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"${ADMIN_EMAIL}\",\"password\":\"${ADMIN_PASSWORD}\"}")"

ACCESS="$(python3 -c 'import json,sys; print(json.loads(sys.argv[1])["tokens"]["accessToken"])' "${TOKEN_JSON}")"

curl -fsS "${BASE}/v1/auth/me" -H "Authorization: Bearer ${ACCESS}"
```

---

## M. Smoke — create category / brand / tag / media / product

**OPERATOR CONFIRMATION REQUIRED**

Follow your standard admin API sequence (OpenAPI **`docs/swagger/swagger.json`** or Postman **`postman/collections/`** + **`postman/environments/`**). Minimum meaningful checks for **offline cache**:

1. Create **category**, **brand**, optional **tag**.
2. Init **media** upload (`/v1/admin/media/uploads/init` or legacy path per deployed OpenAPI), complete upload, confirm **`ready`**.
3. Create **product** with **`primaryMediaId`** when **`active`** / sellable per API rules.
4. **GET** product — confirm **`media.primary`** / variant URLs or IDs present as expected.

---

## N. Smoke — gRPC catalog media manifest

**OPERATOR CONFIRMATION REQUIRED**

From an operator host with **`grpcurl`** and TLS trust for the machine gRPC surface (see **`docs/runbooks/grpc-production.md`**):

```bash
grpcurl \
  -d '{"company_id":"<UUID>","machine_id":"<UUID>","catalog_version":"..."}' \
  your-grpc-host:443 \
  avf.machine.v1.MachineCatalogService/GetMediaManifest
```

Confirm **non-error** response and manifest entries consistent with sale catalog (exact fields depend on proto revision).

---

## O. Smoke — MQTT `catalog.refresh` ACK

**OPERATOR CONFIRMATION REQUIRED**

Verify broker connectivity using **`MQTT_BROKER_URL`** / credentials from `.env.app-node`. Payload shape should match **`docs/api/mqtt-contract.md`** and repo **`testdata/telemetry/valid_catalog_refresh_command_ack.json`** (pattern reference only).

Subscribe for command ACKs on the topic prefix configured for your fleet, publish a **`catalog.refresh`** command per contract, confirm **ACK** with expected correlation / status fields.

---

## P. Rollback notes

| Scenario | Action |
|----------|--------|
| **Bad application code after migrate** | Roll **images** via **`deployments/prod/app-node/scripts/rollback_app_node.sh`** (see **`deployments/prod/app-node/README.md`**). **Schema may remain at goose 4**. |
| **Bad migration / data corruption** | Restore from **`pg_dump`** (§ D–E) using provider/runbook **`docs/runbooks/production-backup-restore-dr.md`** / **`restore_managed_postgres.sh`** — **OPERATOR CONFIRMATION REQUIRED**. **`goose down`** is **not** automated in production rollback scripts. |
| **Partial migrate** | Capture **`docker compose ... run --rm migrate`** logs; avoid duplicate **`up`** until reviewed. |

---

## Verification SQL (reference)

These mirror **`docs/reports/product-media-offline-cache/local-migration-verification.md`** and **`scripts/db/verify_product_media_migration.sh`**.

### Goose revision **4** applied

```sql
SELECT version_id, is_applied
FROM goose_db_version
WHERE version_id >= 3
ORDER BY version_id;
```

Expect **`version_id = 4`** with **`is_applied = true`** when **`00004_product_media_offline_cache.sql`** is current.

### Media tables

```sql
SELECT table_name
FROM information_schema.tables
WHERE table_schema = 'public'
  AND table_name IN ('media_assets', 'media_variants')
ORDER BY 1;
```

### Product primary media column

```sql
SELECT column_name, data_type
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name = 'products'
  AND column_name = 'primary_image_id';
```

### `product_tags`

```sql
SELECT EXISTS (
  SELECT 1 FROM information_schema.tables
  WHERE table_schema = 'public' AND table_name = 'product_tags'
) AS product_tags_exists;
```

### `product_media.media_role`

```sql
SELECT COUNT(*) AS media_role_column_present
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name = 'product_media'
  AND column_name = 'media_role';
```

### Legacy org / tenant / scope naming (public)

```sql
SELECT COUNT(*) AS bad_tables FROM information_schema.tables
WHERE table_schema = 'public'
  AND (table_name ILIKE '%organization%' OR table_name ILIKE '%tenant%' OR table_name ILIKE '%scope%'
       OR table_name IN ('organizations', 'tenants'));

SELECT COUNT(*) AS bad_columns FROM information_schema.columns
WHERE table_schema = 'public'
  AND (column_name ILIKE '%organization_id%' OR column_name ILIKE '%tenant_id%' OR column_name ILIKE '%scope_id%' OR column_name ILIKE '%org_admin%');

SELECT COUNT(*) AS bad_indexes FROM pg_indexes
WHERE schemaname = 'public'
  AND (indexname ILIKE '%organization%' OR indexname ILIKE '%tenant%' OR indexname ILIKE '%scope%');
```

Expect **0** for each **`COUNT(*)`** (same gate as offline-cache verification).

### Active sellable products missing primary media (informational)

```sql
SELECT COUNT(*) AS active_products_missing_primary
FROM products p
WHERE p.active = true
  AND p.primary_image_id IS NULL;
```

Investigate any **non-zero** result before declaring catalog healthy.

---

## Related documents

- **`docs/runbooks/migration-safety.md`**
- **`docs/runbooks/backup-evidence-for-production-migrations.md`**
- **`deployments/prod/app-node/README.md`**
- **`docs/reports/product-media-offline-cache/server-migration-verification-template.md`**
