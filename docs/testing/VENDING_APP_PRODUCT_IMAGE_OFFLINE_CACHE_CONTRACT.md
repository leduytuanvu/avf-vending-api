# Vending App Product Image Offline Cache Contract

This document defines how the vending machine app should cache product images delivered by the sale catalog API (REST or gRPC). The backend never sends image bytes over MQTT; machines sync catalog metadata and download images over HTTPS when online.

## Catalog image metadata

Each catalog line may include an `image` object (REST) or `primary_media` (gRPC) with:

| Field | Purpose |
|-------|---------|
| `mediaId` | Stable media_assets row id |
| `sourceType` | `external_url` for hosted URLs; `upload` for object-storage pipeline |
| `displayUrl` / `thumbnailUrl` | HTTPS fetch location (may be presigned for uploads) |
| `cacheKey` | Deterministic local cache identity — **prefer this over URL** |
| `version` / `mediaVersion` | Bump when image content or binding changes |
| `contentType` | MIME type for local file extension |
| `offlineRequired` | When true, app should prefetch for offline sale UI |
| `downloadStrategy` | `download_when_online_use_local_when_offline` |

For external URLs, `cacheKey` format:

```text
external-image:<sha256(normalizedUrl)>:v<version>
```

## Sync algorithm

On catalog sync (full snapshot or delta):

1. Iterate all catalog items with non-deleted images.
2. For each image, compute lookup key = `cacheKey` if present, else `mediaId + ":" + version`.
3. If local metadata exists for same key and version, use cached file.
4. If online and missing or stale:
   - Download `displayUrl` (fallback `thumbnailUrl`).
   - Verify size limits; optionally verify `contentHash` / sha256 when provided.
   - Write file atomically (temp + rename).
   - Upsert local DB row: `mediaId`, `productId`, `remoteUrl`, `localPath`, `cacheKey`, `version`, `contentType`, `downloadedAt`, `lastAccessedAt`, `checksumSha256`.
5. On UI render:
   1. Local cached file
   2. Remote URL if online
   3. Product placeholder

## Offline behavior

- Never block product listing because a remote URL is unreachable.
- Use cached image or placeholder when offline.
- MQTT catalog/config commands only signal sync; they do not carry image bytes.

## Example external URL products

- https://adm.avf.vn/storage/photos/1/Product/69f0e277129d9.png
- https://adm.avf.vn/storage/photos/1/Product/69f5c789105c0.png
- https://adm.avf.vn/storage/photos/1/Product/68833a13a45e5.png
