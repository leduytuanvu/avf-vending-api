# UUID v7 Standardization Audit (Phase 0)

**Date:** 2026-05-20  
**Scope:** Read-only inventory of UUID usage across the AVF Vending API repository  
**Goal context:** Standardize **future internally generated resource UUIDs** to UUID v7 without rewriting production data or breaking APIs.

---

## 1. Executive summary

| Layer | Current state | v7 impact |
|-------|---------------|-----------|
| **PostgreSQL defaults** | ~**91** `DEFAULT gen_random_uuid()` PK columns (UUID **v4** via `pgcrypto`) | **Primary production ID path** for most inserts — must change defaults or app pre-assigns IDs |
| **Go explicit generation** | **`github.com/google/uuid` v1.6.0** — `uuid.New()` / `uuid.NewString()` (v4) in **~10 production files** | Replace resource-ID call sites with centralized v7 helper |
| **sqlc inserts** | Most `INSERT` queries **omit** `id` and rely on DB `RETURNING *` | Unaffected at query level until DB default or app ID assignment changes |
| **Existing production rows** | Mixed v4 (and dev seed fixed UUIDs) | **Do not rewrite** — remain valid UUIDs |
| **API / OpenAPI / gRPC** | `format: uuid` string fields — version-agnostic | **No wire-format change** required |
| **Secrets / tokens** | Opaque bytes, JWT JTI, idempotency strings | **Out of scope** — do not convert to v7 |

**Dominant finding:** The backend already generates most resource IDs in **PostgreSQL**, not in Go. A v7 rollout must address **DB column defaults** (new migration) **and** the small set of Go paths that pre-assign UUID PKs before insert.

---

## 2. Repository scan methodology

Commands run (Phase 0):

```bash
git status --short
git grep -nE "uuid|UUID|gen_random_uuid|uuid_generate|uuid_v7|NewV7|uuid\.New|uuid\.NewString|crypto/rand" -- .
```

Additional ripgrep passes on `*.go`, `migrations/*.sql`, `db/schema/*.sql`, `db/queries/*.sql`, Postman scripts, and swagger artifacts.

**Note:** Working tree contains unrelated uncommitted changes (repo cleanup, production auto-migration work). This audit describes **current codebase patterns**, not a clean `main` snapshot.

---

## 3. UUID library and dependencies

| Item | Value |
|------|--------|
| Go module | `github.com/google/uuid v1.6.0` (`go.mod`) |
| v7 support in library | **`uuid.NewV7()` available** in v1.6.0 (not used anywhere today) |
| goose / migrations | Root `migrations/*.sql` — source of truth for production schema |
| Parallel schema copy | `db/schema/01_platform.sql` mirrors platform DDL (keep in sync when migrating) |

No existing references to `uuid_generate_v7`, `pg_uuidv7`, `NewV7`, or `uuidv7` in the repository.

---

## 4. Database layer

### 4.1 Migration files

| File | UUID role |
|------|-----------|
| `migrations/00002_platform_schema.sql` | **~90** `id uuid PRIMARY KEY DEFAULT gen_random_uuid()` |
| `migrations/00004_product_media_offline_cache.sql` | **1** `media_variants.id` default `gen_random_uuid()` |
| `migrations/00003_seed_dev.sql` | **Fixed deterministic UUIDs** for local dev/integration — **do not change existing seed values** |

`gen_random_uuid()` (PostgreSQL `pgcrypto`) produces **RFC 4122 variant-1 random UUIDs (effectively v4)**. It is **not** time-ordered.

### 4.2 Tables using DB-generated UUID PKs (representative set)

All platform tables created in `00002_platform_schema.sql` with `DEFAULT gen_random_uuid()` on `id`, including but not limited to:

- **Fleet / topology:** `regions`, `sites`, `machines`, `machine_hardware_profiles`, `machine_credentials`, `machine_sessions`, `machine_lineage`, …
- **Auth / admin:** `platform_auth_accounts`, `auth_refresh_tokens`, `admin_sessions`, `password_reset_tokens`, `technicians`, …
- **Catalog / pricing:** `products`, `tags`, `media_assets`, `price_books`, `promotions`, `planograms`, …
- **Commerce:** `orders`, `payments`, `refunds`, `vend_sessions`, `payment_provider_events`, …
- **Device / ops:** `command_ledger`, `outbox_events`, `audit_events`, `ota_*`, `machine_activation_codes`, …
- **Enterprise:** `rollout_campaigns`, `feature_flags`, `machine_provisioning_batches`, …

Full `CREATE TABLE` list: **107 tables** in platform migrations (including composite-PK and projection tables).

### 4.3 Exceptions (no `gen_random_uuid` default on resource `id`)

| Pattern | Example | Notes |
|---------|---------|-------|
| **App-supplied PK, no default** | `product_media.id` | `id uuid PRIMARY KEY` — matches `product_images.id` (trigger/projection) |
| **Natural / FK PK** | `machine_shadow.machine_id`, `machine_current_snapshot.machine_id` | Not generated UUID resources |
| **Nullable correlation UUIDs** | `correlation_id uuid` on many tables | Often set from request/command context, not auto-generated PK |
| **Sentinel nil UUID** | `'00000000-0000-0000-0000-000000000000'::uuid` in reporting SQL | Filter “unset optional UUID” — **unchanged** |

### 4.4 sqlc query pattern

Typical insert (machines) **does not pass `id`** — DB default applies:

```sql
-- db/queries/fleet.sql — InsertMachine
INSERT INTO machines (site_id, hardware_profile_id, serial_number, ...)
VALUES ($1, $2, $3, ...)
RETURNING *;
```

**Implication:** Changing Go alone is insufficient; **ALTER DEFAULT** (or always passing app-generated v7 `id`) is required for these code paths.

### 4.5 Dev / test SQL

- `migrations/00003_seed_dev.sql` — explicit UUID literals (v4-looking fixed hex patterns).
- Integration tests occasionally embed `gen_random_uuid()` in raw SQL (e.g. enterprise retention test) — test-only.

---

## 5. Go application — explicit UUID generation

### 5.1 Production files using `uuid.New()` / `uuid.NewString()` (non-test)

| File | Usage | Classification |
|------|--------|----------------|
| `internal/app/auth/service.go` | `rtID`, `sessID` for `auth_refresh_tokens`, `admin_sessions` | **Convert → v7 resource ID** |
| `internal/app/auth/admin_users.go` | `tokID` for `password_reset_tokens.id` | **Convert → v7** (row PK; token *secret* is separate opaque hash) |
| `internal/app/mediaadmin/service.go` | `mediaID` before `MediaAdminInsertAsset` | **Convert → v7** |
| `internal/app/artifacts/service.go` | `ReserveArtifact()` returns `uuid.New()` | **Convert → v7** |
| `internal/app/planogram/service.go` | `"planogram-" + uuid.NewString()` idempotency fallback | **Exclude** — idempotency string, not resource PK |
| `internal/app/activation/service.go` | `refreshJTI := uuid.NewString()` for machine runtime refresh ledger | **Exclude** — JWT/JTI-style identifier |
| `internal/platform/auth/token_issuer.go` | JWT `Jti: uuid.NewString()` | **Exclude** — token claim |
| `internal/middleware/requestid.go` | `X-Request-ID` when absent | **Exclude** — HTTP correlation (optional: v7 OK but not a DB resource ID) |
| `internal/grpcserver/interceptors.go` | gRPC request ID fallback | **Exclude** — same as request ID |
| `internal/httpserver/admin_machine_diagnostics_http.go` | `request_id` in MQTT diagnostic payload | **Exclude** — ephemeral command correlation |
| `tools/loadtest/*.go` | Load-test idempotency prefixes | **Exclude** — test tooling |

**Observation:** `internal/modules/postgres/*.go` (production repositories) has **no** `uuid.New()` — repositories depend on DB defaults or caller-supplied IDs.

### 5.2 Opaque secrets (not UUID resource IDs)

These use **`crypto/rand`**, not UUIDs — **do not convert**:

| File | Mechanism |
|------|-----------|
| `internal/platform/auth/refresh_tokens.go` | 32-byte random → base64url refresh token |
| `internal/app/activation/service.go` | `randomActivationCode()` — activation **codes** (hashed at rest) |
| `internal/platform/auth/mfa_encrypt.go` | Encryption randomness |
| `internal/platform/redis/locks.go` | Lock token randomness |

### 5.3 UUID as type only (parse, validate, nil sentinel)

**~200+ Go files** import `github.com/google/uuid` for:

- Domain/API DTOs (`uuid.UUID` fields)
- `uuid.Parse` / `uuid.MustParse` on path params and JWT subjects
- `uuid.Nil` company sentinel (legacy singleton org)
- Test fixtures (`internal/testfixtures/ids.go` ↔ `00003_seed_dev.sql`)

No generation change needed for parse/validate paths.

---

## 6. Test, fixtures, and tooling

| Area | Pattern | v7 policy |
|------|---------|-----------|
| `internal/testfixtures/ids.go` | Fixed UUIDs from seed migration | **Keep** — deterministic tests |
| `migrations/00003_seed_dev.sql` | Fixed insert UUIDs | **Keep** — never rewrite production/dev seed |
| Integration / e2e tests | `uuid.New()` for ephemeral sites/machines/orders | Optional align to v7 helper for consistency; not production |
| `postman/scripts/collection_prerequest.js` | Client-side **UUID v4** generator for Postman variables | **Exclude** — client convenience; API accepts any UUID version |
| `deployments/prod/scripts/telemetry_storm_load_test.sh` | SHA256-derived deterministic UUID shape for load test | **Exclude** — synthetic load identities |

---

## 7. API, OpenAPI, gRPC, Postman

| Surface | UUID handling |
|---------|----------------|
| **OpenAPI / Swagger** (`docs/swagger/swagger.json`) | Many fields `"format": "uuid"` — **version-agnostic** |
| **HTTP handlers** | Path/query UUIDs validated via parsing — any RFC 4122 UUID accepted |
| **gRPC / proto** | Resource IDs as `string` UUID fields — no version field |
| **Postman collections** | Example UUIDs in bodies — documentation only |

**Conclusion:** Standardizing internal generation to v7 **does not require** OpenAPI or Postman schema changes. Existing v4 production IDs remain valid in all APIs.

---

## 8. Classification matrix (for Phase 1+)

| Category | Examples | Action |
|----------|----------|--------|
| **A — Internal resource PK / FK target** | `machines.id`, `orders.id`, `media_assets.id`, `auth_refresh_tokens.id` | **Future inserts → v7** |
| **B — DB default only (no Go ID)** | Most sqlc `INSERT … RETURNING *` | **Change PG default or require app ID** |
| **C — Go pre-assigned PK** | auth sessions, media upload init, artifacts reserve | **Central v7 helper** |
| **D — Correlation / request tracing** | `X-Request-ID`, gRPC request id, diagnostic `request_id` | **Exclude** (or optionally v7 for sortability — not required) |
| **E — JWT / session token identifiers** | JWT `jti`, machine refresh JTI | **Exclude** |
| **F — Idempotency / dedupe strings** | `Idempotency-Key`, `"order-"+uuid`, MQTT dedupe keys | **Exclude** — often client- or prefix-derived |
| **G — External provider IDs** | `payment_provider_events.provider_reference`, webhook event IDs in tests | **Exclude** — store as-is even if UUID-shaped |
| **H — Secrets** | refresh token raw, activation code plaintext, API keys | **Exclude** — not UUIDs |
| **I — Existing production + seed data** | All rows already inserted | **Never rewrite** |

---

## 9. Risks and constraints

1. **Supabase / managed Postgres:** `gen_random_uuid()` cannot emit v7. Need either:
   - **`pg_uuidv7`** (or equivalent) extension + `uuid_generate_v7()` as new `DEFAULT`, or
   - **Application-assigned v7** on every insert that today omits `id`.
2. **91+ column default changes:** Requires a **new goose migration** (`ALTER COLUMN … SET DEFAULT`). Safe for existing rows; affects only inserts that omit `id`.
3. **Mixed versions in one table:** Existing v4 rows + new v7 rows is **valid** and common during transition; queries must not assume version.
4. **Time-ordering expectations:** v7 enables better B-tree locality for PK indexes — benefit is forward-looking only.
5. **product_media / product_images:** IDs often coupled — any change must preserve trigger/projection invariants.
6. **Do not reset DB** or backfill UUIDs in production deploy paths (aligns with production migration policy).
7. **`db/schema/01_platform.sql`:** Must stay aligned with `migrations/` when defaults change (sqlc / local dev).

---

## 10. Proposed implementation plan (Phase 1+ — not executed in Phase 0)

### Phase 1 — Central Go generator

1. Add `internal/platform/uuidgen` (or `pkg/id`) with:
   - `NewResourceID() (uuid.UUID, error)` → wraps `uuid.NewV7()`
   - `MustNewResourceID()` for must-not-fail paths
   - Unit test: version nibble == 7, monotonic non-decreasing in tight loop
2. Replace category **C** call sites (auth, mediaadmin, artifacts).
3. **Do not** replace category D/E/F/G.

### Phase 2 — Database forward defaults

1. Verify Supabase production supports `pg_uuidv7` or chosen v7 SQL function.
2. Add goose migration `00005_uuid_v7_defaults.sql`:
   - `CREATE EXTENSION IF NOT EXISTS pg_uuidv7` (if available), or document app-only strategy
   - For each table with `DEFAULT gen_random_uuid()` on `id`:  
     `ALTER TABLE … ALTER COLUMN id SET DEFAULT uuid_generate_v7();`  
     (**No** `UPDATE` of existing rows)
3. Mirror change in `db/schema/01_platform.sql` for new environments.
4. New tables only in future migrations should use v7 default from the start.

### Phase 3 — Optional insert hardening

1. Audit sqlc queries that might pass explicit v4 IDs from tests into production code paths (none found in prod repos).
2. Consider CI grep/lint: ban `uuid.New(` outside `uuidgen` package and `_test.go` files.

### Phase 4 — Validation

1. `go test ./...`
2. Integration test: insert row without `id`, assert version 7 via `(id::text)[15]` or Go parse
3. Confirm OpenAPI/Postman smoke unchanged
4. Document operator note: existing IDs unchanged; new IDs are v7

---

## 11. Files likely to change (Phase 1+)

| Path | Reason |
|------|--------|
| **New** `internal/platform/uuidgen/uuidgen.go` | Central v7 API |
| `internal/app/auth/service.go` | Refresh/session row IDs |
| `internal/app/auth/admin_users.go` | Password reset token row ID |
| `internal/app/mediaadmin/service.go` | Media asset PK pre-insert |
| `internal/app/artifacts/service.go` | Artifact reservation ID |
| **New** `migrations/00005_*_uuid_v7_defaults.sql` | ALTER DEFAULT on PK columns |
| `db/schema/01_platform.sql` | Dev schema parity |
| **Optional** `scripts/ci/check_uuid_generation.sh` | Ban raw `uuid.New` in prod code |
| **New** `docs/architecture/uuid-v7-policy.md` | Operator/dev policy |

**Explicitly not in scope:** `migrations/00003_seed_dev.sql`, production data backfill, JWT/request-id middleware, refresh token entropy, activation codes, payment provider external IDs.

---

## 12. Git status snapshot (Phase 0)

`git status --short` shows a **large dirty working tree** (docs/postman/scripts reorg, production auto-migration files, etc.). UUID audit did **not** modify any tracked files except this document.

---

## 13. Acceptance criteria for Phase 0

- [x] Full-repo UUID grep inventory
- [x] DB vs Go generation paths identified
- [x] Convert vs exclude classification
- [x] Production no-rewrite constraint documented
- [x] Proposed phased plan and file list
- [x] No application logic changes in Phase 0

---

## Appendix A — Production Go `uuid.New` / `uuid.NewString` call sites (quick reference)

```
internal/app/auth/service.go          — rtID, sessID (resource PK)
internal/app/auth/admin_users.go      — tokID (password_reset_tokens.id)
internal/app/mediaadmin/service.go    — mediaID (media_assets.id)
internal/app/artifacts/service.go     — ReserveArtifact()
internal/app/planogram/service.go     — idempotency string (EXCLUDE)
internal/app/activation/service.go    — refresh JTI strings (EXCLUDE)
internal/platform/auth/token_issuer.go — JWT jti (EXCLUDE)
internal/middleware/requestid.go      — X-Request-ID (EXCLUDE)
internal/grpcserver/interceptors.go   — gRPC request id (EXCLUDE)
internal/httpserver/admin_machine_diagnostics_http.go — payload request_id (EXCLUDE)
tools/loadtest/*.go                   — load test (EXCLUDE)
```

## Appendix B — `gen_random_uuid()` counts

| Location | Count |
|----------|------:|
| `migrations/00002_platform_schema.sql` | ~90 |
| `migrations/00004_product_media_offline_cache.sql` | 1 |
| `db/schema/01_platform.sql` | ~91 (mirror) |

**Total PK defaults to migrate for forward inserts:** ~91 columns across ~80+ tables.

---

## Phase 3 completion (2026-05-20)

**Migration:** `migrations/00005_uuid_v7_defaults.sql`

- Adds `public.uuid_generate_v7()` (PL/pgSQL, `pgcrypto` `gen_random_bytes`, RFC 9562 v7).
- Alters **91** `public.*.id` columns from `gen_random_uuid()` → `public.uuid_generate_v7()` via catalog-driven `DO` block.
- **No** `UPDATE` of existing rows; seed literals in `00003_seed_dev.sql` unchanged.
- **Down:** restores `gen_random_uuid()` defaults and drops `uuid_generate_v7()` (local/dev only).
- **Mirror:** `db/schema/01_platform.sql` updated for sqlc.
- **Test:** `TestUUIDV7DefaultOnInsert` in `internal/modules/postgres/integration_test.go` (requires `TEST_DATABASE_URL`).

---

## Phase 5 completion (2026-05-20)

**Gate:** `scripts/checks/check-uuid-v7.sh` (also `make check-uuid-v7`, wired in `.github/workflows/ci.yml`).

- Fails on forbidden `uuid.New*` / v4 helpers in non-test Go production paths.
- Fails on `DEFAULT gen_random_uuid()` / `uuid_generate_v4()` in goose Up sections and `db/schema/`.
- Allowlist: `scripts/checks/uuid-v7-allowlist.txt`; inline `uuid-v7-allow` comments supported.
- Policy doc: `docs/architecture/UUID_V7_POLICY.md`.

**Tests & fixtures**

- `internal/testfixtures/uuid_v7_assert.go` — shared `AssertResourceUUIDV7` (v7, non-nil, parseable).
- Resource ID tests updated in `auth`, `mediaadmin`, `artifacts`; DB default test strengthened in `integration_test.go`.
- `enterprise_retention_integration_test.go` — aggregate_id uses Go v7 instead of SQL `gen_random_uuid()`.

**Postman**

- `postman/scripts/collection_prerequest.js` — `uuid7()` + `resource_uuid` collection variable; request/correlation/idempotency remain v4-shaped.
- `generate_full_postman_suite.py` — body field `id` / `artifactId` and matching OpenAPI examples use `{{resource_uuid}}`; generic UUID validation unchanged.
- Regenerate: `python postman/suites/full-production-suite/generate_full_postman_suite.py` and `python tools/build_postman_collection.py`.

**Unchanged (by design)**

- Seed fixture UUIDs (`00003_seed_dev.sql`, `testfixtures/ids.go`).
- OpenAPI `format: uuid` accepts any RFC 4122 version.
- Idempotency keys, JWT jti, webhook/provider IDs, canary name/code suffixes still use `{{$guid}}` or opaque strings.
