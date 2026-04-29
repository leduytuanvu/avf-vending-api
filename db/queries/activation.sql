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

-- name: ListMachineActivationCodesForOrganization :many
SELECT
    *
FROM
    machine_activation_codes
WHERE
    organization_id = $1
ORDER BY
    created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountMachineActivationCodesForOrganization :one
SELECT
    count(*)::bigint AS cnt
FROM
    machine_activation_codes
WHERE
    organization_id = $1;

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

-- name: RevokeMachineActivationCodeForOrganization :one
UPDATE machine_activation_codes
SET
    status = 'revoked',
    updated_at = now()
WHERE
    id = $1
    AND organization_id = $2
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

-- name: CountSucceededMachineActivationClaims :one
SELECT
    COUNT(*)::bigint AS cnt
FROM
    machine_activation_claims
WHERE
    activation_code_id = $1
    AND result = 'succeeded';

-- name: GetSucceededMachineActivationClaimByCodeAndFingerprint :one
SELECT
    *
FROM
    machine_activation_claims
WHERE
    activation_code_id = $1
    AND fingerprint_hash = $2
    AND result = 'succeeded';

-- name: InsertMachineActivationClaim :one
INSERT INTO machine_activation_claims (
    activation_code_id,
    organization_id,
    machine_id,
    fingerprint_hash,
    ip_address,
    user_agent,
    result,
    failure_reason
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: RefreshMachineActivationCodeAggregate :one
WITH cnt AS (
    SELECT
        COUNT(*)::int AS c
    FROM
        machine_activation_claims
    WHERE
        activation_code_id = $1
        AND result = 'succeeded'
)
UPDATE
    machine_activation_codes AS mac
SET
    uses = cnt.c,
    claimed_fingerprint_hash = $2,
    status = CASE
        WHEN cnt.c >= mac.max_uses THEN 'expired'::text
        ELSE mac.status
    END,
    updated_at = now()
FROM
    cnt
WHERE
    mac.id = $1
RETURNING
    mac.*;
