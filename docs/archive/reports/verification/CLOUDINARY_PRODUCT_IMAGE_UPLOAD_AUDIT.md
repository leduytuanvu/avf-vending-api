# Cloudinary Product Image Upload Audit

Generated: 2026-05-21

## Existing routes (before)

| Route | Status |
| --- | --- |
| `POST /v1/admin/media/uploads/init` | S3 presign; 503 when `API_ARTIFACTS_ENABLED=false` |
| `POST /v1/admin/media/external-images` | External URL; 503 when feature disabled |
| `POST /v1/admin/products` | Supports `primaryMediaId`, `primaryImageUrl` |
| `POST /v1/admin/products/{productId}/media` | Bind ready media as primary |

## Added

| Route | Purpose |
| --- | --- |
| `POST /v1/admin/product-images` | Multipart Cloudinary server-side upload |
| `POST /v1/admin/media/product-images` | Alias of the same handler |

## DB

Extended `media_assets`:

- `storage_provider` (`s3`, `cloudinary`, `external`)
- `provider_public_id`, `provider_asset_id`
- `source_type` includes `cloudinary`

Migration: `migrations/00007_cloudinary_media_assets.sql`

## Config

| Env | Role |
| --- | --- |
| `MEDIA_PROVIDER=cloudinary` | Select Cloudinary provider |
| `MEDIA_UPLOAD_ENABLED=true` | Enable multipart upload |
| `CLOUDINARY_CLOUD_NAME` | Cloud name |
| `CLOUDINARY_API_KEY` | API key |
| `CLOUDINARY_API_SECRET` | **Server only** |
| `CLOUDINARY_FOLDER` | Upload folder (default `avf-vending/products`) |
| `MEDIA_MAX_IMAGE_SIZE_MB` | Max file size (default 5) |
| `MEDIA_ALLOWED_IMAGE_TYPES` | MIME allowlist |

Capability: when Cloudinary configured, `MediaAdmin` is wired and `POST /v1/admin/product-images` returns 201. When absent, **503** `capability_not_configured` / `v1.admin.media`.

External URL flow unchanged (`PRODUCT_IMAGE_EXTERNAL_URLS_ENABLED`).

## Vending catalog contract

`salecatalog.ImageMeta` for `source_type=cloudinary`:

- `displayUrl`, `thumbnailUrl` from Cloudinary HTTPS URLs
- `cacheKey`: `{mediaId}:{version}:{checksum}`
- `offlineRequired=true`, `downloadStrategy=download_when_online_use_local_when_offline`

## Test plan

- Unit: config, image validation, fake Cloudinary uploader, cache key
- HTTP: 503 when unwired, 400 missing file
- Manual: real Cloudinary with env credentials (`CLOUDINARY_INTEGRATION_TEST=true` optional)
