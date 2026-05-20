# Phase 2 — Admin REST contract (primary media + tags)

## Media pipeline

- **POST /v1/admin/media/uploads/init** — camelCase body (`filename`, `contentType`, `purpose`); returns `mediaId`, `uploadUrl`, `objectKey`, `status`.
- **POST /v1/admin/media/uploads/{mediaId}/complete** — optional JSON `{ sizeBytes, sha256, contentType }`; validates against stored object when provided.
- Legacy **POST /v1/admin/media/uploads** (snake_case `content_type`) remains for backward compatibility.

Upload completion returns **V1AdminMediaUploadCompleteResponseV2**: `id`, `status`, `variants[]` with presigned `downloadUrl` per variant.

### Processing note

When object storage and the WebP generator are enabled, **original**, **thumb**, and **display** variants are produced and persisted to `media_variants`. If the pipeline is unavailable in an environment, handlers return **503** / capability errors rather than fabricating variant rows.

## Products

- Mutations accept optional **`tagIds`**, optional **`primaryMediaId`**, optional **`status`** (alias for active/inactive semantics).
- **Active / sellable** products must have primary media after create/update (`invalid_argument` when violated).
- **`primaryMediaId`** must reference **ready** `media_assets` (enforced in the media binder).

## Tests

See `internal/app/catalogadmin/writes_primary_media_integration_test.go` (requires `TEST_DATABASE_URL`) and updated `writes_product_tags_integration_test.go`.
