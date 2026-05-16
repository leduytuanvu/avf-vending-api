-- name: GetProductByID :one
SELECT *
FROM products
WHERE id = $1;

-- name: ListProductsByCompany :many
SELECT *
FROM products
WHERE TRUE
ORDER BY sku;
