# Pgcrypto / UUID v7 production audit

**Date:** 2026-05-20  
**Incident:** `POST /v1/auth/login` → 500 — `function gen_random_bytes(integer) does not exist (SQLSTATE 42883)` during audit logging.

## gen_random_bytes occurrences

| Location | Qualified? | Notes |
|----------|------------|-------|
| `migrations/00005_uuid_v7_defaults.sql` (Up) | **Fixed** → `extensions.gen_random_bytes(10)` | Was unqualified; root cause |
| `migrations/00006_fix_pgcrypto_uuid_v7_schema_qualification.sql` | Yes | Forward migration for production |
| `db/schema/01_platform.sql` | **Fixed** → `extensions.gen_random_bytes(10)` | sqlc baseline mirror |

No other `gen_random_bytes(` calls in project SQL (excluding vendor).

## gen_random_uuid occurrences

| Location | Qualified? | Notes |
|----------|------------|-------|
| `migrations/00002_platform_schema.sql` | N/A (historical baseline) | 91 tables; converted to v7 by 00005 Up |
| `migrations/00005_uuid_v7_defaults.sql` (Down) | **Fixed** → `extensions.gen_random_uuid()` | Local/dev rollback only |
| `migrations/00004_product_media_offline_cache.sql` | N/A (historical) | Allowlisted baseline |

Application Go code does not call `gen_random_uuid` directly.

## Custom UUID v7 function

| Function | Schema | search_path | Random bytes |
|----------|--------|-------------|--------------|
| `public.uuid_generate_v7()` | public | `public, extensions, pg_temp` | `extensions.gen_random_bytes(10)` |

Defined in: `migrations/00005`, `migrations/00006`, `db/schema/01_platform.sql`.

## Audit path (audit_events)

No separate audit trigger or SQL helper. Login audit uses `EnterpriseAuditInsertEvent` (`db/queries/enterprise_audit.sql`) which `INSERT`s into `audit_events` omitting `id`. The column default is `public.uuid_generate_v7()`, which invoked unqualified `gen_random_bytes()` and failed on Supabase (pgcrypto in `extensions` schema).

## Root cause conclusion

**Confirmed:** `public.uuid_generate_v7()` used `SET search_path = public, pg_temp` and called `gen_random_bytes(10)` without schema qualification. On Supabase/production, `pgcrypto` is installed in the `extensions` schema, so the unqualified name resolves to nothing → SQLSTATE 42883 on every `audit_events` INSERT (including auth login audit).

## Fix applied

1. Baseline (`00005`, `db/schema/01_platform.sql`): `CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA extensions`; qualify `extensions.gen_random_bytes`; safe `search_path`.
2. Forward migration `00006_fix_pgcrypto_uuid_v7_schema_qualification.sql`: idempotent `CREATE OR REPLACE` + verification DO block for already-migrated production.
3. Tests: `internal/migrations/pgcrypto_uuid_v7_test.go`, `scripts/checks/check-pgcrypto-schema-qualification.sh` (wired into `make ci-gates`).

## Final fix

- **Root cause:** `public.uuid_generate_v7()` used `SET search_path = public, pg_temp` and unqualified `gen_random_bytes(10)`. Supabase installs `pgcrypto` in `extensions`, so audit `INSERT` (via `DEFAULT public.uuid_generate_v7()`) failed with SQLSTATE 42883. Successful login uses fail-closed `RecordCritical` audit → HTTP 500; failed login swallows audit errors → HTTP 401.
- **Files changed:** `migrations/00005_uuid_v7_defaults.sql`, `migrations/00006_fix_pgcrypto_uuid_v7_schema_qualification.sql`, `db/schema/01_platform.sql`, `internal/migrations/pgcrypto_uuid_v7_test.go`, `scripts/checks/check-pgcrypto-schema-qualification.sh`, `Makefile`, `docs/audits/PGCRYPTO_UUID_V7_AUDIT.md`
- **Migration added:** `00006_fix_pgcrypto_uuid_v7_schema_qualification.sql`
- **Tests run:** `go test ./... -short`, `go vet ./...`, `verify_migrations.py --deploy-target ci`, PR #250 CI (all pass)
- **Production backup path:** Not completed — SSH to `root@72.62.244.94` denied (`Permission denied (publickey,password)` from this environment)
- **Production deploy run:** Not completed — fix merged to **develop** (PR [#250](https://github.com/leduytuanvu/avf-vending-api/pull/250)); **main** merge blocked on PR [#251](https://github.com/leduytuanvu/avf-vending-api/pull/251) (requires approval from someone other than last pusher). Production deploy workflow not triggered (`run_migration=true` deploy pending main merge + Security Release + operator inputs).
- **Production health result:** `GET /health/live` → **200**
- **Login result:** Wrong credentials → **401** `invalid_credentials` (audit failure on failed login is non-fatal). Successful login **not verified** (no prod credentials; migration **00006 not deployed** yet — success path still expected to 500 until migrate).
- **Final verdict:** **BLOCKED** — code/migration fix ready on `develop`; production verification and deploy require PR #251 approval, production deploy with migration, and valid login credentials.

### Operator next steps

1. Approve and merge [PR #251](https://github.com/leduytuanvu/avf-vending-api/pull/251) (`develop` → `main`).
2. Wait for main CI + Security Release + Build and Push Images.
3. Run **Deploy Production** workflow with `run_migration=true`, digest-pinned images, and required evidence inputs.
4. SSH backup + verify: `SELECT extensions.gen_random_bytes(16); SELECT public.uuid_generate_v7();` and `goose_db_version` includes `00006`.
5. `POST /v1/auth/login` with valid prod credentials → expect **200** and no `gen_random_bytes` in API logs.
