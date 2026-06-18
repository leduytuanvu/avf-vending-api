# Phase 9 — Local database migration verification

**Report path:** `docs/reports/product-media-offline-cache/local-migration-verification.md`  
**Scope:** Disposable local Postgres only — **no production or remote servers.**

---

## 1. Database URL conventions (repo)

| Source | Value / convention |
|--------|-------------------|
| **`.env` (repo)** | `DATABASE_URL=postgres://postgres:postgres@127.0.0.1:15432/avf_vending?sslmode=disable` |
| **Docker Compose** | `deployments/docker/docker-compose.yml` — Postgres maps host **`127.0.0.1:15432`** → container `5432` |
| **`TEST_DATABASE_URL`** | Used by integration tests (e.g. `internal/app/catalogadmin/*_integration_test.go`, `internal/modules/postgres/integration_test.go`). When unset, those tests **skip**. See root **`README.md`**. |

**This run:** disposable database name **`avf_vending_media_migration_verify`** on the same host/credentials as `.env`, changing only the **database path**.

**DSN used:**

```text
postgres://postgres:postgres@127.0.0.1:15432/avf_vending_media_migration_verify?sslmode=disable
```

---

## 2. Migration commands executed

```powershell
# Fresh disposable DB (PostgreSQL 16 in Docker container avf-postgres)
docker exec avf-postgres psql -U postgres -v ON_ERROR_STOP=1 -c "DROP DATABASE IF EXISTS avf_vending_media_migration_verify WITH (FORCE);"
docker exec avf-postgres psql -U postgres -v ON_ERROR_STOP=1 -c "CREATE DATABASE avf_vending_media_migration_verify;"
```

```powershell
cd /path/to/avf-vending-api
go run github.com/pressly/goose/v3/cmd/goose@v3.27.0 `
  -dir migrations postgres `
  "postgres://postgres:postgres@127.0.0.1:15432/avf_vending_media_migration_verify?sslmode=disable" up
```

**Goose CLI result:** **PASS** — versions applied:

- `00001_placeholder.sql`
- `00002_platform_schema.sql`
- `00003_seed_dev.sql`
- `00004_product_media_offline_cache.sql`

**Final goose message:** `successfully migrated database to version: 4`

---

## 3. Verification SQL (run against `avf_vending_media_migration_verify`)

### 3.1 `goose_db_version` — all expected revisions applied

```sql
SELECT version_id, is_applied
FROM goose_db_version
ORDER BY version_id;
```

**Observed output:**

```text
 version_id | is_applied 
------------+------------
          0 | t
          1 | t
          2 | t
          3 | t
          4 | t
```

**Verdict:** **PASS** — revisions **0–4** recorded applied (current migration set ends at **00004**).

---

### 3.2 `products` — primary media relationship

Catalog links primary imagery via **`products.primary_image_id`** (uuid, nullable), aligned with `product_media` / binding flows.

```sql
SELECT column_name, data_type
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name = 'products'
  AND column_name = 'primary_image_id';
```

**Observed:** `primary_image_id | uuid` → **PASS**

**Supporting projection / constraints (post-00004):**

```sql
SELECT tablename, indexname
FROM pg_indexes
WHERE schemaname = 'public'
  AND indexname IN (
    'ux_product_media_one_primary_per_product',
    'ix_products_active_missing_primary_image',
    'ix_product_media_product_role'
  )
ORDER BY tablename, indexname;
```

**Observed:** all three indexes present → **PASS**

---

### 3.3 Media tables

```sql
SELECT table_name
FROM information_schema.tables
WHERE table_schema = 'public'
  AND table_name ~ '^media'
ORDER BY 1;
```

**Observed:**

- `media_assets`
- `media_variants`

→ **PASS**

---

### 3.4 `product_tags`

```sql
SELECT EXISTS (
  SELECT 1
  FROM information_schema.tables
  WHERE table_schema = 'public'
    AND table_name = 'product_tags'
) AS product_tags_exists;
```

**Observed:** `t` → **PASS**

---

### 3.5 `product_media.media_role` (offline-cache migration)

```sql
SELECT column_name
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name = 'product_media'
ORDER BY ordinal_position;
```

**Observed:** includes **`media_role`** (among other columns) → **PASS**

---

### 3.6 Legacy org / scope / tenant identifiers (public schema)

**Tables:**

```sql
SELECT table_name
FROM information_schema.tables
WHERE table_schema = 'public'
  AND (
    table_name ILIKE '%organization%'
    OR table_name ILIKE '%tenant%'
    OR table_name ILIKE '%scope%'
    OR table_name IN ('organizations', 'tenants')
  )
ORDER BY 1;
```

**Columns:**

```sql
SELECT table_name, column_name
FROM information_schema.columns
WHERE table_schema = 'public'
  AND (
    column_name ILIKE '%organization_id%'
    OR column_name ILIKE '%tenant_id%'
    OR column_name ILIKE '%scope_id%'
    OR column_name ILIKE '%org_admin%'
  )
ORDER BY 1, 2;
```

**Indexes:**

```sql
SELECT tablename, indexname
FROM pg_indexes
WHERE schemaname = 'public'
  AND (
    indexname ILIKE '%organization%'
    OR indexname ILIKE '%tenant%'
    OR indexname ILIKE '%scope%'
  )
ORDER BY 1, 2;
```

**Observed:** **0 rows** for all three queries → **PASS**

*(Note: legitimate company-scoping columns such as `company_id` may exist elsewhere; this gate targets retired **org / tenant / scope** naming patterns listed above.)*

---

## 4. Focused integration tests (against disposable DB)

**Environment:**

```powershell
$env:TEST_DATABASE_URL = "postgres://postgres:postgres@127.0.0.1:15432/avf_vending_media_migration_verify?sslmode=disable"
```

**Command:**

```powershell
go test ./internal/app/catalogadmin/... -count=1 -v
```

**Results:**

| Test | Result |
|------|--------|
| `TestProductTags_CreateUpdateReplaceClear_OmitUnchanged` | **PASS** |
| `TestPrimaryMedia_phase2_catalog_rules` | **PASS** (all subtests) |
| `TestPrimaryMedia_phase3_manifest_bind_complete_validation` | **PASS** (all subtests) |

**Application fix (post–Phase 9 diagnosis):** `InitUpload` chose a `mediaID` and built canonical object-store keys from it, but **`MediaAdminInsertAsset` did not insert `id`**, so Postgres assigned a different UUID to `media_assets.id`. `CompleteUpload` / variant generation then failed validation (`objectstore: key does not match canonical media path`). **`MediaAdminInsertAsset` now includes `id`** (`db/queries/media_admin.sql` + `sqlc generate`), and **`internal/app/mediaadmin/service.go`** passes the same `mediaID` used for keys.

---

## 5. Full unit/integration sweep (`go test ./...`)

**Command (unset both URLs so postgres-heavy suites skip unless you intend to run them):**

```powershell
Remove-Item Env:TEST_DATABASE_URL -ErrorAction SilentlyContinue
Remove-Item Env:DATABASE_URL -ErrorAction SilentlyContinue
go test ./... -count=1
```

**Result:** **PASS** (exit code **0**).

**Note:** If `DATABASE_URL` is set in your shell (e.g. to a disposable or empty DB), `internal/modules/postgres` and related integration tests may **run and fail** for reasons unrelated to migrations. For the default “offline” sweep matching CI skip behavior, clear **`DATABASE_URL`** as above (see root **`README.md`**).

---

## 6. E2E orchestration (`run-all-local.sh`)

**Command:**

```bash
bash tests/e2e/run-all-local.sh --fresh-data
```

**Result:** **NOT RUN** — `bash` unavailable in this Windows automation environment (`execvpe(/bin/bash) failed`).

**Operator command** (Linux/macOS/Git Bash, deps up):

```bash
bash tests/e2e/run-all-local.sh --fresh-data
```

---

## 7. Deliverables summary

| Item | Value |
|------|--------|
| **DB used** | `avf_vending_media_migration_verify` on `127.0.0.1:15432` (user `postgres`, same as local compose / `.env` pattern) |
| **Migration commands** | See §2 |
| **Verification SQL** | See §3 |
| **Focused integration tests** | **PASS** — tags + primary-media (`phase2` / `phase3`) against disposable DB |
| **`go test ./... -count=1`** | **PASS** with **`DATABASE_URL`** and **`TEST_DATABASE_URL`** unset (see §5) |
| **E2E** | **NOT RUN** (no bash in this Windows automation environment) |

---

## 8. Final local migration verdict

**Schema & migration:** **PASS** — fresh DB reaches latest goose revision (**4**), **`products.primary_image_id`** present, **`media_assets` / `media_variants`** present, **`product_tags`** present, **`product_media.media_role`** + primary indexes present, legacy **org/scope/tenant** pattern scan **clean**.

**Runtime proof:** **PASS** for Phase 9 scope — **`internal/app/catalogadmin/...`** integration tests green on **`avf_vending_media_migration_verify`** after aligning **`media_assets.id`** with **`InitUpload`** keys; **`go test ./... -count=1`** green when integration DB env vars are unset.

**Overall Phase 9 verdict:** **MIGRATION_VERIFY_PASS — RUNTIME_TESTS_PASS** (E2E script still operator-dependent on bash + local stack).
