# Phase 3 — Media storage and variant abstraction

## Storage behavior

- **PostgreSQL** holds `media_assets` and `media_variants` metadata only (status, keys, `sha256`, `size_bytes`, dimensions, MIME, versions). No image bytes in the database.
- **Object storage** uses the existing `objectstore` abstraction (`Deps.Store`). Upload keys come from init-upload; complete-upload reads the object (Head/stream), validates size and type, computes integrity fields, and persists variant rows.
- **Offline cache identity** is **`(mediaId, variant, sha256, version)`** — not `downloadUrl`. Manifest entries expose all of these plus `downloadUrl` for fetch only.

## Variant behavior

- **Production path**: `WebPVariantGenerator` produces distinct **`original`**, **`thumb`**, and **`display`** objects (real resized/transcoded WebP when imaging pipeline is available).
- **Passthrough / tests**: `PassthroughVariantGenerator` copies original bytes into thumb/display keys. **TODO(phase3)**: not suitable for production cache differentiation — thumb/display SHA256 match the original; documented in code comments.
- Tests assert manifest **`original`** integrity (`sha256`, `version`, `sizeBytes`, `downloadUrl`) without claiming thumb/display were uniquely generated unless the configured generator actually produces distinct artifacts.

## Versioning

- Asset **`object_version`** (and related flows) increments when media content is finalized or replaced so clients can invalidate caches without treating URLs as stable identities.
- Per-variant **`version`** in `media_variants` is included in `MediaManifestEntry`.

## Security

- Non-empty Head **Content-Type** must be **`image/*`** (after normalization) or **`application/octet-stream`** / empty for sniff-at-complete behavior; non-image types are rejected at complete.
- Upload size is capped by **`MaxUploadBytes`** (configured on `mediaadmin.Service`).
- Signed URLs are obtained for manifests and binds but must not be logged as secrets (audit records use ids/keys, not presigned strings).

## API surface

- **`MediaService`** (`internal/app/mediaadmin/mediaservice.go`): `InitUpload`, `CompleteUpload` / `CompleteUploadWithOptions`, **`MediaManifest`** (ready-only), compile-time check `var _ MediaService = (*Service)(nil)`.

## Tests (integration)

See `TestPrimaryMedia_phase3_manifest_bind_complete_validation` in `internal/app/catalogadmin/writes_primary_media_integration_test.go`: pending after init, complete → ready, non-image Head rejected, manifest fields, manifest gated on ready, bind/create active blocked until ready, ready bind succeeds.
