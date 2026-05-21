# Product Admin To App Flow Production Pass Report

## Run metadata

| Field | Value |
|-------|--------|
| Date/time UTC | 2026-05-20T12:00:00Z (session run) |
| Base URL | `https://api.ldtv.dev` |
| Git branch | `fix/product-flow-postman-production-suite` |
| Branch SHA | `7dfb78ae8e4b844138f1bff40e7621aaa1d004e7` (+ Postman patches) |
| develop SHA | `204dab6faec27cebbe6fe152a3d961cb77be5629` |
| Production API git_sha (from /version) | `52a076e340a15a69dad7787cad54d7e3000fcafe` |
| Collection tested | `avf-product-admin-to-app-flow.postman_collection.json` |
| Full suite | `avf-production-full.postman_collection.json` |
| Environment tested | `avf-production.postman_environment.json` |
| Operator notes | No `ADMIN_PASSWORD` in CI/agent shell; authenticated canary blocked |

## Verdict

**PARTIAL**

Public health/version and Newman Health folder **PASS**. Authenticated product flow (login → create → assign → catalog) **NOT RUN** in this session because production admin credentials were not available in the environment. Postman contract fixes validated statically and align with backend DTO source.

## Step-by-step evidence

### 1. Health live

- **GET** `/health/live`
- Headers: none required
- Body: none
- **HTTP 200**
- Response body: `ok`
- Variables saved: none

### 2. Health ready

- **GET** `/health/ready`
- **HTTP 200**
- Response body: `ok`

### 3. Version

- **GET** `/version`
- **HTTP 200**
- Response (sanitized):

```json
{
  "name": "avf-vending-api",
  "version": "v1.0.01",
  "git_sha": "52a076e340a15a69dad7787cad54d7e3000fcafe",
  "app_env": "production",
  "public_base_url": "https://api.ldtv.dev"
}
```

### 4. Login

- **NOT RUN** — `ADMIN_PASSWORD` / `AVF_ADMIN_PASSWORD` not set locally
- Expected: **POST** `/v1/auth/login` → 200, save `accessToken` (redacted in reports)

### 5. Category

- **NOT RUN** (blocked by login)

### 6. Brand

- **NOT RUN** (blocked by login)

### 7. Tag

- **NOT RUN** (blocked by login)
- Backend supports tags: `POST /v1/admin/tags` (`V1AdminTagMutationRequest` in `internal/httpserver/openapi_types.go`)

### 8. Product image upload

- **NOT RUN** (blocked by login)
- Prior operator evidence: **201** with Cloudinary URLs (per task brief)
- Postman fix confirmed: multipart form-data, no manual Content-Type, saves `primaryMediaId`

### 9. Product create

- **NOT RUN** (blocked by login)
- Postman fix confirmed: pre-request `JSON.stringify` body with camelCase DTO fields only
- Root cause of prior `invalid_json`: strict decode + invalid templated JSON / unknown fields

### 10. Product read-back

- **NOT RUN**

### 11. Price

- **NOT RUN**
- Implemented flow: price book create → PUT items → assign-target to machine

### 12. Machine assignment

- **NOT RUN**
- Implemented flow: planogram draft → planogram publish (`internal/httpserver/admin_catalog_mutations_http.go`, planogram handlers)

### 13. App/machine catalog read

- **NOT RUN**
- REST: `GET /v1/machines/{machineId}/sale-catalog`
- gRPC: `MachineCatalogService.GetSaleCatalog` / `SyncSaleCatalog` (`proto/avf/machine/v1/catalog.proto`)

### 14. gRPC

- **NOT RUN** — requires machine JWT; documented in test guide

### 15. MQTT

- **NOT RUN** — publish side-effect of planogram publish; documented in test guide

## Important IDs

| ID | Value this run |
|----|----------------|
| categoryId | — |
| brandId | — |
| tagId | — |
| primaryMediaId | — |
| productId | — |
| machineId | operator must set `<set-in-postman-or-discovered>` |
| priceBookId | — |
| planogramId | — |

## Image evidence

Not captured this run (login blocked). Expected fields when upload succeeds: `displayUrl`, `thumbnailUrl`, `contentType`, `width`, `height`, `sizeBytes`.

## Postman fixes confirmed

| Fix | Status |
|-----|--------|
| Product image multipart, no manual Content-Type | Static validator PASS |
| Product create valid JSON via JSON.stringify | Static validator PASS |
| Product create DTO aligned to backend | Keys match `V1AdminProductMutationRequest` |
| ID variables saved in tests | `primaryMediaId`, `productId`, `priceBookId`, etc. |
| Gated-write idempotency runtime keys | 0 missing across suite |
| Planogram/price pre-request builders | Patched in main collection |
| Focused flow collection | `avf-product-admin-to-app-flow.postman_collection.json` |

## Newman evidence

```
Folder: Health — 3 requests, 3 assertions, 0 failures
Duration: ~857ms
```

## Remaining risks

- Full authenticated canary not executed in this session; operator must re-run flow with real `adminPassword` and `machineId`.
- Planogram publish requires valid machine topology and operator session; may 409/422 on machines without prior setup.
- gRPC/MQTT paths require machine credentials, not admin JWT.

None found in public health/canary scope beyond credential gap above.
