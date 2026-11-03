-- name: ProductListCount :one
SELECT count(*)::bigint AS cnt
FROM assortments a
WHERE ($1::text = '' OR a.name ILIKE '%' || $1 || '%')
  AND ($2::text = '' OR a.status = $2)
  AND ($3::text = '' OR a.kind = $3);

-- name: ProductListList :many
SELECT
    a.id,
    a.name,
    a.status,
    a.kind,
    a.description,
    a.meta,
    a.created_at,
    a.updated_at,
    (
        SELECT count(*)::bigint
        FROM assortment_items ai
        WHERE ai.assortment_id = a.id
    ) AS item_count
FROM assortments a
WHERE ($1::text = '' OR a.name ILIKE '%' || $1 || '%')
  AND ($2::text = '' OR a.status = $2)
  AND ($3::text = '' OR a.kind = $3)
ORDER BY a.kind ASC, a.name ASC, a.id ASC
LIMIT $4 OFFSET $5;

-- name: ProductListGet :one
SELECT
    a.id,
    a.name,
    a.status,
    a.kind,
    a.description,
    a.meta,
    a.created_at,
    a.updated_at
FROM assortments a
WHERE a.id = $1;

-- name: ProductListInsert :one
INSERT INTO assortments (
    name,
    status,
    kind,
    description
)
VALUES (
    $1,
    $2,
    $3,
    $4
)
RETURNING
    id,
    name,
    status,
    kind,
    description,
    meta,
    created_at,
    updated_at;

-- name: ProductListUpdate :one
UPDATE assortments
SET
    name = $2,
    status = $3,
    description = $4,
    updated_at = now()
WHERE id = $1
RETURNING
    id,
    name,
    status,
    kind,
    description,
    meta,
    created_at,
    updated_at;

-- name: ProductListDeleteItems :exec
DELETE FROM assortment_items
WHERE assortment_id = $1;

-- name: ProductListInsertItem :one
INSERT INTO assortment_items (
    assortment_id,
    product_id,
    sort_order
)
VALUES (
    $1,
    $2,
    $3
)
ON CONFLICT (assortment_id, product_id) DO UPDATE
SET
    sort_order = EXCLUDED.sort_order
RETURNING
    id,
    assortment_id,
    product_id,
    sort_order,
    notes,
    created_at;

-- name: ProductListItems :many
SELECT
    ai.product_id,
    ai.sort_order,
    p.sku,
    p.name,
    p.active,
    p.barcode,
    p.description,
    p.category_id,
    p.brand_id
FROM assortment_items ai
INNER JOIN products p ON p.id = ai.product_id
WHERE ai.assortment_id = $1
ORDER BY ai.sort_order ASC, p.name ASC, p.id ASC;

-- name: ProductListCountItems :one
SELECT count(*)::bigint AS cnt
FROM assortment_items ai
WHERE ai.assortment_id = $1;

-- name: ProductListGetProductBySku :one
SELECT
    p.id,
    p.sku,
    p.name,
    p.active
FROM products p
WHERE lower(p.sku) = lower($1)
LIMIT 1;

-- name: ProductListExistingProductIDs :many
SELECT p.id
FROM products p
WHERE p.id = ANY(sqlc.arg('product_ids')::uuid[]);

-- name: ProductListMachineBindings :many
SELECT
    b.id,
    b.machine_id,
    b.assortment_id,
    b.is_primary,
    b.valid_from,
    b.valid_to,
    a.name AS assortment_name,
    a.kind AS assortment_kind,
    a.status AS assortment_status
FROM machine_assortment_bindings b
INNER JOIN assortments a ON a.id = b.assortment_id
WHERE b.machine_id = $1
  AND b.valid_to IS NULL
ORDER BY b.is_primary DESC, a.kind ASC, a.name ASC;

-- name: ProductListCloseMachineBindings :exec
UPDATE machine_assortment_bindings
SET valid_to = now()
WHERE machine_id = $1
  AND valid_to IS NULL
  AND (
    ($2::bool AND is_primary)
    OR (NOT $2::bool AND NOT is_primary)
  );

-- name: ProductListInsertBinding :exec
INSERT INTO machine_assortment_bindings (
    machine_id,
    assortment_id,
    is_primary,
    valid_from
)
VALUES (
    $1,
    $2,
    $3,
    now()
);

-- name: ProductListVisibleForMachine :many
SELECT
    a.id,
    a.name,
    a.status,
    a.kind,
    a.description,
    a.updated_at,
    COALESCE(b.is_primary, FALSE) AS is_primary,
    (b.id IS NOT NULL)::boolean AS assigned
FROM assortments a
LEFT JOIN machine_assortment_bindings b
    ON b.assortment_id = a.id
    AND b.machine_id = $1
    AND b.valid_to IS NULL
WHERE a.status = 'published'
  AND (
    a.kind = 'common'
    OR b.id IS NOT NULL
  )
ORDER BY a.kind ASC, a.name ASC, a.id ASC;

-- name: ProductListSnapshotRows :many
SELECT
    p.id,
    p.sku,
    p.barcode,
    p.name,
    p.description,
    p.active,
    p.category_id,
    p.brand_id,
    p.updated_at,
    a.id AS assortment_id,
    a.updated_at AS assortment_updated_at
FROM assortments a
INNER JOIN assortment_items ai ON ai.assortment_id = a.id
INNER JOIN products p ON p.id = ai.product_id
WHERE a.id = sqlc.arg('list_id')
  AND p.active = true
  AND (
    cardinality(sqlc.arg('product_ids')::uuid[]) = 0
    OR p.id = ANY(sqlc.arg('product_ids')::uuid[])
  )
ORDER BY ai.sort_order ASC, lower(p.name) ASC, lower(p.sku) ASC, p.id ASC;
