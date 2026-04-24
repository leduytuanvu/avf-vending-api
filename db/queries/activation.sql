-- name: InsertMachineActivationCode :one
INSERT INTO machine_activation_codes (
    machine_id,
    organization_id,
    code_hash,
    max_uses,
    uses,
    expires_at,
    notes,
    status
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListMachineActivationCodesForMachine :many
SELECT
    *
FROM
    machine_activation_codes
WHERE
    machine_id = $1
ORDER BY
    created_at DESC;

-- name: GetMachineActivationCodeByIDForOrg :one
SELECT
    *
FROM
    machine_activation_codes
WHERE
    id = $1
    AND organization_id = $2;

-- name: RevokeMachineActivationCode :one
UPDATE machine_activation_codes
SET
    status = 'revoked',
    updated_at = now()
WHERE
    id = $1
    AND machine_id = $2
    AND organization_id = $3
    AND status = 'active'
RETURNING *;

-- name: GetMachineActivationCodeByHashForUpdate :one
SELECT
    *
FROM
    machine_activation_codes
WHERE
    code_hash = $1
FOR UPDATE;

-- name: MarkActivationCodeUsed :one
UPDATE machine_activation_codes
SET
    uses = uses + 1,
    claimed_fingerprint_hash = $2,
    status = CASE
        WHEN uses + 1 >= max_uses THEN 'expired'
        ELSE status
    END,
    updated_at = now()
WHERE
    id = $1
    AND status = 'active'
RETURNING *;
