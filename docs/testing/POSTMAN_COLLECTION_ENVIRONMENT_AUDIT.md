# Postman Collection & Environment Audit

Generated: 2026-05-20

## Artifacts

| File | Role |
|------|------|
| `postman/collections/avf-vending-api.postman_collection.json` | Primary REST collection (OpenAPI-generated) |
| `postman/collections/avf-vending-api-function-path.postman_collection.json` | Function-path variant |
| `postman/environments/avf-local.postman_environment.json` | Local template |
| `postman/environments/avf-staging.postman_environment.json` | Staging template |
| `postman/environments/avf-production.postman_environment.json` | Production template (no secrets) |
| `postman/suites/full-production-suite/AVF_REST_365_FULL.postman_collection.json` | Full parity suite |

## Regeneration

```bash
python tools/build_openapi.py
python tools/build_postman_collection.py
python tools/check_postman_artifacts.py
```

**Result: PASS** — `OK: Postman artifact checks`

## JSON validation

- All collection/environment files parse as valid JSON v2.1
- Request URLs use object form `{ raw, host, path, query }` — no empty-string URL roots
- Optional query params disabled by default where generator applies

## Collection scripts (auth + IDs)

Collection-level prerequest (`postman/scripts/collection_prerequest.js`):

- Sets `x_request_id`, `x_correlation_id`, `idempotency_key` (UUID v4 — idempotency strings, not resource PKs)
- Sets `resource_uuid` via **UUID v7** (`uuid7()`)
- Bearer injection from `accessToken` / `machineToken` based on folder auth type

Login folder tests should capture:

- `accessToken`, `refreshToken` from login response
- Created resource IDs (`categoryId`, `productId`, `machineId`, etc.) via test scripts

## Environment variables (local)

| Variable | Purpose | Default |
|----------|---------|---------|
| `baseUrl` | REST API root | `http://localhost:8080` |
| `adminEmail` | Bootstrap admin | empty — operator fills |
| `adminPassword` | Bootstrap admin | empty — operator fills |
| `accessToken` | Admin JWT | captured after login |
| `refreshToken` | Refresh token | captured after login |
| `machineToken` | Machine JWT | captured after activation claim |
| `machineId` | Target machine UUID | captured / seed |
| `idempotencyKey` | Write dedupe | auto per request |
| `resource_uuid` | Client-supplied PK | UUID v7 per request |
| `productId`, `categoryId`, `brandId`, `tagId`, `mediaId` | Catalog flow | captured |
| `siteId`, `regionId` | Fleet | captured |
| `order_id`, `paymentId` | Commerce | captured |

**Removed / forbidden:** `organization_id`, `tenant_id`, `scope_id` — validated absent by `check_postman_artifacts.py`

## OpenAPI parity

| Metric | Count |
|--------|------:|
| OpenAPI operations | 327 |
| Primary collection requests | matches OpenAPI generator output |

Full suite parity gate: `openapi_operations == postman_requests` (see `generate_full_postman_suite.py`).

## Gated writes

These operations require explicit canary/gate env vars before mutating production:

- `POST /v1/auth/logout`
- `DELETE /v1/auth/sessions`
- Refund / vend-failure endpoints

See `POSTMAN_VARIABLE_AUDIT_REPORT.md` for full gated-write list.

## Newman smoke (optional)

```bash
npm i -g newman
VERIFY_WITH_NEWMAN=1 bash scripts/local/verify-full-system.sh
```

Runs Health folder against local env — requires API up + valid tokens for protected folders.

## Drift note

`postman-drift` git check may fail until UUID v7 prerequest changes are committed — intentional updates in this pass.

## Verdict

**Postman: PASS** — valid JSON, import-safe URLs, artifact checks pass, UUID v7 for resource IDs, auth capture scripts present.
