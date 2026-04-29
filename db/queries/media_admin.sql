-- name: MediaAdminInsertAsset :one
INSERT INTO media_assets (
    organization_id,
    kind,
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
    $10
)
RETURNING *;

-- name: MediaAdminGetAssetForOrg :one
SELECT
    *
FROM
    media_assets
WHERE
    id = $1
    AND organization_id = $2;

-- name: MediaAdminListAssetsForOrg :many
SELECT
    *
FROM
    media_assets
WHERE
    organization_id = $1
    AND status != 'deleted'
ORDER BY
    created_at DESC
LIMIT $2 OFFSET $3;

-- name: MediaAdminCountAssetsForOrg :one
SELECT
    count(*)::bigint
FROM
    media_assets
WHERE
    organization_id = $1
    AND status != 'deleted';

-- name: MediaAdminMarkAssetFailed :one
UPDATE media_assets
SET
    status = 'failed',
    failed_reason = $3,
    updated_at = now()
WHERE
    id = $1
    AND organization_id = $2
RETURNING *;

-- name: MediaAdminDeletePendingAsset :execrows
DELETE FROM media_assets
WHERE
    id = $1
    AND organization_id = $2
    AND status = 'pending';

-- name: MediaAdminUpdateAssetReady :one
UPDATE media_assets
SET
    mime_type = $3,
    size_bytes = $4,
    sha256 = $5,
    width = $6,
    height = $7,
    object_version = object_version + 1,
    etag = $8,
    status = 'ready',
    updated_at = now()
WHERE
    id = $1
    AND organization_id = $2
    AND status IN ('pending', 'processing')
RETURNING *;

-- name: MediaAdminSetAssetStatus :one
UPDATE media_assets
SET
    status = $3,
    updated_at = now()
WHERE
    id = $1
    AND organization_id = $2
RETURNING *;

-- name: MediaAdminSoftDeleteAsset :one
UPDATE media_assets
SET
    status = 'deleted',
    updated_at = now()
WHERE
    id = $1
    AND organization_id = $2
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
    pi.is_primary,
    p.organization_id
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
    p.organization_id = $1
    AND pi.product_id = $2
    AND pi.media_asset_id = $3
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
