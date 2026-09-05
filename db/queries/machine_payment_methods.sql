-- name: ListMachinePaymentMethods :many
SELECT
    id,
    machine_id,
    method_key,
    enabled,
    sort_order,
    created_at,
    updated_at
FROM
    machine_payment_methods
WHERE
    machine_id = $1
ORDER BY
    sort_order ASC,
    method_key ASC;

-- name: DeleteMachinePaymentMethods :exec
DELETE FROM machine_payment_methods
WHERE
    machine_id = $1;

-- name: InsertMachinePaymentMethod :one
INSERT INTO machine_payment_methods (
    machine_id,
    method_key,
    enabled,
    sort_order
)
VALUES (
    $1,
    $2,
    $3,
    $4
)
RETURNING *;
