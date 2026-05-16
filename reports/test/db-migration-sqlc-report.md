# Database migrations & SQLC — verification report

This report covers **schema sources**, **SQLC regeneration**, **Go tests** touching migrations/postgres/gen/db, and **intended live SQL inspection**. A full “migrate from zero + catalog queries” run against Postgres **did not complete here** because admin authentication to the local server failed (see §1).

---

## 1. Fresh database & migrations (live) — **blocked**

### Intended procedure

1. Start Postgres with credentials you control (for example Docker Compose core profile):

   ```powershell
   docker compose -f deployments/docker/docker-compose.yml up -d postgres
   ```

   Compose maps host **`127.0.0.1:15432` → container `5432`** (see `deployments/docker/docker-compose.yml`). Align `DATABASE_URL` / admin URLs with that mapping.

2. Create a **throwaway** database (example name only):

   ```sql
   DROP DATABASE IF EXISTS avf_migration_verify WITH (FORCE);
   CREATE DATABASE avf_migration_verify;
   ```

3. Apply migrations from repo root:

   ```powershell
   go run github.com/pressly/goose/v3/cmd/goose@v3.27.0 `
     -dir migrations postgres "<POSTGRES_DSN_FOR_THROWAWAY_DB>" up
   ```

### What ran on this workstation

| Step | Result |
|------|--------|
| Connect as `postgres` / default URL from `.env.example` | **FAIL** — `FATAL: password authentication failed for user "postgres"` (`SQLSTATE 28P01`) |
| Docker Desktop engine | **Unavailable** in the prior session (`dockerDesktopLinuxEngine` pipe missing); not retried in this pass |

**Conclusion:** No migration-from-zero or attached SQL catalog output could be collected against a live cluster without valid **`DATABASE_ADMIN_URL`** / **`DATABASE_URL`** secrets.

### Recommended next command (operator)

```powershell
$env:DATABASE_ADMIN_URL = "postgres://USER:PASS@127.0.0.1:PORT/postgres?sslmode=disable"
# Then rerun goose `up` against a dedicated DB DSN and execute §5 queries via psql or any SQL client.
```

---

## 2. Static source audit (no live DB required)

Searched **`db/schema`**, **`migrations`**, **`db/queries`**, and generated **`internal/gen/db`** for leftover multi-tenant table/column naming:

```powershell
git grep -n -I -E 'organizations\b|organization_id|tenant_id|\btenant\b' `
  -- db/schema migrations db/queries internal/gen/db ':!vendor' ':!.git'
```

**Result:** **no matches** (`git grep` exit code **1**).

Additional spot checks:

| Area | `organizations` / `organization_id` / `tenant_id` (literal substrings) |
|------|----------------------------------------------------------------------|
| `db/schema/*.sql` | **none** |
| `migrations/*.sql` | **none** |
| `db/queries/*.sql` | **none** |
| `internal/gen/db/models.go` & peers | **none** |

---

## 3. SQLC regeneration

```powershell
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0 generate
```

**Exit code:** **0**

---

## 4. Go tests

```powershell
go test ./internal/modules/postgres/... -count=1
go test ./internal/migrations/... -count=1
go test ./internal/gen/db/... -count=1
```

| Package | Result | Notes |
|---------|--------|-------|
| `./internal/modules/postgres/...` | **PASS** | Fast path: **pure/unit tests execute**; **integration tests `SKIP`** unless `TEST_DATABASE_URL` is set (see `integration_test.go::testDSN`). |
| `./internal/migrations/...` | **PASS** | File-level migration safety tests only (no DB). |
| `./internal/gen/db/...` | **PASS** | `[no test files]` — compilation smoke via `-run` filter not needed; package builds as dependency of other tests. |

**Verbose sanity (`internal/modules/postgres`, excerpt):** many tests log `--- SKIP: …` when `TEST_DATABASE_URL` is unset; representative examples include schema-critical integration tests gated behind `testPool`.

**Stronger guarantee:** re-run with Postgres available:

```powershell
$env:TEST_DATABASE_URL = "postgres://USER:PASS@127.0.0.1:PORT/avf_vending_test?sslmode=disable"
go test ./internal/modules/postgres/... -count=1 -v
```

---

## 5. SQL inspection checklist (run after successful `goose up`)

Execute against the migrated database (throwaway DB recommended).

### 5.1 Tables

```sql
SELECT tablename
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY tablename;
```

**Expect:** **no** `organizations` table.

```sql
SELECT COUNT(*) AS organizations_table_present
FROM pg_tables
WHERE schemaname = 'public' AND tablename = 'organizations';
```

**Expect:** `0`.

### 5.2 Columns

```sql
SELECT table_name, column_name, data_type
FROM information_schema.columns
WHERE table_schema = 'public'
  AND (
    column_name ILIKE '%organization%'
    OR column_name ILIKE '%tenant%'
    OR column_name ILIKE '%org\_%' ESCAPE '\'
    OR column_name ILIKE '%\_org' ESCAPE '\'
  )
ORDER BY table_name, column_name;
```

**Expect:** **no rows** (policy: no `organization_id`, `tenant_id`, etc.).

### 5.3 Foreign keys to `organizations`

```sql
SELECT src.relname AS src_table, c.conname
FROM pg_constraint c
JOIN pg_class src ON src.oid = c.conrelid
JOIN pg_class tgt ON tgt.oid = c.confrelid
JOIN pg_namespace n ON n.oid = src.relnamespace
WHERE n.nspname = 'public'
  AND c.contype = 'f'
  AND tgt.relname = 'organizations'
ORDER BY src.relname;
```

**Expect:** **no rows**.

### 5.4 Constraint names

```sql
SELECT rel.relname AS table_name, c.conname
FROM pg_constraint c
JOIN pg_class rel ON rel.oid = c.conrelid
JOIN pg_namespace n ON n.oid = rel.relnamespace
WHERE n.nspname = 'public'
  AND (
    c.conname ILIKE '%organization%'
    OR c.conname ILIKE '%tenant%'
    OR c.conname ILIKE '%org_admin%'
    OR c.conname ILIKE '%org\_%' ESCAPE '\'
  )
ORDER BY rel.relname, c.conname;
```

**Expect:** **no rows**.

### 5.5 Index names

```sql
SELECT tab.relname AS table_name, i.relname AS index_name
FROM pg_index ix
JOIN pg_class i ON i.oid = ix.indexrelid
JOIN pg_class tab ON tab.oid = ix.indrelid
JOIN pg_namespace n ON n.oid = tab.relnamespace
WHERE n.nspname = 'public'
  AND (
    i.relname ILIKE '%organization%'
    OR i.relname ILIKE '%tenant%'
    OR i.relname ILIKE '%org_admin%'
  )
ORDER BY tab.relname, i.relname;
```

**Expect:** **no rows**.

---

## 6. Remaining DB-layer hits

| Scope | Status |
|-------|--------|
| Schema/migrations/queries sources | **Clean** (§2) |
| Generated Go (`internal/gen/db`) | **Clean** (substring scan) |
| Live catalog (§5) | **Not executed** — blocked by §1 |

---

## 7. Summary for auditors

- **SQLC + compile-level tests:** green on this machine.
- **Physical schema proof:** requires working Postgres + credentials; follow §1 then §5.
- **Source-of-truth drift:** none detected via repository grep for the targeted identifiers.
