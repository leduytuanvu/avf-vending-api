# Runbook: presigned / HTTPS media URL expired on kiosk

## Symptoms

- Thumbnails or hero images fail to load with HTTP **403** / **404** from object storage.
- Local logs show GET to a presigned URL whose query-string expiry is in the past.
- **`GetMediaManifest`** / catalog still lists the SKU, but the URL no longer fetches.

## Why this happens

Presigned GET URLs embed a short TTL. The API may have persisted older URLs in `product_images` / `product_media` at bind time. **Online** catalog RPCs refresh those URLs when **`MediaStore`** + **`MediaPresignTTL`** are configured (`internal/app/salecatalog/presign.go`).

The manifest also carries **`expires_at`** (aligned to the presign TTL used for that response) on `ProductMediaRef` and each `ProductMediaVariant`. **Clients must not treat URL strings as long-lived identity.**

## Resolution (client)

1. If **offline** and **`media_fingerprint`** has not changed since the file was verified: **continue to render the on-disk file**; schedule refresh when back online.
2. If **online**: call **`GetCatalogSnapshot`**, **`GetMediaDelta`**, **`GetMediaManifest`**, or **`GetMediaDelta`** (same **`include_unavailable`** as prior fingerprint) to obtain fresh HTTPS URLs.
3. After download, **verify bytes** against `checksum_sha256` for that variant; store durably under a path keyed by **kind + `media_asset_id` + hash + `media_version`**.

## Resolution (server / ops)

1. Confirm **`MediaStore`** is wired in the API process serving machine gRPC and **`MediaPresignTTL`** is non-zero for environments that rely on presigned GET.
2. Confirm the machine JWT can reach the catalog service (no ingress mismatch).
3. Re-bind or re-publish media only if the asset is actually wrong; URL expiry alone is fixed by a fresh RPC, not a catalog rewrite.

## References

- `docs/architecture/media-sync.md`
- `docs/runbooks/product-media-cache-invalidation.md`
