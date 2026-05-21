-- name: MediaAdminGetAssetByOriginalURL :one
SELECT
    *
FROM
    media_assets
WHERE
    source_type = 'external'
    AND lower(trim(original_url)) = lower(trim($1::text))
    AND status NOT IN ('deleted', 'archived')
LIMIT
    1;

-- name: MediaAdminInsertAsset :one
INSERT INTO media_assets (
    id,
    kind,
    original_filename,
    object_key,
    original_object_key,
    thumb_object_key,
    display_object_key,
    source_type,
    original_url,
    mime_type,
    created_by,
    status
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    $10,
    $11,
    $12
)
RETURNING *;

-- name: MediaAdminInsertCloudinaryAsset :one
INSERT INTO media_assets (
    id,
    kind,
    original_filename,
    object_key,
    original_object_key,
    thumb_object_key,
    display_object_key,
    source_type,
    storage_provider,
    provider_public_id,
    provider_asset_id,
    original_url,
    mime_type,
    size_bytes,
    sha256,
    width,
    height,
    created_by,
    status
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    $10,
    $11,
    $12,
    $13,
    $14,
    $15,
    $16,
    $17,
    $18,
    $19
)
RETURNING *;

-- name: MediaAdminDeleteVariantsForAsset :exec
DELETE FROM media_variants
WHERE media_asset_id = $1;

-- name: MediaAdminInsertMediaVariant :one
INSERT INTO media_variants (
    media_asset_id,
    variant,
    object_key,
    mime_type,
    width,
    height,
    size_bytes,
    sha256,
    version
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9
)
RETURNING *;

-- name: MediaAdminListVariantsForAssets :many
SELECT
    *
FROM
    media_variants
WHERE
    media_asset_id = ANY ($1::uuid[])
ORDER BY
    media_asset_id ASC,
    variant ASC;

-- name: MediaAdminEnsureCanonicalObjectKey :one
UPDATE media_assets
SET
    object_key = COALESCE(NULLIF(TRIM(object_key), ''), original_object_key),
    updated_at = now()
WHERE
    id = $1
RETURNING *;

-- name: MediaAdminListAssetsByIDs :many
SELECT
    *
FROM
    media_assets
WHERE
    id = ANY ($1::uuid[])
    AND status NOT IN ('deleted', 'archived');

-- name: MediaAdminGetAssetForOrg :one
SELECT
    *
FROM
    media_assets
WHERE
    id = $1
    AND TRUE;

-- name: MediaAdminListAssetsForOrg :many
SELECT
    *
FROM
    media_assets
WHERE
    status != 'deleted'
ORDER BY
    created_at DESC
LIMIT $1 OFFSET $2;

-- name: MediaAdminCountAssetsForOrg :one
SELECT
    count(*)::bigint
FROM
    media_assets
WHERE
    status != 'deleted';

-- name: MediaAdminMarkAssetFailed :one
UPDATE media_assets
SET
    status = 'failed',
    failed_reason = $1,
    updated_at = now()
WHERE
    id = $2
    AND TRUE
RETURNING *;

-- name: MediaAdminDeletePendingAsset :execrows
DELETE FROM media_assets
WHERE
    id = $1
    AND TRUE
    AND status = 'pending';

-- name: MediaAdminUpdateAssetReady :one
UPDATE media_assets
SET
    mime_type = $1,
    size_bytes = $2,
    sha256 = $3,
    width = $4,
    height = $5,
    object_version = object_version + 1,
    etag = $6,
    status = 'ready',
    updated_at = now()
WHERE
    id = $7
    AND TRUE
    AND status IN ('pending', 'processing')
RETURNING *;

-- name: MediaAdminSetAssetStatus :one
UPDATE media_assets
SET
    status = $1,
    updated_at = now()
WHERE
    id = $2
    AND TRUE
RETURNING *;

-- name: MediaAdminSoftDeleteAsset :one
UPDATE media_assets
SET
    status = 'deleted',
    updated_at = now()
WHERE
    id = $1
    AND TRUE
    AND status != 'deleted'
RETURNING *;

-- name: MediaAdminClearProductImageMediaBinding :exec
UPDATE product_images
SET
    media_asset_id = NULL
WHERE
    media_asset_id = $1;

-- name: MediaAdminArchiveProductImagesForMediaAsset :exec
UPDATE product_images
SET
    status = 'archived',
    is_primary = false,
    media_version = media_version + 1,
    updated_at = now()
WHERE
    media_asset_id = $1
    AND status = 'active';

-- name: MediaAdminCountProductBindingsForAsset :one
SELECT
    count(*)::bigint
FROM
    product_images
WHERE
    media_asset_id = $1;

-- name: MediaAdminListProductImagesForAsset :many
SELECT
    pi.id,
    pi.product_id,
    pi.is_primary
FROM
    product_images pi
    INNER JOIN products p ON p.id = pi.product_id
WHERE
    pi.media_asset_id = $1;

-- name: MediaAdminFindProductImageBinding :one
SELECT
    pi.id
FROM
    product_images pi
    INNER JOIN products p ON p.id = pi.product_id
WHERE
    pi.product_id = $1
    AND pi.media_asset_id = $2
    AND pi.status = 'active'
LIMIT
    1;

-- name: CatalogWriteInsertProductImageWithMedia :one
INSERT INTO product_images (
    product_id,
    storage_key,
    cdn_url,
    thumb_cdn_url,
    content_hash,
    width,
    height,
    mime_type,
    alt_text,
    sort_order,
    is_primary,
    media_asset_id
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    $10,
    $11,
    $12
)
RETURNING *;
