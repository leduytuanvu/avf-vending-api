# Production-Safe External Product Image Smoke Test

Use a **canary machine** only. Do not use real credentials in committed files.

## Prerequisites

Production API must have:

```env
PRODUCT_IMAGE_EXTERNAL_URLS_ENABLED=true
PRODUCT_IMAGE_EXTERNAL_URL_ALLOWED_HOSTS=adm.avf.vn
```

Object storage may remain disabled for this smoke.

## 1. Login

```bash
curl -sS -X POST "$BASE_URL/v1/auth/login" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -d '{"email":"YOUR_ADMIN_EMAIL","password":"YOUR_PASSWORD"}'
```

Save `accessToken`.

## 2. Register external image

```bash
curl -sS -X POST "$BASE_URL/v1/admin/media/external-images" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -H "Idempotency-Key: $(uuidgen)" \
  -H "X-Request-ID: $(uuidgen)" \
  -H "X-Correlation-ID: $(uuidgen)" \
  -d '{
    "url": "https://adm.avf.vn/storage/photos/1/Product/69f0e277129d9.png",
    "purpose": "product_image",
    "filename": "69f0e277129d9.png",
    "contentType": "image/png"
  }'
```

Expect **201** (or **200** on idempotent replay). Save `mediaId`, `cacheKey`.

If **503** with `capability_not_configured`: enable `PRODUCT_IMAGE_EXTERNAL_URLS_ENABLED` and redeploy.

## 3. Create product

Use unique SKU/barcode. Set `primaryMediaId` to saved `mediaId`.

## 4. Assign to canary machine

1. `PUT /v1/admin/machines/{canaryMachineId}/planograms/draft` — slot with new `productId`
2. `POST /v1/admin/machines/{canaryMachineId}/planograms/publish` with `Idempotency-Key`

## 5. Fetch sale catalog

```bash
curl -sS "$BASE_URL/v1/machines/$CANARY_MACHINE_ID/sale-catalog?include_images=true" \
  -H "Authorization: Bearer $MACHINE_OR_ADMIN_TOKEN" \
  -H "Accept: application/json"
```

Verify item contains:

- `image.displayUrl` → `https://adm.avf.vn/...`
- `image.cacheKey` → `external-image:...:v1`
- `image.version` / `image.mediaVersion`
- `image.sourceType` → `external_url`
- `image.offlineRequired` → true

## 6. Vending app checks

- Online: product tile loads image from `displayUrl`
- After sync while online: local cache file keyed by `cacheKey`
- Offline: cached image or placeholder (never blank product row)

## 7. Cleanup

Deactivate test product or remove from canary planogram if cleanup endpoints exist.

## Example URLs

- https://adm.avf.vn/storage/photos/1/Product/69f0e277129d9.png
- https://adm.avf.vn/storage/photos/1/Product/69f5c789105c0.png
- https://adm.avf.vn/storage/photos/1/Product/68833a13a45e5.png
