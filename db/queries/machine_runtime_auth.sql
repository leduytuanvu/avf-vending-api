-- name: InsertMachineRuntimeRefreshToken :one
INSERT INTO
    machine_runtime_refresh_tokens (machine_id, organization_id, token_hash, expires_at)
VALUES
    ($1, $2, $3, $4)
RETURNING
    *;

-- name: GetMachineRuntimeRefreshTokenByHashForUpdate :one
SELECT
    *
FROM
    machine_runtime_refresh_tokens
WHERE
    token_hash = $1
FOR UPDATE;

-- name: RevokeMachineRuntimeRefreshToken :exec
UPDATE machine_runtime_refresh_tokens
SET
    revoked_at = now(),
    rotated_at = COALESCE(rotated_at, now())
WHERE
    id = $1;

-- name: MarkMachineRuntimeRefreshTokenUsed :exec
UPDATE machine_runtime_refresh_tokens
SET
    last_used_at = now()
WHERE
    id = $1;

-- name: HasActiveMachineRuntimeRefreshToken :one
SELECT
    EXISTS (
        SELECT
            1
        FROM
            machine_runtime_refresh_tokens
        WHERE
            machine_id = $1
            AND revoked_at IS NULL
            AND expires_at > now()
    ) AS exists;
