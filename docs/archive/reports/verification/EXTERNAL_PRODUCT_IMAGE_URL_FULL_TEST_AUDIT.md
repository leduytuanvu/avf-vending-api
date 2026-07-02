# External Product Image URL — Full Test Audit

Date: 2026-05-21  
Branch: `feature/external-product-image-url-cache-flow`  
SHA: `fccb8a8` (+ test commits pending)

## Implemented endpoints

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/v1/admin/media/external-images` | Register allowlisted HTTPS URL → `media_assets` (`source_type=external`) |
| POST | `/v1/admin/products` | Create product; `primaryMediaId` or `primaryImageUrl` |
| POST | `/v1/admin/products` (update) | `primaryMediaIdReplace` on PATCH/PUT |
| PUT | `/v1/admin/planograms/...` | N/A — assignment via planogram draft + publish |
| PUT | `/v1/admin/machines/{machineId}/planograms/draft` | Slot → product assignment |
| POST | `/v1/admin/machines/{machineId}/planograms/publish` | Publish planogram |
| GET | `/v1/machines/{machineId}/sale-catalog` | Runtime catalog with `image.*` metadata |
| gRPC | `MachineCatalogService.GetSaleCatalog` / `SyncCatalogBundle` | `primary_media` with external fields |

Upload routes (separate pipeline, 503 when store nil):

- POST `/v1/admin/media/uploads/init`
- POST `/v1/admin/media/uploads/{mediaId}/complete`

## Required config

```env
PRODUCT_IMAGE_EXTERNAL_URLS_ENABLED=true
PRODUCT_IMAGE_EXTERNAL_URL_ALLOWED_HOSTS=adm.avf.vn
PRODUCT_IMAGE_EXTERNAL_URL_REQUIRE_HTTPS=true
PRODUCT_IMAGE_EXTERNAL_URL_MAX_BYTES=5242880
PRODUCT_IMAGE_EXTERNAL_URL_TIMEOUT=10s
```

Object storage (`API_ARTIFACTS_ENABLED`) **not** required for external URL flow.

## DB tables / fields

No new migration in feature branch. Uses existing:

- `media_assets`: `source_type`, `original_url`, `status`, `mime_type`, `object_version`
- `product_images` + `product_media`: projection with `display_url`, `thumb_url`, `original_url`, `content_hash`, `media_version`
- Runtime query `RuntimeListProductImagesForProducts` exposes `media_source_type`

## Request / response DTOs

**Register external image** (`V1AdminExternalProductImageRequest`):

```json
{
  "url": "https://adm.avf.vn/storage/photos/1/Product/69f0e277129d9.png",
  "purpose": "product_image",
  "filename": "69f0e277129d9.png",
  "contentType": "image/png"
}
```

**Response** includes: `mediaId`, `sourceType`, `displayUrl`, `thumbnailUrl`, `cacheKey`, `version`, `offlineCache`, `status`.

**Sale catalog item image** (REST): `mediaId`, `sourceType`, `displayUrl`, `thumbUrl`, `cacheKey`, `version`, `offlineRequired`, `downloadStrategy`.

**gRPC** `ProductMediaRef`: `source_type`, `cache_key`, `offline_required`, `download_strategy` (backward-compatible field numbers 16–19).

## Test matrix

| Area | Tests | Status |
|------|-------|--------|
| URL validation / SSRF | `external_url_test.go`, `external_url_validation_test.go` | Implemented |
| Service register + idempotency | E2E `TestExternalProductImage_registerBindMachineCatalog` | Implemented (needs `TEST_DATABASE_URL`) |
| HTTP routes / 503 not 404 | `admin_media_http_test.go` | Implemented |
| gRPC proto mapping | `TestProductMediaRefProto_externalURLMetadata` | Implemented |
| Machine catalog metadata | E2E integration asserts `cacheKey`, `external_url` | Implemented |
| MQTT image binary | N/A — MQTT only triggers sync | Documented |
| Vending app offline cache | Contract doc only | Documented |
| Postman production suite | Route in OpenAPI; collection folder missing | **Gap** |
| Production live smoke | Manual plan doc | **Not executed** |

## Known gaps before fixes

1. `postman/production-full-suite/` absent in workspace; generated REST collection has no `external-images` folder yet.
2. Full `go test ./...` with shared Docker Postgres shows parallel integration test pollution (pre-existing); feature-scoped tests pass in isolation.
3. Remote HEAD probe to real `adm.avf.vn` not run in CI (E2E uses `RemoteProbe` override + production URL storage).
4. Vending app client not in this repository.
