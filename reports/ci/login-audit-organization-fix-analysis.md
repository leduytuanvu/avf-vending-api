# Login / audit organization-scope failure — analysis

## Observed symptom

`POST /v1/auth/login` returned **500** with:

`audit: ERROR: null value in column "organization_id" of relation "audit_events" violates not-null constraint (SQLSTATE 23502)`

Postman still sent a stale **`scopeId`** field; runtime **`LoginRequest`** only unmarshals `email` and `password` (no scope), and enterprise audit **`EnterpriseAuditInsertEvent`** omits any company column — so the failure is **purely database constraint vs. insert shape**.

## 1. Canonical schema vs. deployed schema

- **`db/schema/01_platform.sql`**: `audit_events` has **no** `scope_id`, `organization_id`, or `tenant_id` (matches sqlc models and `db/queries/enterprise_audit.sql`).
- **Goose history**: `migrations/00031_enterprise_audit_events.sql` created `audit_events` with **`scope_id uuid NOT NULL REFERENCES companies`** and scope-based indexes.
- **Production error naming**: Some deployments surface the legacy column as **`organization_id`** (rename or parallel migration outside this repo). The fix treats **both** names when relaxing constraints.

## 2. SQL / sqlc

- **`db/queries/enterprise_audit.sql`**: `INSERT INTO audit_events` lists only actor/resource/machine/site/metadata/outcome/etc. — **no** `organization_id` / `scope_id`.
- **`internal/gen/db/enterprise_audit.sql.go`**: Matches the SQL (no `OrganizationID` param). No manual edits required.

## 3. Application audit path

- **`internal/app/audit/service.go`**: `buildInsertParams` fills **`EnterpriseAuditInsertEventParams`** only — no org field.
- **`internal/app/auth/auth_audit.go`**: Login success/failure uses **`EnterpriseRecorder`** without company scope.

## 4. OpenAPI / Postman / generator

- **`tools/build_openapi.py`**: Login/me/admin-auth/account and enterprise audit schemas incorrectly documented **`scopeId`** on payloads that the Go handlers never return. Examples for **`POST /v1/auth/login`** included **`scopeId`** in the request body.
- **`split_swagger_params`** treated `@Param body body V1FooRequest` as **`type: string`** unless `typ == object`. Added **`V1*`** → **`$ref`** mapping and registered missing component schemas (**`V1AuthLoginRequest`**, **`V1AuthRefreshRequest`**, **`V1AuthLogoutRequest`**, **`V1AdminMediaUploadInitRequest`**) so login (and media upload init) request bodies are valid JSON objects in **`docs/swagger/swagger.json`**.
- **`internal/httpserver/openapi_types.go`** aligned documented auth/admin-auth/audit types with runtime JSON (removed **`scopeId`** where Go structs omit it).

## 5. Migration requirement for running databases

Any database that still has **`audit_events.scope_id`** or **`audit_events.organization_id`** defined **`NOT NULL`** will reject inserts that omit those columns until **`migrations/00075_audit_events_relax_company_scope.sql`** is applied.

- **Automatic / CI-safe**: drops legacy scope-only indexes with **`DROP INDEX IF EXISTS`**, then **`ALTER COLUMN … DROP NOT NULL`** inside **`DO $$ … $$`** blocks guarded by **`information_schema`** (no **`DROP COLUMN`** in goose `Up`, so **`DEPLOY_TARGET=ci`** migration verifier stays green).
- **Manual / destructive**: optional **`DROP COLUMN`** after verification — see **`docs/runbooks/manual-db-cleanup/README.md`**.

## Commands attempted (local)

- `CGO_ENABLED=0 go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate` — success; **`internal/gen/db`** unchanged vs. committed queries.
- `python tools/build_openapi.py` + `python tools/openapi_verify_release.py` — success.
- `python tools/build_postman_collection.py` + `python tools/check_postman_artifacts.py` — success.
- `DEPLOY_TARGET=ci bash scripts/ci/verify_migrations.sh` — success (**75** migration files, **0** destructive findings).
- `bash scripts/check_production_placeholders.sh` — **not run**: Git Bash environment lacked **`rg`** on PATH (Windows agent limitation); run in CI or install ripgrep locally.
