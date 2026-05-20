# Database Migration Verification

Generated: 2026-05-20

## Commands

### Offline layout + safety

```bash
bash scripts/ci/verify_migrations.sh
```

**Result: PASS** — 5 files, 0 destructive findings, `migration-evidence/migration-safety-report.json`

### Fresh database (zero → latest)

```bash
# Create empty DB (Docker postgres on 15432)
docker exec avf-postgres psql -U postgres -c "DROP DATABASE IF EXISTS avf_vending_test_full_verify;"
docker exec avf-postgres psql -U postgres -c "CREATE DATABASE avf_vending_test_full_verify;"

MIGRATIONS_DIR=migrations \
DATABASE_URL="postgres://postgres:postgres@127.0.0.1:15432/avf_vending_test_full_verify?sslmode=disable" \
  go run ./cmd/migrate up

MIGRATIONS_DIR=migrations DATABASE_URL=... go run ./cmd/migrate version
```

**Result: PASS**

| Step | Version | Duration |
|------|---------|----------|
| 00001_placeholder.sql | 1 | 4ms |
| 00002_platform_schema.sql | 2 | ~2s |
| 00003_seed_dev.sql | 3 | ~50ms |
| 00004_product_media_offline_cache.sql | 4 | ~60ms |
| 00005_uuid_v7_defaults.sql | 5 | ~83ms |

Final version: **5**

### UUID v7 default verification

```sql
INSERT INTO regions (name, code) VALUES ('v7-check', 'v7-check') RETURNING id,
  substring(id::text FROM 15 FOR 1) AS version_nibble;
-- version_nibble = 7
```

**Result: PASS**

### Migrate validate (embedded runner contract)

```bash
MIGRATIONS_DIR=migrations go run ./cmd/migrate validate
# OK: 5 migration file(s) in migrations
```

**Result: PASS**

## goose_db_version state

Table: `goose_db_version` (goose v3). After fresh migrate: single row version **5**, `is_applied=true`.

## Schema ↔ code alignment

| Check | Status |
|-------|--------|
| sqlc codegen vs `db/schema/` | **PASS** (`go run sqlc generate && git diff --exit-code internal/gen/db/`) |
| `audit_events` — no legacy `organization_id`/`scope_id` required by current Go | **PASS** (single-company model; audit uses `actor_id`, `machine_id`, `resource_type`) |
| Seed bootstrap admin (`00003_seed_dev.sql`) | **PASS** — 50 `platform_auth_accounts` after migrate |
| Critical indexes (`TestSchemaCriticalIndexes`) | **PASS** with `TEST_DATABASE_URL` |

## Idempotency

Re-running `go run ./cmd/migrate up` on version 5:

```
goose: no migrations to run. current version: 5
```

**Result: PASS**

## App startup against fresh DB

Not run as long-running server in this pass. Integration tests against migrated DB (`internal/modules/postgres`, `internal/migrations`) **PASS**.

## Documented migration commands

| Environment | Command |
|-------------|---------|
| Local (goose CLI) | `make migrate-up` (requires `DATABASE_URL`) |
| Local (embed runner) | `MIGRATIONS_DIR=migrations DATABASE_URL=... go run ./cmd/migrate up` |
| Docker dev reset | `make dev-reset-db` / `make dev-migrate` |
| Production | `scripts/deploy/production-migrate.sh` (backup + up from image) |

## Issues found and fixed

**00005 goose StatementBegin** — migration failed on fresh DB with `unterminated dollar-quoted string`. Fixed by wrapping plpgsql blocks.

## Verdict

**Database migrations: PASS** on fresh local Postgres after fix.
