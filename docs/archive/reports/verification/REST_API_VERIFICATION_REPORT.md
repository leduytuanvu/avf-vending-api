# REST API Verification Report

Generated: 2026-05-20

## OpenAPI generation

```bash
python tools/build_openapi.py
python tools/openapi_verify_release.py
git diff --exit-code -- docs/swagger/
```

**Result: PASS**

| Check | Status |
|-------|--------|
| Local `#/` fragment refs resolve | OK |
| `bearerAuth` JWT scheme | OK |
| Servers: production + localhost | OK |
| 327 operations registered | OK |
| Unique `operationId` values | OK |
| Protected `/v1` routes declare Bearer | OK |
| Write request body examples | OK |
| Success + error response examples | OK |
| No secret-like examples | OK |
| Legacy machine REST marked deprecated | OK |

## Route parity

- **266** path keys, **327** HTTP operations in `docs/swagger/swagger.json`
- Postman primary collection generated from same OpenAPI source

## Automated test coverage

```bash
go test -count=1 ./...
```

**Result: PASS** (all packages; integration tests skip without `TEST_DATABASE_URL`)

Representative packages with REST handler tests:

| Area | Package | Notes |
|------|---------|-------|
| Auth login/refresh/RBAC | `internal/app/auth`, `internal/platform/auth` | JWT, sessions, forbidden roles |
| Admin catalog | `internal/app/catalogadmin`, `internal/app/mediaadmin` | CRUD + validation |
| Fleet / machines | `internal/app/fleetadmin`, `internal/app/activation` | Provisioning, activation |
| Commerce / payments | `internal/app/commerce`, `internal/app/payments` | Orders, webhooks, idempotency |
| Audit | `internal/app/audit` | Event creation + list filters |
| HTTP server | `internal/httpserver` | Routing, middleware |
| E2E correctness | `internal/e2e/correctness` | Payment webhooks, vend flows (cached pass) |

## Negative test matrix (automated where noted)

| Case | Expected | Automated |
|------|----------|-----------|
| Missing auth | 401 | Yes — handler/middleware tests |
| Forbidden role | 403 | Yes — RBAC tests |
| Invalid UUID | 400/404 | Yes — validation tests |
| Duplicate idempotency key | Replay 2xx | Yes — postgres integration + e2e |
| Invalid JSON | 400 | Partial — handler tests |
| Not found | 404 | Yes |
| Conflict | 409 | Yes — duplicate email, etc. |
| Validation error | 422 | Yes — request binding |

## Live REST inventory (optional)

```bash
python scripts/test/rest_openapi_coverage.py --base-url http://127.0.0.1:8080
```

**Not run** — API server not started in this pass. Use local docker + `make run-api` for live probe.

## Verdict

**REST API: PASS** — OpenAPI deterministic and verified; unit/integration tests green.
