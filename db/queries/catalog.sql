-- name: GetProductByID :one
SELECT *
FROM products
WHERE id = $1;

-- name: ListProductsByOrganization :many
SELECT *
FROM products
WHERE organization_id = $1
ORDER BY sku;
