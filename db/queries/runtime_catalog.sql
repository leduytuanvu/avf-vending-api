-- name: RuntimeListProductImagesForProducts :many
SELECT
    id,
    product_id,
    storage_key,
    cdn_url,
    thumb_cdn_url,
    content_hash,
    width,
    height,
    mime_type,
    sort_order,
    is_primary,
    created_at
FROM
    product_images
WHERE
    product_id = ANY ($1::uuid[])
ORDER BY
    product_id,
    is_primary DESC,
    sort_order,
    created_at;

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
