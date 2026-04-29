-- name: HasActiveMachineSession :one
SELECT
    EXISTS (
        SELECT
            1
        FROM
            machine_sessions
        WHERE
            machine_id = $1
            AND status = 'active'
            AND revoked_at IS NULL
            AND expires_at > now()
    ) AS exists;

-- name: GetActiveMachineSessionForMachine :one
SELECT
    *
FROM
    machine_sessions
WHERE
    machine_id = $1
    AND status = 'active'
    AND revoked_at IS NULL
    AND expires_at > now()
ORDER BY
    issued_at DESC
LIMIT
    1;

-- name: GetMachineCredentialByMachineAndVersion :one
SELECT
    *
FROM
    machine_credentials
WHERE
    machine_id = $1
    AND credential_version = $2;

-- name: InsertMachineCredential :one
INSERT INTO
    machine_credentials (organization_id, machine_id, credential_version, secret_hash, status)
VALUES
    ($1, $2, $3, $4, $5)
RETURNING
    *;

-- name: MarkMachineCredentialRotatedByVersion :exec
UPDATE machine_credentials
SET
    status = 'rotated',
    rotated_at = now()
WHERE
    machine_id = $1
    AND organization_id = $2
    AND credential_version = $3
    AND status = 'active';

-- name: MarkMachineCredentialsRevokedActive :exec
UPDATE machine_credentials
SET
    status = 'revoked',
    revoked_at = now()
WHERE
    machine_id = $1
    AND organization_id = $2
    AND status = 'active';

-- name: MarkMachineCredentialsCompromised :exec
UPDATE machine_credentials
SET
    status = 'compromised',
    revoked_at = COALESCE(revoked_at, now())
WHERE
    machine_id = $1
    AND organization_id = $2
    AND status = 'active';

-- name: InsertMachineSession :one
INSERT INTO
    machine_sessions (
        organization_id,
        machine_id,
        credential_id,
        refresh_token_hash,
        access_token_jti,
        refresh_token_jti,
        credential_version,
        status,
        expires_at,
        user_agent,
        ip_address
    )
VALUES
    ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING
    *;

-- name: GetMachineSessionByRefreshHashForUpdate :one
SELECT
    *
FROM
    machine_sessions
WHERE
    refresh_token_hash = $1
FOR UPDATE;

-- name: RevokeMachineSessionByID :exec
UPDATE machine_sessions
SET
    status = 'revoked',
    revoked_at = now()
WHERE
    id = $1;

-- name: MarkMachineSessionUsedByID :exec
UPDATE machine_sessions
SET
    last_used_at = now()
WHERE
    id = $1;

-- name: RevokeAllMachineSessionsForMachine :exec
UPDATE machine_sessions
SET
    status = 'revoked',
    revoked_at = COALESCE(revoked_at, now())
WHERE
    machine_id = $1
    AND organization_id = $2
    AND status = 'active';

-- name: GetMachineSessionGate :one
SELECT
    ms.id,
    ms.organization_id,
    ms.machine_id,
    ms.credential_id,
    ms.credential_version AS session_credential_version,
    ms.status AS session_status,
    ms.expires_at AS session_expires_at,
    ms.revoked_at AS session_revoked_at,
    mc.status AS credential_status,
    mc.credential_version AS credential_row_version,
    m.credential_version AS machine_credential_version,
    m.credential_revoked_at AS machine_credential_revoked_at,
    m.status AS machine_status
FROM
    machine_sessions ms
    INNER JOIN machine_credentials mc ON mc.id = ms.credential_id
    INNER JOIN machines m ON m.id = ms.machine_id
WHERE
    ms.id = $1
    AND ms.machine_id = $2;
