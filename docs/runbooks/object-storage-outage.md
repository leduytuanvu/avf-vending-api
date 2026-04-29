# Object storage outage / degraded media plane

Media manifests expose **HTTPS URLs + hashes**—no binary payloads over gRPC (see [`../architecture/media-sync.md`](../architecture/media-sync.md)). When object storage or bucket IAM breaks:

**Config reference (S3-compatible):** `OBJECT_STORAGE_BUCKET` / `S3_BUCKET`, `OBJECT_STORAGE_ACCESS_KEY`, `OBJECT_STORAGE_SECRET_KEY`, optional `OBJECT_STORAGE_ENDPOINT`, `OBJECT_STORAGE_REGION`, optional `OBJECT_STORAGE_PUBLIC_BASE_URL` or `CDN_PUBLIC_BASE_URL` (documented CDN origin; clients still use presigned GET from the API). See `internal/platform/objectstore/config.go`.

## Symptoms

- Catalog/media admin uploads fail (`internal/app/mediaadmin`, artifacts paths).
- Machines receive manifests pointing at URLs that **403/timeout** — kiosk caches mask transient CDN issues until TTL expiry.
- Reporting/media reconciliation dashboards show elevated failures.

## Mitigations

1. Restore bucket policy / KMS / endpoint DNS—prefer restoring reads **before** writes when partially degraded.
2. Invalidate stale CDN caches after recovery — [`product-media-cache-invalidation.md`](product-media-cache-invalidation.md).
3. Communicate ETA to field ops; machines continue vending when cached assets suffice—manifest staleness thresholds remain client-driven.

## Related

- Backup expectations: [`production-backup-restore-dr.md`](production-backup-restore-dr.md).
