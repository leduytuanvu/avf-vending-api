-- name: CatalogAdminCountProducts :one
SELECT count(*)::bigint AS cnt
FROM products p
WHERE TRUE
  AND ($1::text = '' OR p.name ILIKE '%' || $1 || '%' OR p.sku ILIKE '%' || $1 || '%')
  AND (NOT $2 OR p.active = true);

-- name: CatalogAdminListProducts :many
SELECT
    p.id,
    p.sku,
    p.barcode,
    p.name,
    p.description,
    p.active,
    p.category_id,
    p.brand_id,
    p.created_at,
    p.updated_at
FROM products p
WHERE TRUE
  AND ($1::text = '' OR p.name ILIKE '%' || $1 || '%' OR p.sku ILIKE '%' || $1 || '%')
  AND (NOT $2 OR p.active = true)
ORDER BY p.updated_at DESC, p.id
LIMIT $3 OFFSET $4;

-- name: CatalogAdminGetProduct :one
SELECT
    p.id,
    p.sku,
    p.barcode,
    p.name,
    p.description,
    p.attrs,
    p.active,
    p.category_id,
    p.brand_id,
    p.primary_image_id,
    p.country_of_origin,
    p.age_restricted,
    p.allergen_codes,
    p.nutritional_note,
    p.created_at,
    p.updated_at
FROM products p
WHERE TRUE
  AND p.id = $1;

-- name: CatalogAdminGetPrimaryProductImageForOrg :one
SELECT
    pi.*
FROM product_images pi
JOIN products p ON p.id = pi.product_id
INNER JOIN product_media pm ON pm.id = pi.id
    AND pm.product_id = pi.product_id
WHERE TRUE
  AND p.id = $1
  AND pi.is_primary = true
  AND pi.status = 'active';

-- name: CatalogAdminListProductImagesForOrg :many
SELECT
    pi.*
FROM product_images pi
JOIN products p ON p.id = pi.product_id
INNER JOIN product_media pm ON pm.id = pi.id
    AND pm.product_id = pi.product_id
WHERE TRUE
  AND p.id = $1
  AND ($2::bool OR pi.status = 'active')
ORDER BY pi.is_primary DESC, pi.sort_order ASC, pi.created_at ASC;

-- name: CatalogAdminGetProductImageForOrg :one
SELECT
    pi.*
FROM product_images pi
JOIN products p ON p.id = pi.product_id
INNER JOIN product_media pm ON pm.id = pi.id
    AND pm.product_id = pi.product_id
WHERE TRUE
  AND p.id = $1
  AND pi.id = $2;

-- name: CatalogAdminListPriceBooks :many
SELECT
    pb.id,
    pb.name,
    pb.currency,
    pb.effective_from,
    pb.effective_to,
    pb.is_default,
    pb.active,
    pb.price_book_level,
    pb.site_id,
    pb.machine_id,
    pb.priority,
    pb.created_at,
    pb.updated_at
FROM price_books pb
WHERE TRUE
  AND ($1::bool OR pb.active = true)
ORDER BY pb.effective_from DESC, pb.priority DESC, pb.name
LIMIT $2 OFFSET $3;

-- name: CatalogAdminCountPriceBooks :one
SELECT count(*)::bigint AS cnt
FROM price_books pb
WHERE TRUE
  AND ($1::bool OR pb.active = true);

-- name: CatalogAdminGetPriceBook :one
SELECT
    pb.id,
    pb.name,
    pb.currency,
    pb.effective_from,
    pb.effective_to,
    pb.is_default,
    pb.active,
    pb.price_book_level,
    pb.site_id,
    pb.machine_id,
    pb.priority,
    pb.created_at,
    pb.updated_at
FROM price_books pb
WHERE pb.id = $1;

-- name: CatalogAdminPricingPreviewBooksActiveAt :many
SELECT
    pb.id,
    pb.name,
    pb.currency,
    pb.effective_from,
    pb.effective_to,
    pb.is_default,
    pb.active,
    pb.price_book_level,
    pb.site_id,
    pb.machine_id,
    pb.priority,
    pb.created_at,
    pb.updated_at
FROM price_books pb
WHERE TRUE
  AND pb.active = true
  AND pb.effective_from <= $1::timestamptz
  AND (pb.effective_to IS NULL OR pb.effective_to > $1::timestamptz);

-- name: CatalogAdminListPriceBookTargetsByOrg :many
SELECT
    id,
    price_book_id,
    site_id,
    machine_id,
    created_at
FROM price_book_targets
WHERE TRUE;

-- name: CatalogAdminListPriceBookTargetsByBook :many
SELECT
    id,
    price_book_id,
    site_id,
    machine_id,
    created_at
FROM price_book_targets
WHERE price_book_id = $1
ORDER BY created_at ASC, id ASC;

-- name: CatalogAdminGetPriceBookTarget :one
SELECT
    id,
    price_book_id,
    site_id,
    machine_id,
    created_at
FROM price_book_targets
WHERE id = $1;

-- name: CatalogAdminListPriceBookItems :many
SELECT
    id,
    price_book_id,
    product_id,
    unit_price_minor,
    created_at
FROM price_book_items
WHERE price_book_id = $1
ORDER BY product_id ASC;

-- name: CatalogAdminGetMachineSiteForOrg :one
SELECT site_id
FROM machines
WHERE id = $1;

-- name: CatalogAdminPriceBookItemsForPreview :many
SELECT
    pbi.price_book_id,
    pbi.product_id,
    pbi.unit_price_minor
FROM price_book_items pbi
WHERE TRUE
  AND pbi.price_book_id = ANY($1::uuid[])
  AND pbi.product_id = ANY($2::uuid[]);

-- name: CatalogAdminCountProductsInOrgByIDs :one
SELECT count(*)::bigint
FROM products p
WHERE TRUE
  AND p.id = ANY($1::uuid[]);

-- name: CatalogAdminListPlanograms :many
SELECT
    pg.id,
    pg.name,
    pg.revision,
    pg.status,
    pg.meta,
    pg.created_at
FROM planograms pg
WHERE TRUE
ORDER BY pg.created_at DESC, pg.name, pg.revision DESC
LIMIT $1 OFFSET $2;

-- name: CatalogAdminCountPlanograms :one
SELECT count(*)::bigint AS cnt
FROM planograms pg
WHERE TRUE;

-- name: CatalogAdminGetPlanogram :one
SELECT
    pg.id,
    pg.name,
    pg.revision,
    pg.status,
    pg.meta,
    pg.created_at
FROM planograms pg
WHERE TRUE
  AND pg.id = $1;

-- name: CatalogAdminListSlotsByPlanogram :many
SELECT
    s.id,
    s.planogram_id,
    s.slot_index,
    s.product_id,
    s.max_quantity,
    s.created_at,
    pr.sku AS product_sku,
    pr.name AS product_name
FROM slots s
LEFT JOIN products pr ON pr.id = s.product_id
WHERE s.planogram_id = $1
ORDER BY s.slot_index ASC;

-- name: CatalogAdminListBrands :many
SELECT *
FROM brands b
WHERE TRUE
ORDER BY b.name ASC, b.id
LIMIT $1 OFFSET $2;

-- name: CatalogAdminCountBrands :one
SELECT count(*)::bigint
FROM brands b
WHERE TRUE;

-- name: CatalogAdminGetBrand :one
SELECT *
FROM brands b
WHERE b.id = $1;

-- name: CatalogAdminListCategories :many
SELECT *
FROM categories c
WHERE TRUE
ORDER BY c.name ASC, c.id
LIMIT $1 OFFSET $2;

-- name: CatalogAdminCountCategories :one
SELECT count(*)::bigint
FROM categories c
WHERE TRUE;

-- name: CatalogAdminGetCategory :one
SELECT *
FROM categories c
WHERE c.id = $1;

-- name: CatalogAdminListTags :many
SELECT *
FROM tags t
WHERE TRUE
ORDER BY t.name ASC, t.id
LIMIT $1 OFFSET $2;

-- name: CatalogAdminCountTags :one
SELECT count(*)::bigint
FROM tags t
WHERE TRUE;

-- name: CatalogAdminGetTag :one
SELECT *
FROM tags t
WHERE t.id = $1;

-- name: CatalogAdminListProductTagsForProducts :many
SELECT
    pt.product_id,
    t.id,
    t.slug,
    t.name,
    t.active,
    t.created_at,
    t.updated_at
FROM product_tags pt
INNER JOIN tags t ON t.id = pt.tag_id
WHERE pt.product_id = ANY($1::uuid[])
ORDER BY pt.product_id ASC, t.name ASC, t.id ASC;

-- name: CatalogAdminCountTagsMatchingIDs :one
SELECT count(*)::bigint
FROM tags
WHERE id = ANY($1::uuid[]);

-- name: CatalogAdminListPrimaryMediaAssetIDsForProducts :many
SELECT
    p.id AS product_id,
    pi.media_asset_id AS media_asset_id
FROM
    products p
    INNER JOIN product_images pi ON pi.product_id = p.id
        AND pi.id = p.primary_image_id
        AND pi.status = 'active'
WHERE
    p.id = ANY ($1::uuid[])
    AND pi.media_asset_id IS NOT NULL;

-- name: CatalogAdminListProductMediumRowsForProduct :many
SELECT pm.*
FROM product_media pm
WHERE TRUE
    AND pm.product_id = $1
ORDER BY pm.sort_order ASC, pm.created_at ASC;

-- name: CatalogAdminGetProductMediumForOrgProductImage :one
SELECT pm.*
FROM product_media pm
JOIN products p ON p.id = pm.product_id
WHERE TRUE
    AND pm.product_id = $1
    AND pm.id = $2;
