# Product Flow Test Guide And Result

Production canary for admin catalog → price → planogram → machine sale catalog at `https://api.ldtv.dev`.

**Important:** A product is catalog-level first. It is **not** sold on every machine until price book + planogram slot assignment + publish complete. The vending app should fetch the sale catalog online and cache `displayUrl` / `thumbnailUrl` for offline tiles.

## Variables

| Variable | Committed placeholder | Notes |
|----------|----------------------|--------|
| `baseUrl` | `https://api.ldtv.dev` | |
| `adminEmail` | `admin@ldtv.dev` | |
| `adminPassword` | `<set-in-postman>` | Never commit real password |
| `allowGatedWrites` | `true` | |
| `confirmProductionWrites` | `I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION` | |
| `machineId` | `<set-existing-machine-id>` | Or run GET `/v1/admin/machines` (saves `machineId` from list) |
| `accessToken` | `<auto-populated-after-login>` | From `tokens.accessToken` |
| `categoryId`, `brandId`, `tagId` | set after create | |
| `primaryMediaId`, `primaryMediaUrl`, `primaryMediaThumbnailUrl` | set after image upload | |
| `productId`, `canaryProductSku` | set after product create | |
| `priceBookId`, `unitPriceMinor` | `15000` (= $150.00 USD) | |
| Local image file | Postman form-data `file` | Real png/jpg/webp/gif ≤ 5MB; **do not** set multipart Content-Type manually |

Import: `avf-product-admin-to-app-flow.postman_collection.json` + `avf-production.postman_environment.json`

## Flow

1. **Login** — `POST /v1/auth/login`
2. **Create category** — `POST /v1/admin/categories`
3. **Create brand** — `POST /v1/admin/brands`
4. **Create tag** — `POST /v1/admin/tags`
5. **Upload image** — `POST /v1/admin/product-images` (multipart)
6. **Create product** — `POST /v1/admin/products` (pre-request builds `_runtimeProductCreateBody`)
7. **Configure price** — price book create → PUT items → assign-target to machine
8. **Planogram** — operator session (if legacy REST enabled) → PUT draft → POST publish
9. **Sale catalog** — `GET /v1/machines/{machineId}/sale-catalog` (gRPC preferred when REST legacy off)

## Key Requests

### Login

```http
POST /v1/auth/login
Content-Type: application/json

{"email":"{{adminEmail}}","password":"{{adminPassword}}"}
```

Success (sanitized):

```json
{
  "accountId": "d80fa04f-baec-4852-b55b-4017641e47b9",
  "email": "admin@ldtv.dev",
  "roles": ["platform_admin", "admin"],
  "tokens": { "accessToken": "[REDACTED]", "refreshToken": "[REDACTED]" }
}
```

### Upload product image

```http
POST /v1/admin/product-images
Authorization: Bearer {{accessToken}}
Idempotency-Key: {{_runtimeIdempotencyKey}}

form-data: file=<local image>, purpose=product_image, altText=Coca Cola 330ml product image
```

Success **201**:

```json
{
  "mediaId": "019e4ba2-8428-7e24-abcc-71c4dca83a9b",
  "displayUrl": "https://res.cloudinary.com/.../products/....png",
  "thumbnailUrl": "https://res.cloudinary.com/.../w_300,h_300,...",
  "contentType": "image/png",
  "status": "ready"
}
```

### Create product

Pre-request sets `{{_runtimeProductCreateBody}}` (camelCase `V1AdminProductMutationRequest`):

```json
{
  "sku": "COCA-330ML-1779385403057",
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

```http
POST /v1/admin/products
Content-Type: application/json
Body: {{_runtimeProductCreateBody}}
```

### Planogram publish

```http
PUT /v1/admin/machines/{{machineId}}/planograms/draft
POST /v1/admin/machines/{{machineId}}/planograms/publish

{
  "operator_session_id": "<active session uuid>",
  "syncLegacyReadModel": true,
  "items": [{
    "cabinetCode": "A",
    "slotCode": "A1",
    "legacySlotIndex": 1,
    "productId": "{{productId}}",
    "maxQuantity": 12,
    "priceMinor": 15000
  }]
}
```

Requires **ACTIVE** operator session on the machine (`POST /v1/machines/{machineId}/operator-sessions/login` with admin JWT + `force_admin_takeover: true` when legacy REST is enabled).

### Sale catalog

```http
GET /v1/machines/{{machineId}}/sale-catalog
Authorization: Bearer {{machineToken}} or admin with catalog read
```

Prefer gRPC: `MachineCatalogService.SyncSaleCatalog` (`proto/avf/machine/v1/catalog.proto`).

## Common Errors

| Error | Cause | Fix |
|-------|--------|-----|
| `invalid_json` on product create | Bad Postman templating / unknown fields | Use `{{_runtimeProductCreateBody}}` from pre-request |
| `primaryMediaId requires company context` | Backend: `requireCatalogPrincipalUUID` returns Nil while upload uses `MEDIA_COMPANY_ID` | Backend fix: resolve company like product-image upload (`internal/httpserver/admin_scope.go`) |
| `invalid_image_file` on upload | Manual Content-Type on multipart | Remove Content-Type header; pick real image file |
| `operator: session not found` on planogram | Random UUID or no active session | Start operator session first |
| `404 page not found` on sale-catalog / operator login | `MachineRESTLegacyEnabled=false` in production | Use gRPC catalog sync or enable legacy REST |
| GATED-WRITE blocked | Safety flags | Set `allowGatedWrites` + `confirmProductionWrites` |

## Latest Production Result

**Run:** 2026-05-21 UTC · branch `fix/postman-product-flow-production` · API `https://api.ldtv.dev`

| Step | Status |
|------|--------|
| Health live/ready/version | **PASS** |
| Login | **PASS** |
| Category / brand / tag create | **PASS** |
| Image upload | **PASS** (201, Cloudinary URLs) |
| Product create (active + primaryMediaId) | **FAIL** — `400 primaryMediaId requires company context` (backend defect) |
| Product create (inactive draft) | **PASS** (200, no media bind) |
| Price book + items + assign-target | **PASS** |
| Planogram draft/publish | **FAIL** — `404 operator: session not found` (no legacy operator REST + no real session) |
| Sale catalog | **FAIL** — `404 page not found` (legacy machine REST disabled) |

### IDs from last run (sanitized)

| ID | Value |
|----|--------|
| primaryMediaId | `019e4ba2-8428-7e24-abcc-71c4dca83a9b` |
| productId (inactive draft) | `019e4ba2-8b61-7574-8cfa-0836695dea09` |
| machineId (from admin list) | `55555555-5555-5555-5555-555555555555` |
| priceBookId | `019e4ba2-9202-79be-8ef0-975b61b0ca37` |
| displayUrl | `https://res.cloudinary.com/dz4qz0tk9/image/upload/.../....png` |

### Sale catalog contains product?

**No** — planogram publish did not complete; catalog endpoint unavailable on current production transport config.

### Image URL in sale catalog?

**N/A** — sale catalog not reachable via REST in this environment.

## Postman fixes applied

- Product create body via `_runtimeProductCreateBody` + strict field validation
- Gated-write runtime idempotency headers on all `[GATED-WRITE]` requests
- Image upload multipart without manual Content-Type
- `unitPriceMinor=15000`; machine list saves `machineId` field
- Environment placeholders (no secrets committed)

Validate: `python postman/production-full-suite/validate_product_flow_suite.py`
