-- Runtime product images joined to denormalized `product_media` and optional `media_assets` for authoritative
-- checksums, etag, and deterministic thumb/display keys (presigned HTTPS refresh on gRPC snapshot/manifest).

-- name: RuntimeListProductImagesForProducts :many
SELECT
    pi.id,
    pi.product_id,
    pi.storage_key,
    COALESCE(NULLIF(TRIM(pm.display_url), ''), pi.cdn_url, '') AS cdn_url,
    COALESCE(NULLIF(TRIM(pm.thumb_url), ''), pi.thumb_cdn_url, '') AS thumb_cdn_url,
    CAST(
        COALESCE(
            NULLIF(TRIM(COALESCE(pm.original_url, '')), ''),
            NULLIF(TRIM(COALESCE(ma.original_url, '')), ''),
            ''
        ) AS text
    ) AS original_cdn_url,
    COALESCE(NULLIF(TRIM(pm.content_hash), ''), pi.content_hash) AS content_hash,
    pi.width,
    pi.height,
    pi.mime_type,
    pi.sort_order,
    pi.is_primary,
    pi.created_at,
    COALESCE(pm.media_version, pi.media_version) AS media_version,
    GREATEST(pi.updated_at, pm.updated_at)::timestamptz AS updated_at,
    pi.media_asset_id,
    ma.sha256 AS asset_sha256,
    ma.size_bytes AS asset_size_bytes,
    ma.object_version AS asset_object_version,
    ma.etag AS asset_etag,
    ma.status AS asset_status,
    CAST(
        COALESCE(
            NULLIF(TRIM(COALESCE(pm.original_object_key, '')), ''),
            NULLIF(TRIM(COALESCE(ma.original_object_key, '')), ''),
            ''
        ) AS text
    ) AS original_object_key,
    CAST(
        COALESCE(
            NULLIF(TRIM(COALESCE(pm.thumb_object_key, '')), ''),
            NULLIF(TRIM(COALESCE(ma.thumb_object_key, '')), ''),
            ''
        ) AS text
    ) AS thumb_object_key,
    CAST(
        COALESCE(
            NULLIF(TRIM(COALESCE(pm.display_object_key, '')), ''),
            NULLIF(TRIM(COALESCE(ma.display_object_key, '')), ''),
            ''
        ) AS text
    ) AS display_object_key
FROM
    product_images pi
    INNER JOIN product_media pm ON pm.id = pi.id
        AND pm.product_id = pi.product_id
    LEFT JOIN media_assets ma ON ma.id = pi.media_asset_id
WHERE
    pi.product_id = ANY ($1::uuid[])
    AND pi.status = 'active'
    AND pm.status = 'active'
    AND (
        pi.media_asset_id IS NULL
        OR ma.status = 'ready'
    )
ORDER BY
    pi.product_id,
    pi.is_primary DESC,
    pi.sort_order,
    pi.created_at;

-- name: RuntimeGetProductsByIDs :many
SELECT
    id,
    sku,
    name,
    active,
    attrs
FROM
    products
WHERE
    organization_id = $1
    AND id = ANY ($2::uuid[]);
