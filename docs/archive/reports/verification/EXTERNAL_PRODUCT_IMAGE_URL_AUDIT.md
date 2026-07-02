# External Product Image URL Flow — Audit

Date: 2026-05-20  
Branch: `feature/external-product-image-url-cache-flow`

## Summary

The platform already models external images in PostgreSQL (`media_assets.source_type`, `original_url`; `product_media.source_type`, `original_url`, display/thumb URLs). The gap was **admin registration**, **bind projection** for external assets (no object-store presign), and **catalog cache metadata** (`cacheKey`, `sourceType`, offline hints) for vending clients.

Object storage upload remains a separate enterprise pipeline gated by `API_ARTIFACTS_ENABLED`. External URL registration is gated by `PRODUCT_IMAGE_EXTERNAL_URLS_ENABLED`.

## Current product create API

`POST /v1/admin/products` accepts `primaryMediaId` (camelCase). Active products require a primary media binding when the media binder is wired. Response includes `primaryMediaId` and optional nested `media.primary` with variants when loaded.

Optional `primaryImageUrl` on create registers external media inline (same transaction) when external URLs are enabled.

## Current media upload API

When `API_ARTIFACTS_ENABLED` / object storage is wired:

- `POST /v1/admin/media/uploads/init` — pending asset + presigned PUT
- `POST /v1/admin/media/uploads/{mediaId}/complete` — variants + `ready`

When not wired: upload routes return **503 JSON** `capability_not_configured` (not raw 404).

## New external image API

`POST /v1/admin/media/external-images` — registers HTTPS allowlisted URL as `media_assets.source_type = external`, `status = ready`. Does **not** require object storage.

When `PRODUCT_IMAGE_EXTERNAL_URLS_ENABLED=false`: **503** `capability_not_configured`.

## Sale catalog response image shape

`GET /v1/machines/{machineId}/sale-catalog` and gRPC `GetCatalogSnapshot` / `SyncCatalogBundle` expose per-item image metadata:

- `mediaId`, `sourceType`, `displayUrl`, `thumbnailUrl`, `contentType`
- `cacheKey`, `version` (media version)
- `offlineRequired`, `downloadStrategy`

Legacy fields (`contentHash`, `etag`, `mediaVersion`, variants) remain for upload/presigned assets.

## Vending app image metadata

Runtime queries join `product_images` → `product_media` → optional `media_assets`. External URLs populate `display_url` / `thumb_url` and `original_url` without presigned rotation.

## DB changes

**No new migration required** — schema from `00002_platform_schema.sql` already supports `source_type = external` and URL columns.

SQL/query updates:

- `CatalogWriteUpsertProductMediaProjection` — parameterized `source_type` + `original_url`
- `MediaAdminGetAssetByOriginalURL` — idempotent external registration
- `RuntimeListProductImagesForProducts` — expose `media_source_type`

## API changes

| Area | Change |
|------|--------|
| Config | `PRODUCT_IMAGE_EXTERNAL_URLS_*` env vars |
| Admin media | `POST /v1/admin/media/external-images` |
| Media bind | External assets use stored URL, not presign |
| Product create | Optional `primaryImageUrl` |
| Sale catalog HTTP/gRPC | `cacheKey`, `sourceType`, offline cache hints |
| Routes | Always mount media routes; capability-specific 503 |

## Tests needed

- URL validation (allowlist, HTTPS, SSRF rejects)
- Register external asset `source_type=external`, `status=ready`
- Product bind + sale catalog includes `displayUrl` / `cacheKey` / `version`
- Feature disabled → 503 JSON
- Idempotent re-register same normalized URL
- OpenAPI + Postman inventory includes new route
- Upload flow unchanged when artifacts enabled

## Object storage requirement

| Flow | Requires object storage? |
|------|-------------------------|
| External URL registration + product bind | **No** |
| Presigned upload pipeline | **Yes** (`API_ARTIFACTS_ENABLED`) |
