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

-- name: CatalogAdminListPriceBooks :many
SELECT
    pb.id,
    pb.organization_id,
    pb.name,
    pb.currency,
    pb.effective_from,
    pb.effective_to,
    pb.is_default,
    pb.scope_type,
    pb.site_id,
    pb.machine_id,
    pb.priority,
    pb.created_at
FROM price_books pb
WHERE pb.organization_id = $1
ORDER BY pb.effective_from DESC, pb.priority DESC, pb.name
LIMIT $2 OFFSET $3;

-- name: CatalogAdminCountPriceBooks :one
SELECT count(*)::bigint AS cnt
FROM price_books pb
WHERE pb.organization_id = $1;

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
