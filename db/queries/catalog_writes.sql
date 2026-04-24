-- name: CatalogWriteInsertProduct :one
INSERT INTO products (
    organization_id, sku, barcode, name, description, attrs, active,
    category_id, brand_id, country_of_origin, age_restricted, allergen_codes, nutritional_note
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
)
RETURNING *;

-- name: CatalogWriteUpdateProduct :one
UPDATE products p
SET
    sku = $3,
    barcode = $4,
    name = $5,
    description = $6,
    attrs = $7,
    active = $8,
    category_id = $9,
    brand_id = $10,
    country_of_origin = $11,
    age_restricted = $12,
    allergen_codes = $13,
    nutritional_note = $14,
    updated_at = now()
WHERE p.organization_id = $1 AND p.id = $2
RETURNING *;

-- name: CatalogWriteSetProductActive :one
UPDATE products p
SET active = $3, updated_at = now()
WHERE p.organization_id = $1 AND p.id = $2
RETURNING *;

-- name: CatalogWriteProductInCurrentSlot :one
SELECT EXISTS (
    SELECT 1
    FROM machine_slot_configs msc
    WHERE msc.organization_id = $1
      AND msc.product_id = $2
      AND msc.is_current = true
) AS v;

-- name: CatalogWriteInsertBrand :one
INSERT INTO brands (organization_id, slug, name, active)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: CatalogWriteUpdateBrand :one
UPDATE brands b
SET
    slug = $3,
    name = $4,
    active = $5,
    updated_at = now()
WHERE b.organization_id = $1 AND b.id = $2
RETURNING *;

-- name: CatalogWriteInsertCategory :one
INSERT INTO categories (organization_id, slug, name, parent_id, active)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: CatalogWriteUpdateCategory :one
UPDATE categories c
SET
    slug = $3,
    name = $4,
    parent_id = $5,
    active = $6,
    updated_at = now()
WHERE c.organization_id = $1 AND c.id = $2
RETURNING *;

-- name: CatalogWriteInsertTag :one
INSERT INTO tags (organization_id, slug, name, active)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: CatalogWriteUpdateTag :one
UPDATE tags t
SET
    slug = $3,
    name = $4,
    active = $5,
    updated_at = now()
WHERE t.organization_id = $1 AND t.id = $2
RETURNING *;

-- name: CatalogWriteUnsetPrimaryImagesForProduct :exec
UPDATE product_images pi
SET is_primary = false
FROM products p
WHERE pi.product_id = p.id
  AND p.organization_id = $1
  AND p.id = $2;

-- name: CatalogWriteInsertProductImage :one
INSERT INTO product_images (
    product_id, storage_key, cdn_url, thumb_cdn_url, content_hash,
    width, height, mime_type, alt_text, sort_order, is_primary
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: CatalogWriteSetProductPrimaryImage :one
UPDATE products p
SET primary_image_id = $3, updated_at = now()
WHERE p.organization_id = $1 AND p.id = $2
RETURNING *;

-- name: CatalogWriteClearProductPrimaryImage :one
UPDATE products p
SET primary_image_id = NULL, updated_at = now()
WHERE p.organization_id = $1 AND p.id = $2
RETURNING *;

-- name: CatalogWriteGetPrimaryProductImage :one
SELECT *
FROM product_images
WHERE product_id = $1 AND is_primary = true
LIMIT 1;

-- name: CatalogWriteDeleteProductImage :exec
DELETE FROM product_images pi
USING products p
WHERE pi.id = $3
  AND pi.product_id = p.id
  AND p.organization_id = $1
  AND p.id = $2;

-- name: CatalogWriteProductReferencedPublishedPlanogram :one
SELECT EXISTS (
    SELECT 1
    FROM slots s
    JOIN planograms pg ON pg.id = s.planogram_id
    WHERE pg.organization_id = $1
      AND s.product_id = $2
      AND pg.status = 'published'
) AS v;

-- name: CatalogWriteProductReferencedOpenOrder :one
SELECT EXISTS (
    SELECT 1
    FROM vend_sessions vs
    JOIN orders o ON o.id = vs.order_id
    WHERE o.organization_id = $1
      AND vs.product_id = $2
      AND o.status IN ('created', 'quoted', 'paid', 'vending')
) AS v;
