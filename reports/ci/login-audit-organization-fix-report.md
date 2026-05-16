# Login / audit organization-scope fix — report

## 1. Root cause

Deployed **`audit_events`** still enforced **`NOT NULL`** on a legacy company-scope column (**`scope_id`** from goose `00031`, or **`organization_id`** in some environments). Application code and sqlc **`INSERT`** omit that column entirely, so PostgreSQL attempted to store **NULL** and raised **23502**. This surfaced as **`audit: …`** because **`Login`** wraps **`auditLoginSuccess`** errors (**`internal/app/auth/service.go`**).

Stale **`scopeId`** in Postman/OpenAPI did **not** drive the SQL insert (ignored by **`LoginRequest`**), but it confused operators and contradicted the single-company contract.

## 2. Database fix

- **New migration**: **`migrations/00075_audit_events_relax_company_scope.sql`**
  - **`DROP INDEX IF EXISTS`** on legacy scope-prefixed **`audit_events`** indexes (`ix_audit_events_org_*`, `ix_audit_events_company_occurred_action`, etc.).
  - **`ALTER COLUMN … DROP NOT NULL`** for **`scope_id`** and **`organization_id`** when present (guarded PL/pgSQL blocks).
  - **Irreversible Down** (raises), consistent with other safety migrations.
- **Runbook**: **`docs/runbooks/manual-db-cleanup/README.md`** updated with optional manual **`DROP COLUMN`** follow-up after verification.

## 3. SQLc / audit runtime

- **`db/queries/enterprise_audit.sql`** unchanged — already correct.
- **`CGO_ENABLED=0`** **`sqlc generate`** — no drift in **`internal/gen/db`**.

## 4. OpenAPI / Postman / generator

- **`tools/build_openapi.py`**: Removed **`scopeId`** from auth account shapes, session items, enterprise audit event schema, and examples; login/password-reset bodies use **email/password** or **email** only.
- **`tools/build_openapi.py`**: **`split_swagger_params`** — **`V1*`** body types now emit **`$ref`** to **`#/components/schemas/…`**; added **`V1AuthLoginRequest`**, **`V1AuthRefreshRequest`**, **`V1AuthLogoutRequest`**, **`V1AdminMediaUploadInitRequest`** components (fixes incorrect **`type: string`** bodies for swag `body V1…` params).
- **`internal/httpserver/openapi_types.go`** + **`internal/httpserver/swagger_operations.go`**: Documentation aligned with handlers.
- Regenerated **`docs/swagger/swagger.json`**, **`docs/swagger/docs.go`**, and **`docs/postman/*.json`**.

## 5. Migration safety

```
DEPLOY_TARGET=ci bash scripts/ci/verify_migrations.sh --report ci-reports/migration-safety-report.json
```

Result: **`files_checked=75`**, **`destructive=0`**, **`blocked=False`**.

## 6. Tests and checks run

| Command | Result |
|--------|--------|
| `go vet ./...` | PASS |
| `go test ./internal/app/audit/... ./internal/app/auth/... ./internal/httpserver/... -run 'Audit\|Login\|Auth\|Me' -count=1` | PASS |
| `go test ./... -count=1` | PASS |
| `python tools/openapi_verify_release.py` | PASS |
| `python tools/check_postman_artifacts.py` | PASS |
| `bash scripts/check_production_placeholders.sh` | **Skipped locally** (no `rg` in Git Bash PATH on agent) — rely on CI |

## 7. Deploy / ops notes

1. **Apply goose migration `00075`** to staging/production **before** expecting login audit writes against legacy **`audit_events`** shape.
2. **Restart API** processes after deploy so all instances use current code + artifacts.
3. **Retest** `POST /v1/auth/login` with **`{"email":"…","password":"…"}`** only (no **`scopeId`**).
4. Optional maintenance: after retention/verification, execute manual cleanup to **`DROP COLUMN`** legacy scope columns per **`docs/runbooks/manual-db-cleanup/README.md`**.

## 8. Phase 8 (E2E) — blocker

Full **`tests/e2e/run-*-local.sh`** flows were **not** executed in this Windows agent session (bash scripts + Docker/Postgres fixtures not validated here). Run on a developer machine or CI job that already executes the e2e harness.

## 9. Phase 7 policy grep — scope

A repo-wide grep for **`scopeId`** still matches **catalog/fleet**-style schemas and legacy examples — **by design** until the broader OpenAPI/product surfaces drop **`scopeId`** everywhere. This change satisfies the **auth login + enterprise audit event + admin account/session documentation** slice and fixes the **audit_events NOT NULL** production failure path.
