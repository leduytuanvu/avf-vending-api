-- name: CatalogWriteInsertProduct :one
INSERT INTO products (
    sku,
    barcode,
    name,
    description,
    attrs,
    active,
    category_id,
    brand_id,
    country_of_origin,
    age_restricted,
    allergen_codes,
    nutritional_note
) VALUES (
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

-- name: CatalogWriteUpdateProduct :one
UPDATE products p
SET
    sku = $1,
    barcode = $2,
    name = $3,
    description = $4,
    attrs = $5,
    active = $6,
    category_id = $7,
    brand_id = $8,
    country_of_origin = $9,
    age_restricted = $10,
    allergen_codes = $11,
    nutritional_note = $12,
    updated_at = now()
WHERE p.id = $13
RETURNING *;

-- name: CatalogWriteSetProductActive :one
UPDATE products p
SET active = $1, updated_at = now()
WHERE p.id = $2
RETURNING *;

-- name: CatalogWriteProductInCurrentSlot :one
SELECT EXISTS (
    SELECT 1
    FROM machine_slot_configs msc
    WHERE TRUE
      AND msc.product_id = $1
      AND msc.is_current = true
) AS v;

-- name: CatalogWriteInsertBrand :one
INSERT INTO brands (
    slug,
    name,
    active
)
VALUES (
    $1,
    $2,
    $3
)
RETURNING *;

-- name: CatalogWriteUpdateBrand :one
UPDATE brands b
SET
    slug = $1,
    name = $2,
    active = $3,
    updated_at = now()
WHERE b.id = $4
RETURNING *;

-- name: CatalogWriteInsertCategory :one
INSERT INTO categories (
    slug,
    name,
    parent_id,
    active
)
VALUES (
    $1,
    $2,
    $3,
    $4
)
RETURNING *;

-- name: CatalogWriteUpdateCategory :one
UPDATE categories c
SET
    slug = $1,
    name = $2,
    parent_id = $3,
    active = $4,
    updated_at = now()
WHERE c.id = $5
RETURNING *;

-- name: CatalogWriteDeleteProductTagsForProduct :exec
DELETE FROM product_tags
WHERE product_id = $1;

-- name: CatalogWriteInsertProductTag :exec
INSERT INTO product_tags (product_id, tag_id)
VALUES ($1, $2);

-- name: CatalogWriteInsertTag :one
INSERT INTO tags (
    slug,
    name,
    active
)
VALUES (
    $1,
    $2,
    $3
)
RETURNING *;

-- name: CatalogWriteUpdateTag :one
UPDATE tags t
SET
    slug = $1,
    name = $2,
    active = $3,
    updated_at = now()
WHERE t.id = $4
RETURNING *;

-- name: CatalogWriteUnsetPrimaryImagesForProduct :exec
UPDATE product_images pi
SET is_primary = false
FROM products p
WHERE pi.product_id = p.id
  AND TRUE
  AND p.id = $1
  AND pi.status = 'active';

-- name: CatalogWriteInsertProductImage :one
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
    is_primary
) VALUES (
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
    $11
)
RETURNING *;

-- name: CatalogWriteSetProductPrimaryImage :one
UPDATE products p
SET primary_image_id = $1, updated_at = now()
WHERE p.id = $2
RETURNING *;

-- name: CatalogWriteClearProductPrimaryImage :one
UPDATE products p
SET primary_image_id = NULL, updated_at = now()
WHERE p.id = $1
RETURNING *;

-- name: CatalogWriteGetPrimaryProductImage :one
SELECT *
FROM product_images
WHERE product_id = $1 AND is_primary = true AND status = 'active'
LIMIT 1;

-- name: CatalogWriteArchiveProductImage :one
UPDATE product_images pi
SET
    status = 'archived',
    is_primary = false,
    media_version = media_version + 1,
    updated_at = now()
FROM products p
WHERE pi.id = $1
  AND pi.product_id = p.id
  AND TRUE
  AND p.id = $2
  AND pi.status = 'active'
RETURNING pi.*;

-- name: CatalogWriteUpdateProductImageMetadata :one
UPDATE product_images pi
SET
    sort_order = COALESCE(sqlc.narg('sort_order')::int, sort_order),
    is_primary = COALESCE(sqlc.narg('is_primary')::bool, is_primary),
    alt_text = COALESCE(sqlc.narg('alt_text')::text, alt_text),
    media_version = media_version + 1,
    updated_at = now()
FROM products p
WHERE pi.id = $1
  AND pi.product_id = p.id
  AND TRUE
  AND p.id = $2
  AND pi.status = 'active'
RETURNING pi.*;

-- name: CatalogWriteArchiveAllProductImagesForProduct :exec
UPDATE product_images pi
SET
    status = 'archived',
    is_primary = false,
    media_version = media_version + 1,
    updated_at = now()
FROM products p
WHERE pi.product_id = p.id
  AND TRUE
  AND p.id = $1
  AND pi.status = 'active';

-- name: CatalogWriteProductReferencedPublishedPlanogram :one
SELECT EXISTS (
    SELECT 1
    FROM slots s
    JOIN planograms pg ON pg.id = s.planogram_id
    WHERE TRUE
      AND s.product_id = $1
      AND pg.status = 'published'
) AS v;

-- name: CatalogWriteProductReferencedOpenOrder :one
SELECT EXISTS (
    SELECT 1
    FROM vend_sessions vs
    JOIN orders o ON o.id = vs.order_id
    WHERE TRUE
      AND vs.product_id = $1
      AND o.status IN ('created', 'quoted', 'paid', 'vending')
) AS v;

-- name: CatalogWriteInsertPriceBook :one
INSERT INTO price_books (
    name,
    currency,
    effective_from,
    effective_to,
    is_default,
    active,
    price_book_level,
    site_id,
    machine_id,
    priority
) VALUES (
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

-- name: CatalogWriteUpdatePriceBook :one
UPDATE price_books pb
SET
    name = $1,
    currency = $2,
    effective_from = $3,
    effective_to = $4,
    is_default = $5,
    active = $6,
    price_book_level = $7,
    site_id = $8,
    machine_id = $9,
    priority = $10,
    updated_at = now()
WHERE pb.id = $11
RETURNING *;

-- name: CatalogWriteDeactivatePriceBook :one
UPDATE price_books pb
SET
    active = false,
    updated_at = now()
WHERE pb.id = $1
RETURNING *;

-- name: CatalogWriteUpsertPriceBookItem :one
INSERT INTO price_book_items (
    price_book_id,
    product_id,
    unit_price_minor
) VALUES (
    $1,
    $2,
    $3
)
ON CONFLICT (price_book_id, product_id)
DO UPDATE SET unit_price_minor = EXCLUDED.unit_price_minor
RETURNING *;

-- name: CatalogWriteDeletePriceBookItem :execrows
DELETE FROM price_book_items pbi
WHERE pbi.price_book_id = $1
  AND pbi.product_id = $2;

-- name: CatalogWriteDeleteAllPriceBookItems :exec
DELETE FROM price_book_items pbi
WHERE pbi.price_book_id = $1;

-- name: CatalogWriteInsertPriceBookTarget :one
INSERT INTO price_book_targets (
    price_book_id,
    site_id,
    machine_id
) VALUES (
    $1,
    $2,
    $3
)
RETURNING *;

-- name: CatalogWriteDeletePriceBookTarget :execrows
DELETE FROM price_book_targets pbt
WHERE pbt.id = $1;

-- name: CatalogWriteUpsertProductMediaProjection :one
INSERT INTO product_media (
    id,
    product_id,
    media_role,
    media_type,
    source_type,
    original_object_key,
    thumb_object_key,
    display_object_key,
    thumb_url,
    display_url,
    mime_type,
    width,
    height,
    size_bytes,
    content_hash,
    media_version,
    sort_order,
    status
)
VALUES (
    $1,
    $2,
    $3,
    'image',
    'upload',
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
    0,
    'active'
)
ON CONFLICT (id)
    DO UPDATE SET
        media_role = EXCLUDED.media_role,
        original_object_key = EXCLUDED.original_object_key,
        thumb_object_key = EXCLUDED.thumb_object_key,
        display_object_key = EXCLUDED.display_object_key,
        thumb_url = EXCLUDED.thumb_url,
        display_url = EXCLUDED.display_url,
        mime_type = EXCLUDED.mime_type,
        width = EXCLUDED.width,
        height = EXCLUDED.height,
        size_bytes = EXCLUDED.size_bytes,
        content_hash = EXCLUDED.content_hash,
        media_version = EXCLUDED.media_version,
        status = EXCLUDED.status,
        updated_at = now()
RETURNING *;
