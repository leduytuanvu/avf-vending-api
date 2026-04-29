# Product media and cache invalidation runbook

Use this when product images, media manifests, or kiosk sale-catalog images are stale or missing.

## Implemented behavior

- Admin media/product-image writes are HTTP REST APIs under `/v1/admin`.
- When object storage is enabled, media flows use artifact/object storage and HTTPS URLs.
- Machine/catalog responses carry metadata such as `configVersion`, `catalogVersion`, media URLs, checksums, per-rendition **`media_variants`**, `expires_at` when URLs are presigned, and fingerprints. Image bytes are not sent over gRPC.
- Redis-backed sale-catalog/media cache is optional. When enabled, admin media changes bump organization media cache state so kiosks can detect updates.
- Runtime catalog lists only **active** `product_media` and **active** `product_images` with **ready** bound assets.

## App cache invalidation (exact client behavior)

| Scenario | What to compare | Action |
| --- | --- | --- |
| **First sync** | Persist `catalog_version`, `media_fingerprint`, and each variant **`checksum_sha256`** + **`media_version`** + **`media_asset_id`**. | Download each variant URL once; verify file matches **`checksum_sha256`**; store under a stable path derived from **kind + media_asset_id + hash** (not from raw URL). |
| **Signed URL expired, local file OK** | `expires_at` in the past **but** `media_fingerprint` / variant hash unchanged since last good download. | **Keep** rendering from disk. Optionally refresh presigned URLs on next online **`GetCatalogSnapshot`** / **`GetMediaManifest`**; do **not** delete local files solely because URL query params changed. |
| **Hash mismatch after download** | Byte verification fails vs **`checksum_sha256`**. | Treat as corrupted or wrong object; delete local copy; refetch after confirming new manifest; if online sync still fails, escalate (CDN/object mismatch). |
| **Product image changed** | `media_version` or variant **checksum** or **storage-key-derived fingerprint** changes (`media_fingerprint` / `catalog_version` bumps). | Evict old paths; download new renditions; old presigned URLs must not be the only source of truth — use hashes + versions. |
| **Offline mode** | No network. | Render from durable cache using last known hashes; queue **`GetCatalogDelta`** / **`GetMediaDelta`** for reconnect; never assume expired presigned URL is still valid. |

## Checks

1. Confirm `API_ARTIFACTS_ENABLED` and object storage configuration if uploads use object storage.
2. Confirm product has an active primary image and generated display/thumb URLs.
3. Fetch `GET /v1/machines/{machineId}/sale-catalog` with machine/admin token and compare `configVersion` / `catalogVersion`.
4. If gRPC machine catalog is used, call `MachineCatalogService/GetCatalogSnapshot` or `MachineMediaService/GetMediaManifest` with a valid Machine JWT. **`GetCatalogSnapshot`** / **`GetCatalogDelta`** refresh presigned HTTPS GET URLs the same way as **`GetMediaManifest`** when `MediaStore` is configured.
5. If Redis cache is enabled, verify Redis availability and key prefix; if Redis is down, see `redis-outage-behavior.md`.
6. Force the kiosk to refetch when `configVersion`, **`catalog_version`**, **`media_fingerprint`**, checksum, **`media_version`**, `etag`, or object version changes (presigned URL rotation alone should **not** bump `media_fingerprint` when hashes/versions and storage keys are unchanged).

```bash
BASE_URL="http://localhost:8080"
TOKEN="<bearer token with machine access or allowed admin access>"
MACHINE_ID="55555555-5555-5555-5555-555555555555"

curl -sS "$BASE_URL/v1/machines/$MACHINE_ID/sale-catalog?include_images=true" \
  -H "Authorization: Bearer $TOKEN"
```

## PowerShell sale-catalog check

```powershell
$BaseUrl = "http://localhost:8080"
$Token = "<bearer token with machine access or allowed admin access>"
$MachineId = "55555555-5555-5555-5555-555555555555"

Invoke-RestMethod -Method Get `
  -Uri "$BaseUrl/v1/machines/$MachineId/sale-catalog?include_images=true" `
  -Headers @{ Authorization = "Bearer $Token" }
```

## Do not do

- Do not cache presigned URLs past their validity window as if they were assets; cache **files** keyed by integrity metadata.
- Do not ship image bytes through machine gRPC; use HTTPS URLs.
- Do not manually edit Redis keys as the primary fix. Re-run the admin media/catalog write or disable cache only as an incident workaround.

Related: `docs/architecture/media-sync.md`, `docs/api/machine-grpc.md`, `docs/runbooks/media-url-expired.md`, `docs/runbooks/redis-outage-behavior.md`.
