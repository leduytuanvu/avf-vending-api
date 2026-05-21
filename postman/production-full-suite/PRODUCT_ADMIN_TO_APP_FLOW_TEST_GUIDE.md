# Product Admin To App Flow Test Guide

## 1. Purpose

End-to-end production canary for the **catalog admin → price → planogram → machine sale catalog** path.

A **product** is catalog-level metadata (SKU, name, media). It is **not** automatically sold by every machine. A machine sells a product only after:

1. A **price book** assigns a sellable `unitPriceMinor` for the product (and the book is targeted to the machine/site/company as required).
2. A **planogram draft** assigns the product to a slot on that machine.
3. **Planogram publish** pushes config to the machine (MQTT command `machine_planogram_publish`).

Product images upload to **Cloudinary** via `POST /v1/admin/product-images`. The app should render `displayUrl` / `thumbnailUrl` online and cache locally for offline display.

## 2. Import Postman collection/environment

Import both files from `postman/production-full-suite/`:

| File | Role |
|------|------|
| `avf-product-admin-to-app-flow.postman_collection.json` | Curated step-by-step flow (recommended) |
| `avf-production-full.postman_collection.json` | Full API suite (same requests, all modules) |
| `avf-production.postman_environment.json` | Production variables and safety flags |

## 3. Required environment variables

| Variable | Value |
|----------|--------|
| `baseUrl` | `https://api.ldtv.dev` |
| `adminEmail` | `<set-in-postman>` (e.g. `admin@ldtv.dev`) |
| `adminPassword` | `<set-in-postman>` — never commit |
| `machineId` | `<set-in-postman-or-discovered>` — existing production machine UUID |
| `accessToken` | `<auto-populated-after-login>` |
| Refresh token | `<auto-populated-after-login>` (saved as env key after login) |

Flow IDs (auto-populated by tests when steps succeed):

`categoryId`, `brandId`, `tagId`, `primaryMediaId`, `primaryMediaUrl`, `primaryMediaThumbnailUrl`, `productId`, `canaryProductSku`, `priceBookId`, `planogramId`, `commandId`, `operatorSessionId`

## 4. Safety flags for production writes

Set before any `[GATED-WRITE]` request:

```
allowGatedWrites=true
confirmProductionWrites=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION
allow_destructive=true
canaryMode=true
readiness=true
```

Gated writes require headers (set automatically by pre-request scripts):

- `Authorization: Bearer {{accessToken}}`
- `Accept: application/json`
- `X-Request-ID: {{_runtimeRequestId}}`
- `X-Correlation-ID: {{_runtimeCorrelationId}}`
- `Idempotency-Key: {{_runtimeIdempotencyKey}}`

## 5. Login

**POST** `{{baseUrl}}/v1/auth/login`

Headers: `Content-Type: application/json`, `Accept: application/json`

Body:

```json
{
  "email": "{{adminEmail}}",
  "password": "{{adminPassword}}"
}
```

Expected **200** with `accessToken` / `access_token` and optional `refreshToken`.

Variables saved: `accessToken`, `refreshToken`

Common errors:

- **401** — wrong password
- **invalid_json** — body not valid JSON (use raw JSON, not form-data)

## 6. Create category

**POST** `{{baseUrl}}/v1/admin/categories` `[GATED-WRITE]`

Body (built by pre-request, camelCase `V1AdminCategoryMutationRequest`):

```json
{
  "name": "Canary Drinks <timestamp>",
  "slug": "canary-drinks-<timestamp>",
  "active": true
}
```

Expected **200/201** with `id`.

Variables saved: `categoryId`

Read-back: **GET** `{{baseUrl}}/v1/admin/categories` or **GET** `{{baseUrl}}/v1/admin/categories/{{categoryId}}`

## 7. Create brand

**POST** `{{baseUrl}}/v1/admin/brands` `[GATED-WRITE]`

Body (`V1AdminBrandMutationRequest`):

```json
{
  "name": "Canary Brand <timestamp>",
  "slug": "canary-brand-<timestamp>",
  "active": true
}
```

Variables saved: `brandId`

## 8. Create tag (supported)

**POST** `{{baseUrl}}/v1/admin/tags` `[GATED-WRITE]`

Body (`V1AdminTagMutationRequest`):

```json
{
  "name": "Canary Tag <timestamp>",
  "slug": "canary-tag-<timestamp>",
  "active": true
}
```

Optional on product create via `tagIds: ["{{tagId}}"]`.

Variables saved: `tagId`

## 9. Upload product image

**POST** `{{baseUrl}}/v1/admin/product-images` `[GATED-WRITE]`

**multipart/form-data** — do **not** manually set `Content-Type` (Postman sets boundary).

| Field | Type | Value |
|-------|------|--------|
| `file` | file | local png/jpg/webp/gif ≤ 5MB |
| `purpose` | text | `product_image` |
| `altText` | text | `Coca Cola 330ml product image` |

Expected **201** (or **200**) shape:

```json
{
  "mediaId": "<uuid>",
  "provider": "cloudinary",
  "displayUrl": "https://res.cloudinary.com/...",
  "thumbnailUrl": "https://res.cloudinary.com/...",
  "contentType": "image/png",
  "width": 800,
  "height": 800,
  "sizeBytes": 183421,
  "status": "ready"
}
```

Variables saved: `primaryMediaId`, `primaryMediaUrl`, `primaryMediaThumbnailUrl`

Common errors:

- **503 capability_not_configured** — Cloudinary not configured on API
- **invalid_json** — you set `Content-Type: multipart/form-data` manually; remove it

## 10. Create product

**POST** `{{baseUrl}}/v1/admin/products` `[GATED-WRITE]`

Pre-request builds strict JSON via `JSON.stringify` matching `V1AdminProductMutationRequest` (`internal/httpserver/openapi_types.go`):

```json
{
  "sku": "COCA-330ML-<timestamp>",
  "name": "Coca Cola Can 330ml",
  "description": "Production canary test product",
  "active": true,
  "ageRestricted": false,
  "allergenCodes": [],
  "categoryId": "<uuid>",
  "brandId": "<uuid>",
  "primaryMediaId": "<uuid>"
}
```

Optional: `"tagIds": ["<tagId>"]`

**Required when `active: true`:** `primaryMediaId` must reference a ready uploaded media row.

Headers: `Content-Type: application/json` plus gated-write headers above.

Expected **201** with product `id`, `sku`, optional nested media URLs.

Variables saved: `productId`, `canaryProductSku`, `sku`

Common errors:

- **invalid_json** — unknown fields (backend uses `DisallowUnknownFields`) or malformed JSON from Postman `{{var}}` inside invalid JSON
- **validation** — missing `primaryMediaId`, `categoryId`, or `brandId`

## 11. Verify product

**GET** `{{baseUrl}}/v1/admin/products/{{productId}}`

Assert `id`, `sku`, `name`, and primary media references if returned.

## 12. Configure price

### 12a. Create price book

**POST** `{{baseUrl}}/v1/admin/price-books` `[GATED-WRITE]`

```json
{
  "name": "canary-price-book-<timestamp>",
  "currency": "USD",
  "effectiveFrom": "<ISO8601>",
  "isDefault": false,
  "scopeType": "company",
  "priority": 10
}
```

Variables saved: `priceBookId`

### 12b. Add product price

**PUT** `{{baseUrl}}/v1/admin/price-books/{{priceBookId}}/items` `[GATED-WRITE]`

```json
{
  "items": [
    {
      "productId": "{{productId}}",
      "unitPriceMinor": 150
    }
  ]
}
```

### 12c. Assign price book to machine

**POST** `{{baseUrl}}/v1/admin/price-books/{{priceBookId}}/assign-target` `[GATED-WRITE]`

```json
{
  "machineId": "{{machineId}}"
}
```

Read-back: **GET** `{{baseUrl}}/v1/admin/price-books/{{priceBookId}}/items`

## 13. Assign product to machine (planogram)

### 13a. Save planogram draft

**PUT** `{{baseUrl}}/v1/admin/machines/{{machineId}}/planograms/draft` `[GATED-WRITE]`

Pre-request builds:

```json
{
  "operator_session_id": "<uuid>",
  "syncLegacyReadModel": true,
  "items": [
    {
      "cabinetCode": "A",
      "layoutKey": "grid-4x6",
      "layoutRevision": 1,
      "slotCode": "A1",
      "legacySlotIndex": 1,
      "productId": "<productId>",
      "maxQuantity": 12,
      "priceMinor": 150,
      "metadata": {}
    }
  ]
}
```

If updating an existing planogram, include `planogramId` and `planogramRevision`.

### 13b. Publish planogram

**POST** `{{baseUrl}}/v1/admin/machines/{{machineId}}/planograms/publish` `[GATED-WRITE]`

Same body shape as draft. Dispatches MQTT command type `machine_planogram_publish`.

Variables saved: `planogramId`, `commandId`

### 13c. Trigger machine sync (optional)

**POST** `{{baseUrl}}/v1/admin/machines/{{machineId}}/sync` `[GATED-WRITE]`

## 14. Verify machine/app catalog sync

### REST (legacy, machine JWT or configured auth)

**GET** `{{baseUrl}}/v1/machines/{{machineId}}/sale-catalog`

Expect items containing `productId`, `sku`, `name`, `priceMinor`, slot assignment, and image URLs when `include_images` default applies.

### gRPC (preferred runtime)

Service: `avf.machine.v1.MachineCatalogService` (`proto/avf/machine/v1/catalog.proto`)

```bash
grpcurl -H "authorization: Bearer $MACHINE_TOKEN" \
  -d '{"machine_id":"'"$MACHINE_ID"'","include_images":true}' \
  api.ldtv.dev:443 avf.machine.v1.MachineCatalogService/GetSaleCatalog
```

Also available: `SyncSaleCatalog`, `SyncCatalogBundle`, `GetMediaManifest`.

Postman cannot execute gRPC natively; use `grpcurl` or app SDK.

## 15. Verify app image online behavior

Online: render product tile from `displayUrl` (full) or `thumbnailUrl` (grid). URLs come from Cloudinary CDN in upload response and catalog snapshot `primary_media`.

## 16. Verify app image offline cache expectation

After `SyncCatalogBundle` / `GetMediaManifest`, app caches media by URL + checksum/version. Offline UI should use cached file; no binary bytes over gRPC/MQTT.

## 17. gRPC test commands

```bash
# Sale catalog snapshot
grpcurl -H "authorization: Bearer $MACHINE_TOKEN" \
  -d '{"meta":{"machine_id":"'"$MACHINE_ID"'"},"machine_id":"'"$MACHINE_ID"'","include_images":true}' \
  api.ldtv.dev:443 avf.machine.v1.MachineCatalogService/SyncSaleCatalog

# Media manifest only
grpcurl -H "authorization: Bearer $MACHINE_TOKEN" \
  -d '{"meta":{"machine_id":"'"$MACHINE_ID"'"},"machine_id":"'"$MACHINE_ID"'"}' \
  api.ldtv.dev:443 avf.machine.v1.MachineCatalogService/GetMediaManifest
```

Requires a valid **machine JWT** (`machineToken`), not admin JWT.

## 18. MQTT test commands

Planogram publish creates a command ledger entry; machines subscribe to configured command topics.

Observe (replace host/credentials):

```bash
mosquitto_sub -h "$MQTT_HOST" -p 8883 -u "$MQTT_USER" -P "$MQTT_PASS" \
  -t "avf/prod/machines/$MACHINE_ID/commands/#" -v
```

Publish is server-initiated after `POST .../planograms/publish`; manual `mosquitto_pub` is not the normal catalog sync path.

## 19. Troubleshooting

| Symptom | Cause | Fix |
|---------|--------|-----|
| `invalid_json` on product create | Extra fields or bad Postman templating | Use pre-request `JSON.stringify`; only DTO fields |
| `invalid_json` on image upload | Manual multipart Content-Type | Remove Content-Type header on upload request |
| Product create 422 validation | Missing media/category/brand | Run prior steps; check env IDs not placeholders |
| Sale catalog empty | No planogram publish or wrong machine | Complete planogram draft + publish for `machineId` |
| gRPC 403 | Admin token used | Use machine JWT |
| GATED-WRITE blocked | Safety flags | Set `allowGatedWrites` + `confirmProductionWrites` |

## Validation scripts

```bash
python postman/production-full-suite/validate_product_flow_suite.py
newman run postman/production-full-suite/avf-product-admin-to-app-flow.postman_collection.json \
  -e postman/production-full-suite/avf-production.postman_environment.json \
  --folder Health
```
