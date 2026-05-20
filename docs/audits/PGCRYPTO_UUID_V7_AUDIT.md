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

*(Updated after production deploy and verification.)*

- **Root cause:** Unqualified `gen_random_bytes` in `uuid_generate_v7()` with search_path excluding `extensions`.
- **Files changed:** `migrations/00005_uuid_v7_defaults.sql`, `migrations/00006_fix_pgcrypto_uuid_v7_schema_qualification.sql`, `db/schema/01_platform.sql`, `internal/migrations/pgcrypto_uuid_v7_test.go`, `scripts/checks/check-pgcrypto-schema-qualification.sh`, `Makefile`.
- **Migration added:** `00006_fix_pgcrypto_uuid_v7_schema_qualification.sql`
- **Tests run:** *(pending)*
- **Production backup path:** *(pending)*
- **Production deploy run:** *(pending)*
- **Production health result:** *(pending)*
- **Login result:** *(pending)*
- **Final verdict:** *(pending)*
