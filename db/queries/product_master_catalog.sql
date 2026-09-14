-- name: ProductMasterCountActiveProducts :one
SELECT count(*)::bigint AS cnt
FROM products p
WHERE p.active = true;

-- name: ProductMasterListActiveProductsPage :many
SELECT
    p.id,
    p.sku,
    p.barcode,
    p.name,
    p.description,
    p.active,
    p.category_id,
    p.brand_id,
    p.updated_at
FROM products p
WHERE p.active = true
ORDER BY lower(p.name) ASC, lower(p.sku) ASC, p.id ASC
LIMIT $1 OFFSET $2;

-- name: ProductMasterDefaultPriceByProductIDs :many
SELECT
    pbi.product_id,
    pbi.unit_price_minor
FROM price_book_items pbi
INNER JOIN price_books pb ON pb.id = pbi.price_book_id
WHERE pb.active = true
  AND pb.is_default = true
  AND pb.price_book_level = 'global'
  AND pbi.product_id = ANY(sqlc.arg(product_ids)::uuid[]);
