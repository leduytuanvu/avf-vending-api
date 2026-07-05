-- name: InsertMachineActivationCode :one
INSERT INTO machine_activation_codes (
    machine_id,
    code_hash,
    max_uses,
    uses,
    expires_at,
    notes,
    status
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7
)
RETURNING *;

-- name: ListMachineActivationCodesForMachine :many
SELECT
    mac.id,
    mac.machine_id,
    mac.code_hash,
    mac.max_uses,
    mac.uses,
    mac.expires_at,
    mac.notes,
    mac.status,
    mac.claimed_fingerprint_hash,
    mac.created_at,
    mac.updated_at,
    m.code AS machine_code
FROM
    machine_activation_codes mac
    INNER JOIN machines m ON m.id = mac.machine_id
WHERE
    mac.machine_id = $1
ORDER BY
    mac.created_at DESC;

-- name: ListMachineActivationCodesPaged :many
SELECT
    mac.id,
    mac.machine_id,
    mac.code_hash,
    mac.max_uses,
    mac.uses,
    mac.expires_at,
    mac.notes,
    mac.status,
    mac.claimed_fingerprint_hash,
    mac.created_at,
    mac.updated_at,
    m.code AS machine_code
FROM
    machine_activation_codes mac
    INNER JOIN machines m ON m.id = mac.machine_id
ORDER BY
    mac.created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountMachineActivationCodesAll :one
SELECT
    count(*)::bigint AS cnt
FROM
    machine_activation_codes;

-- name: GetMachineActivationCodeByIDForOrg :one
SELECT
    *
FROM
    machine_activation_codes
WHERE
    id = $1
    AND TRUE;

-- name: RevokeMachineActivationCode :one
UPDATE machine_activation_codes
SET
    status = 'revoked',
    updated_at = now()
WHERE
    id = $1
    AND machine_id = $2
    AND TRUE
    AND status = 'active'
RETURNING *;

-- name: RevokeMachineActivationCodeActive :one
UPDATE machine_activation_codes
SET
    status = 'revoked',
    updated_at = now()
WHERE
    id = $1
    AND TRUE
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
    machine_id,
    fingerprint_hash,
    ip_address,
    user_agent,
    result,
    failure_reason
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7
)
RETURNING *;

-- name: InsertMachineActivationClaimExtended :one
INSERT INTO machine_activation_claims (
    activation_code_id,
    machine_id,
    fingerprint_hash,
    ip_address,
    user_agent,
    result,
    failure_reason,
    activated_by_account_id,
    operator_session_id,
    request_id,
    correlation_id,
    app_version,
    boot_id,
    device_serial,
    reason,
    activation_source
)
VALUES (
    sqlc.narg('activation_code_id'),
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    $10,
    $11,
    $12,
    $13,
    $14,
    $15
)
RETURNING *;

-- name: GetLastSucceededActivationClaimForMachine :one
SELECT
    *
FROM
    machine_activation_claims
WHERE
    machine_id = $1
    AND result = 'succeeded'
ORDER BY
    claimed_at DESC
LIMIT
    1;

-- name: ListMachineActivationClaimsForMachine :many
SELECT
    *
FROM
    machine_activation_claims
WHERE
    machine_id = $1
ORDER BY
    claimed_at DESC
LIMIT $2;

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

