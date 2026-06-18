# Server migration verification — product media offline cache

**Report:** `docs/reports/product-media-offline-cache/server-migration-verification-20260519.md`  
**Date (UTC):** 2026-05-19  
**Migration:** goose **`00004_product_media_offline_cache`** (revision **4**)  
**Runbook:** `docs/runbooks/product-media-offline-cache-production-migration.md`

---

## Automation execution status — **STOPPED**

**Step failed / not executed:** **Task 1 — SSH**

**Exact reason:** No **SSH target** (`user@host` or equivalent) was supplied in the operator request, and this environment cannot authenticate to production or assume network path to the pilot server. **Per instruction: do not continue** remote execution without approved access.

**Required from operator to proceed on the server:**

1. SSH target(s), e.g. `ssh USER@HOST` (bastion/jump if applicable — document separately).
2. Confirmation that `/opt/avf-backups/db` exists on the host (or adjust backup volume path).
3. Completed **backup + verification** (§3–4 below) before migration — **if backup fails, stop entire plan.**

Copy this file and append outputs **only as redacted summaries** (no `.env` dumps, no tokens, no passwords).

---

## Preconditions (operator confirm)

| Precondition | Confirmed |
|--------------|-----------|
| Product media offline cache code merged and deployed images contain migration **00004** | ☐ |
| Operator confirms production pilot data may be migrated | ☐ |
| **Backup created before migration** | ☐ |

---

## Metadata

| Field | Value |
|------|--------|
| SSH target used | *(not run — provide when executing)* |
| Operator | |
| Backup artifact path | |
| Backup size (bytes) | |
| `pg_restore -l` line count | |
| Migration method used | *(workflow `run_migration=true` / compose `migrate` / manual goose)* |
| Started (UTC) | |
| Completed (UTC) | |

---

## 2. Server — directory, `DATABASE_URL`, `pg_isready`

**OPERATOR CONFIRMATION REQUIRED.** Do **not** paste `.env.app-node` contents.

```bash
cd /opt/avf-vending-api/deployments/prod/app-node

set -a
# shellcheck source=/dev/null
source ./.env.app-node
set +a

export APP_ENV="${APP_ENV:-production}"
bash /opt/avf-vending-api/scripts/verify_database_environment.sh
```

Redacted output (paste one-liner summary only):

```
(paste verify_database_environment output — must not include password)
```

`pg_isready` (uses `DATABASE_URL` already exported):

```bash
docker run --rm \
  -e DATABASE_URL \
  postgres:17-alpine \
  sh -lc 'pg_isready -d "$DATABASE_URL" -t 10'
```

Exit code / result:

```
(paste: e.g. accepting connections / exit 0)
```

---

## 3. Backup (before migration — **mandatory**)

**If this step fails: STOP — do not run migration.**

Ensure backup directory exists on host:

```bash
sudo mkdir -p /opt/avf-backups/db
sudo chown root:root /opt/avf-backups/db   # adjust ownership per site policy
```

From **`/opt/avf-vending-api/deployments/prod/app-node`** after exporting `DATABASE_URL` as above:

```bash
docker run --rm \
  -e DATABASE_URL="$DATABASE_URL" \
  -v /opt/avf-backups/db:/backups \
  postgres:17-alpine \
  sh -lc 'pg_dump "$DATABASE_URL" --format=custom --no-owner --no-acl --file="/backups/avf_before_product_media_$(date +%Y%m%d_%H%M%S).dump"'
```

Recorded backup path (filename only; **no connection string**):

```
/opt/avf-backups/db/avf_before_product_media_YYYYMMDD_HHMMSS.dump
```

---

## 4. Backup verification

```bash
BACKUP_FILE=/opt/avf-backups/db/avf_before_product_media_<TIMESTAMP>.dump

test -s "$BACKUP_FILE"
ls -la "$BACKUP_FILE"

docker run --rm \
  -v /opt/avf-backups/db:/backups:ro \
  postgres:17-alpine \
  sh -lc 'pg_restore -l "/backups/$(basename "'"$BACKUP_FILE"'")"' > /tmp/pg_restore_list.txt

wc -l /tmp/pg_restore_list.txt
```

| Check | Result |
|-------|--------|
| File size > 0 | |
| `pg_restore -l` succeeds | |
| TOC lines (`wc -l`) | |

---

## 5. Migration (repo-approved method)

Pick **one**; record which was used in Metadata.

### Preferred — GitHub production workflow

- Deploy workflow with **`run_migration=true`** and valid **`backup_evidence_id`** per **`docs/runbooks/backup-evidence-for-production-migrations.md`**.

Workflow run URL / evidence id (no secrets):

```
```

### Alternative — Compose migration service

From **`/opt/avf-vending-api/deployments/prod/app-node`**:

```bash
set -a && source ./.env.app-node && set +a
export APP_ENV="${APP_ENV:-production}"
bash /opt/avf-vending-api/scripts/verify_database_environment.sh

export CONFIRM_PRODUCTION_MIGRATION=true

docker compose --env-file .env.app-node -f docker-compose.app-node.yml \
  --profile migration run --rm migrate
```

Or integrated node release:

```bash
export CONFIRM_PRODUCTION_MIGRATION=true
export RUN_MIGRATION=1
bash scripts/release_app_node.sh "${APP_IMAGE_REF}" "${GOOSE_IMAGE_REF}"
```

Migration command output summary (trim secrets):

```
(paste last ~30 lines or goose success line only)
```

---

## 6. Verification

### 6.1 Schema (read-only script)

```bash
VERIFY_ENV_FILE=/opt/avf-vending-api/deployments/prod/app-node/.env.app-node \
  bash /opt/avf-vending-api/scripts/db/verify_product_media_migration.sh
```

Output:

```
(paste db_summary + PASS line only)
```

### 6.2 Manual SQL spot-check — `goose_db_version`

```bash
docker run --rm -e DATABASE_URL -i postgres:17-alpine \
  psql -v ON_ERROR_STOP=1 -d "$DATABASE_URL" -c \
  "SELECT version_id, is_applied FROM goose_db_version WHERE version_id >= 3 ORDER BY version_id;"
```

Output:

```
```

### 6.3 Containers healthy

```bash
cd /opt/avf-vending-api/deployments/prod/app-node
docker compose --env-file .env.app-node -f docker-compose.app-node.yml ps
bash scripts/healthcheck_app_node.sh
```

Result:

```
```

### 6.4 HTTP — `/health/live`, `/health/ready`, `/version`

Use public API base URL (**no secrets**):

```bash
curl -sS -o /dev/null -w "%{http_code}\n" "https://<API_DOMAIN>/health/live"
curl -sS -o /dev/null -w "%{http_code}\n" "https://<API_DOMAIN>/health/ready"
curl -sS -o /dev/null -w "%{http_code}\n" "https://<API_DOMAIN>/version"
```

| Endpoint | HTTP status |
|----------|-------------|
| `/health/live` | |
| `/health/ready` | |
| `/version` | |

### 6.5 Login

Record **PASS/FAIL** only; do not paste tokens.

| Check | Result |
|-------|--------|
| Login returns access token | |
| `/v1/auth/me` succeeds | |

### 6.6 REST smoke — category / brand / tag / media / product

| Step | Result |
|------|--------|
| Create category | |
| Create brand | |
| Create tag | |
| Media init + complete → ready | |
| Create/update product with primary media when active | |
| GET product includes media + tags as expected | |

### 6.7 gRPC — media manifest

| Check | Result |
|-------|--------|
| Catalog/media manifest RPC succeeds | |

### 6.8 MQTT — catalog refresh ACK

| Check | Result |
|-------|--------|
| Publish refresh | |
| ACK received | |

---

## 7. Final verdict

| Verdict | Notes |
|---------|--------|
| ☐ **PASS** — pilot migration complete | |
| ☐ **FAIL** — stop line (backup / migrate / verify): | |

---

## Failure log (if any)

**Exact error (redact secrets):**

```
(paste command + stderr excerpt only)
```
