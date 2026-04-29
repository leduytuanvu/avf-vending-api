-- name: CatalogAdminCountProducts :one
SELECT count(*)::bigint AS cnt
FROM products p
WHERE p.organization_id = $1
  AND ($2::text = '' OR p.name ILIKE '%' || $2 || '%' OR p.sku ILIKE '%' || $2 || '%')
  AND (NOT $3 OR p.active = true);

-- name: CatalogAdminListProducts :many
SELECT
    p.id,
    p.organization_id,
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
WHERE p.organization_id = $1
  AND ($4::text = '' OR p.name ILIKE '%' || $4 || '%' OR p.sku ILIKE '%' || $4 || '%')
  AND (NOT $5 OR p.active = true)
ORDER BY p.updated_at DESC, p.id
LIMIT $2 OFFSET $3;

-- name: CatalogAdminGetProduct :one
SELECT
    p.id,
    p.organization_id,
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
WHERE p.organization_id = $1
  AND p.id = $2;

-- name: CatalogAdminGetPrimaryProductImageForOrg :one
SELECT
    pi.*
FROM product_images pi
JOIN products p ON p.id = pi.product_id
INNER JOIN product_media pm ON pm.id = pi.id
    AND pm.product_id = pi.product_id
WHERE p.organization_id = $1
  AND p.id = $2
  AND pi.is_primary = true
  AND pi.status = 'active';

-- name: CatalogAdminListProductImagesForOrg :many
SELECT
    pi.*
FROM product_images pi
JOIN products p ON p.id = pi.product_id
INNER JOIN product_media pm ON pm.id = pi.id
    AND pm.product_id = pi.product_id
WHERE p.organization_id = $1
  AND p.id = $2
  AND ($3::bool OR pi.status = 'active')
ORDER BY pi.is_primary DESC, pi.sort_order ASC, pi.created_at ASC;

-- name: CatalogAdminGetProductImageForOrg :one
SELECT
    pi.*
FROM product_images pi
JOIN products p ON p.id = pi.product_id
INNER JOIN product_media pm ON pm.id = pi.id
    AND pm.product_id = pi.product_id
WHERE p.organization_id = $1
  AND p.id = $2
  AND pi.id = $3;

-- name: CatalogAdminListPriceBooks :many
SELECT
    pb.id,
    pb.organization_id,
    pb.name,
    pb.currency,
    pb.effective_from,
    pb.effective_to,
    pb.is_default,
    pb.active,
    pb.scope_type,
    pb.site_id,
    pb.machine_id,
    pb.priority,
    pb.created_at,
    pb.updated_at
FROM price_books pb
WHERE pb.organization_id = $1
  AND ($4::bool OR pb.active = true)
ORDER BY pb.effective_from DESC, pb.priority DESC, pb.name
LIMIT $2 OFFSET $3;

-- name: CatalogAdminCountPriceBooks :one
SELECT count(*)::bigint AS cnt
FROM price_books pb
WHERE pb.organization_id = $1
  AND ($2::bool OR pb.active = true);

-- name: CatalogAdminGetPriceBook :one
SELECT
    pb.id,
    pb.organization_id,
    pb.name,
    pb.currency,
    pb.effective_from,
    pb.effective_to,
    pb.is_default,
    pb.active,
    pb.scope_type,
    pb.site_id,
    pb.machine_id,
    pb.priority,
    pb.created_at,
    pb.updated_at
FROM price_books pb
WHERE pb.organization_id = $1 AND pb.id = $2;

-- name: CatalogAdminPricingPreviewBooksActiveAt :many
SELECT
    pb.id,
    pb.organization_id,
    pb.name,
    pb.currency,
    pb.effective_from,
    pb.effective_to,
    pb.is_default,
    pb.active,
    pb.scope_type,
    pb.site_id,
    pb.machine_id,
    pb.priority,
    pb.created_at,
    pb.updated_at
FROM price_books pb
WHERE pb.organization_id = $1
  AND pb.active = true
  AND pb.effective_from <= $2::timestamptz
  AND (pb.effective_to IS NULL OR pb.effective_to > $2::timestamptz);

-- name: CatalogAdminListPriceBookTargetsByOrg :many
SELECT
    id,
    organization_id,
    price_book_id,
    site_id,
    machine_id,
    created_at
FROM price_book_targets
WHERE organization_id = $1;

-- name: CatalogAdminListPriceBookTargetsByBook :many
SELECT
    id,
    organization_id,
    price_book_id,
    site_id,
    machine_id,
    created_at
FROM price_book_targets
WHERE organization_id = $1 AND price_book_id = $2
ORDER BY created_at ASC, id ASC;

-- name: CatalogAdminGetPriceBookTarget :one
SELECT
    id,
    organization_id,
    price_book_id,
    site_id,
    machine_id,
    created_at
FROM price_book_targets
WHERE organization_id = $1 AND id = $2;

-- name: CatalogAdminListPriceBookItems :many
SELECT
    id,
    organization_id,
    price_book_id,
    product_id,
    unit_price_minor,
    created_at
FROM price_book_items
WHERE organization_id = $1 AND price_book_id = $2
ORDER BY product_id ASC;

-- name: CatalogAdminGetMachineSiteForOrg :one
SELECT site_id
FROM machines
WHERE organization_id = $1 AND id = $2;

-- name: CatalogAdminPriceBookItemsForPreview :many
SELECT
    pbi.price_book_id,
    pbi.product_id,
    pbi.unit_price_minor
FROM price_book_items pbi
WHERE pbi.organization_id = $1
  AND pbi.price_book_id = ANY($2::uuid[])
  AND pbi.product_id = ANY($3::uuid[]);

-- name: CatalogAdminCountProductsInOrgByIDs :one
SELECT count(*)::bigint
FROM products p
WHERE p.organization_id = $1
  AND p.id = ANY($2::uuid[]);

-- name: CatalogAdminListPlanograms :many
SELECT
    pg.id,
    pg.organization_id,
    pg.name,
    pg.revision,
    pg.status,
    pg.meta,
    pg.created_at
FROM planograms pg
WHERE pg.organization_id = $1
ORDER BY pg.created_at DESC, pg.name, pg.revision DESC
LIMIT $2 OFFSET $3;

-- name: CatalogAdminCountPlanograms :one
SELECT count(*)::bigint AS cnt
FROM planograms pg
WHERE pg.organization_id = $1;

-- name: CatalogAdminGetPlanogram :one
SELECT
    pg.id,
    pg.organization_id,
    pg.name,
    pg.revision,
    pg.status,
    pg.meta,
    pg.created_at
FROM planograms pg
WHERE pg.organization_id = $1
  AND pg.id = $2;

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
WHERE b.organization_id = $1
ORDER BY b.name ASC, b.id
LIMIT $2 OFFSET $3;

-- name: CatalogAdminCountBrands :one
SELECT count(*)::bigint
FROM brands b
WHERE b.organization_id = $1;

-- name: CatalogAdminGetBrand :one
SELECT *
FROM brands b
WHERE b.organization_id = $1 AND b.id = $2;

-- name: CatalogAdminListCategories :many
SELECT *
FROM categories c
WHERE c.organization_id = $1
ORDER BY c.name ASC, c.id
LIMIT $2 OFFSET $3;

-- name: CatalogAdminCountCategories :one
SELECT count(*)::bigint
FROM categories c
WHERE c.organization_id = $1;

-- name: CatalogAdminGetCategory :one
SELECT *
FROM categories c
WHERE c.organization_id = $1 AND c.id = $2;

-- name: CatalogAdminListTags :many
SELECT *
FROM tags t
WHERE t.organization_id = $1
ORDER BY t.name ASC, t.id
LIMIT $2 OFFSET $3;

-- name: CatalogAdminCountTags :one
SELECT count(*)::bigint
FROM tags t
WHERE t.organization_id = $1;

-- name: CatalogAdminGetTag :one
SELECT *
FROM tags t
WHERE t.organization_id = $1 AND t.id = $2;

-- name: CatalogAdminListProductMediumRowsForProduct :many
SELECT pm.*
FROM product_media pm
WHERE pm.organization_id = $1
    AND pm.product_id = $2
ORDER BY pm.sort_order ASC, pm.created_at ASC;

-- name: CatalogAdminGetProductMediumForOrgProductImage :one
SELECT pm.*
FROM product_media pm
JOIN products p ON p.id = pm.product_id
WHERE p.organization_id = $1
    AND pm.product_id = $2
    AND pm.id = $3;
