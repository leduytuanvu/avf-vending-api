# Product Flow Test Guide And Result

Production canary for admin catalog → price → planogram → machine sale catalog at `https://api.ldtv.dev`.

## Import

1. Import `avf-production-full.postman_collection.json`
2. Import `avf-production.postman_environment.json`
3. Select **AVF Production** environment

Validate locally: `python postman/production-full-suite/validate_product_flow_suite.py`

Re-apply patches after regenerating the collection: `python postman/production-full-suite/patch_product_flow_suite.py`

## Environment defaults

| Variable | Value |
|----------|--------|
| `baseUrl` | `https://api.ldtv.dev` |
| `adminEmail` | `admin@ldtv.dev` |
| `adminPassword` | set in Postman (defaults in env file for local runs) |
| `allowGatedWrites` | `true` |
| `confirmProductionWrites` | `I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION` |
| `allow_destructive` / `canaryMode` / `readiness` | `true` |
| `unitPriceMinor` | `15000` |
| `accessToken` | empty — filled by login |
| `primaryMediaId`, `categoryId`, `brandId`, `productId`, `machineId` | empty — filled by test scripts |

## Manual Postman flow

1. **Login** — `POST /v1/auth/login` → saves `accessToken`
2. **Create category / brand / tag** (if env vars empty) — `POST /v1/admin/categories`, `/brands`, `/tags`
3. **Upload product image** — `[GATED-WRITE] POST /v1/admin/product-images (Cloudinary multipart)`
   - Body = **form-data** (not raw JSON)
   - `file` (type **File**): select a real local `.png`, `.jpg`, `.jpeg`, `.webp`, or `.gif` (≤ 5 MB)
   - `purpose` = `product_image`, `altText` = `Sample product image`
   - **Do not** add a manual `Content-Type` header — Postman must send `multipart/form-data; boundary=…`
   - Postman may omit the part-level `Content-Type` or send `application/octet-stream`; the API sniffs magic bytes and accepts valid PNG/JPEG/WebP/GIF
   - Test script saves `primaryMediaId` / `mediaId`, `primaryMediaUrl`, `primaryMediaThumbnailUrl`
4. **Create product** — `[GATED-WRITE] POST /v1/admin/products`
   - Raw body = `{{_runtimeProductCreateBody}}` (built by pre-request script)
   - Requires `categoryId`, `brandId`; includes `primaryMediaId` only when upload succeeded
5. **Price** — create price book → PUT items → assign-target to `machineId`
6. **Operator session** — `POST /v1/admin/machines/{machineId}/operator-sessions/start`
7. **Planogram** — PUT draft → POST publish (uses `operatorSessionId`)
8. **Verify MQTT** — publish response includes `commandId` and `dispatchState`
9. **gRPC catalog** (optional) — only when `machine-api.ldtv.dev` DNS resolves and machine token is available

### If upload returns `invalid_multipart`

1. Open request **Headers** — delete any `Content-Type` row (including disabled ones)
2. **Body** → **form-data** (not binary, not raw)
3. `file` key → type **File** → re-select a real image file
4. Do not send raw JSON for this endpoint
5. Pre-request script must not set `Content-Type`

### Newman (image upload)

```bash
newman run postman/production-full-suite/avf-production-full.postman_collection.json \
  -e postman/production-full-suite/avf-production.postman_environment.json \
  --folder "Auth" \
  --folder "Product Media" \
  --working-dir postman/production-full-suite
```

Uses `assets/sample-product.png` when the collection file field `src` is set.

### curl verification (after deploy)

```bash
ACCESS_TOKEN="$(curl -sS -X POST "$BASE_URL/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@ldtv.dev","password":"..."}' | jq -r '.tokens.accessToken')"

# Explicit part type (always works)
curl -i -X POST "$BASE_URL/v1/admin/product-images" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Idempotency-Key: $(uuidgen)" \
  -F "file=@postman/production-full-suite/assets/sample-product.png;type=image/png" \
  -F "purpose=product_image" \
  -F "altText=Sample product image"

# Without explicit part type (Postman-like)
curl -i -X POST "$BASE_URL/v1/admin/product-images" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Idempotency-Key: $(uuidgen)" \
  -F "file=@postman/production-full-suite/assets/sample-product.png" \
  -F "purpose=product_image" \
  -F "altText=Sample product image"
```

Expected **201**: `mediaId`, `displayUrl`, `thumbnailUrl`, `status=ready`, `provider=cloudinary`.

## Expected success responses

### Product image upload — **201**

```json
{
  "mediaId": "<uuid>",
  "displayUrl": "https://res.cloudinary.com/...",
  "thumbnailUrl": "https://res.cloudinary.com/...",
  "status": "ready",
  "provider": "cloudinary"
}
```

### Product create — **200 or 201**

```json
{
  "id": "<uuid>",
  "sku": "COCA-ACTIVE-<timestamp>",
  "active": true,
  "primaryMediaId": "<uuid>"
}
```

### Planogram publish — **200**

```json
{
  "commandId": "<uuid>",
  "dispatchState": "published"
}
```

### gRPC SyncSaleCatalog (when available)

`snapshot.items[]` contains `productId`, `priceMinor`, `slot`, `primaryMedia.displayUrl`, `primaryMedia.thumbnailUrl`.

## Common errors

| Error | Fix |
|-------|-----|
| `invalid_multipart` | Remove manual Content-Type; use form-data + File type |
| `invalid_image_file` / unsupported content type | Use a real image file; API sniffs bytes — fake `.png` text files are rejected |
| GATED-WRITE blocked | Set `allowGatedWrites=true` and `confirmProductionWrites=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION` |
| `invalid_json` on product create | Use `{{_runtimeProductCreateBody}}`; do not hand-edit raw JSON with unresolved `{{vars}}` |

## Latest production result

| Step | Status |
|------|--------|
| Health / login | **PASS** |
| Image upload (Cloudinary) | **PASS** (201) |
| Product create | **PASS** (200) |
| Price + planogram publish | **PASS** |
| gRPC catalog | **BLOCKED_DNS_PENDING** (`machine-api.ldtv.dev` NXDOMAIN) |

Canary IDs from last full run: `productId` `019e4c17-de65-71c4-b0ab-3013619b2e8c`, `primaryMediaId` `019e4c17-d4aa-7c3e-b047-855575ae63c4`, `machineId` `55555555-5555-5555-5555-555555555555`.

## Suite layout

- One collection: `avf-production-full.postman_collection.json`
- One environment: `avf-production.postman_environment.json`
- Sample image: `assets/sample-product.png`
- Validator: `validate_product_flow_suite.py`
- Patch script: `patch_product_flow_suite.py`
