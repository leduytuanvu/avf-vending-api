# Product External Image URL → Machine Flow

End-to-end flow for registering legacy hosted product images and exposing them on machine sale catalogs for online/offline vending UI.

## Why external URL alone is not enough for offline

A bare HTTPS URL in a product record tells the app where to fetch an image when online. Offline vending requires a **stable cache key**, **version**, and **local file** so the UI can render products without network access.

The backend registers external URLs as `media_assets` rows and projects them into `product_media` / sale catalog responses with `cacheKey`, `version`, and offline hints.

## Prerequisites

```env
PRODUCT_IMAGE_EXTERNAL_URLS_ENABLED=true
PRODUCT_IMAGE_EXTERNAL_URL_ALLOWED_HOSTS=adm.avf.vn
PRODUCT_IMAGE_EXTERNAL_URL_REQUIRE_HTTPS=true
```

Object storage (`API_ARTIFACTS_ENABLED`) is **not** required for this flow.

## 1. Admin login

`POST /v1/auth/login` → save `accessToken`.

## 2. Register external image

`POST /v1/admin/media/external-images`

```json
{
  "url": "https://adm.avf.vn/storage/photos/1/Product/69f0e277129d9.png",
  "purpose": "product_image",
  "filename": "69f0e277129d9.png",
  "contentType": "image/png"
}
```

Response includes `mediaId`, `cacheKey`, `displayUrl`, `offlineCache`.

## 3. Create product with primaryMediaId

```json
{
  "sku": "COCA-330ML-001",
  "name": "Coca Cola Can 330ml",
  "categoryId": "{{categoryId}}",
  "brandId": "{{brandId}}",
  "primaryMediaId": "{{mediaId}}",
  "active": true,
  "status": "active"
}
```

Alternatively use `primaryImageUrl` on create to register + bind in one step.

## 4. Assign product to machine

Use the project’s planogram flow:

1. `PUT /v1/admin/machines/{machineId}/planograms/draft` — assign product to slot
2. `POST /v1/admin/machines/{machineId}/planograms/publish` — publish with Idempotency-Key
3. Optional: `POST /v1/admin/machines/{machineId}/sync` — notify machine

## 5. Machine catalog sync

REST: `GET /v1/machines/{machineId}/sale-catalog?include_images=true`

gRPC: `MachineCatalogService.GetSaleCatalog` or `SyncCatalogBundle`

Each item should include:

```json
"image": {
  "mediaId": "...",
  "sourceType": "external_url",
  "displayUrl": "https://adm.avf.vn/storage/photos/1/Product/69f0e277129d9.png",
  "cacheKey": "external-image:...:v1",
  "version": 1,
  "offlineRequired": true,
  "downloadStrategy": "download_when_online_use_local_when_offline"
}
```

## 6. App offline cache

See [VENDING_APP_PRODUCT_IMAGE_OFFLINE_CACHE_CONTRACT.md](./VENDING_APP_PRODUCT_IMAGE_OFFLINE_CACHE_CONTRACT.md).

## MQTT role

MQTT commands notify the machine to sync catalog/config. Image bytes are never sent on MQTT.

## Troubleshooting

| Symptom | Cause / fix |
|---------|-------------|
| Raw 404 on media routes | Route not mounted — upgrade to build that always mounts media routes |
| 503 `capability_not_configured` | External URLs disabled or upload-only env — set `PRODUCT_IMAGE_EXTERNAL_URLS_ENABLED=true` |
| 400 invalid URL | Host not allowlisted, HTTP when HTTPS required, or SSRF-blocked target |
| 400 missing idempotency | Add `Idempotency-Key` header on writes |
| Product not on machine | Publish planogram / check assortment + slot assignment |
| Image online but not offline | App must download using `cacheKey`; verify prior catalog sync while online |

## Example image URLs

- https://adm.avf.vn/storage/photos/1/Product/69f0e277129d9.png
- https://adm.avf.vn/storage/photos/1/Product/69f5c789105c0.png
- https://adm.avf.vn/storage/photos/1/Product/68833a13a45e5.png
