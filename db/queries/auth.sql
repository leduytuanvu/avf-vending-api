-- name: AuthAdminMFACountActiveForUser :one
SELECT count(*)::bigint
FROM admin_mfa_factors
WHERE
    user_id = $1
    AND status = 'active';

-- name: AuthAdminMFAPendingFactor :one
SELECT *
FROM admin_mfa_factors
WHERE
    user_id = $1
    AND factor_type = 'totp'
    AND status = 'pending';

-- name: AuthAdminMFAInsertPending :one
INSERT INTO admin_mfa_factors (
    user_id,
    factor_type,
    secret_ciphertext,
    status
)
VALUES (
    $1,
    'totp',
    $2,
    'pending'
)
RETURNING *;

-- name: AuthAdminMFAActivateFactor :one
UPDATE admin_mfa_factors
SET
    status = 'active',
    verified_at = now()
WHERE
    id = $1
    AND user_id = $2
    AND status = 'pending'
RETURNING *;

-- name: AuthAdminMFADisableActiveTOTP :many
UPDATE admin_mfa_factors
SET
    status = 'disabled',
    disabled_at = now()
WHERE
    user_id = $1
    AND factor_type = 'totp'
    AND status = 'active'
RETURNING *;

-- name: AuthAdminMFADeletePendingForUser :exec
DELETE FROM admin_mfa_factors
WHERE
    user_id = $1
    AND status = 'pending';

-- name: AuthAdminMFAActiveFactorCiphertext :one
SELECT id, secret_ciphertext
FROM admin_mfa_factors
WHERE
    user_id = $1
    AND factor_type = 'totp'
    AND status = 'active';

-- name: AuthAdminInsertAdminSession :exec
INSERT INTO admin_sessions (
    id,
    user_id,
    refresh_token_id,
    refresh_token_hash,
    status,
    ip_address,
    user_agent,
    expires_at
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    'active',
    $5,
    $6,
    $7
);

-- name: AuthAdminListSessionsForAccount :many
SELECT
    id,
    user_id,
    refresh_token_id,
    refresh_token_hash,
    status,
    ip_address,
    user_agent,
    created_at,
    last_used_at,
    expires_at,
    revoked_at
FROM admin_sessions
WHERE
    user_id = $1
ORDER BY created_at DESC;

-- name: AuthAdminGetAdminSessionByRefreshTokenID :one
SELECT
    id,
    user_id,
    refresh_token_id,
    refresh_token_hash,
    status,
    ip_address,
    user_agent,
    created_at,
    last_used_at,
    expires_at,
    revoked_at
FROM admin_sessions
WHERE refresh_token_id = $1;

-- name: AuthAdminRevokeAdminSessionByID :execrows
UPDATE admin_sessions
SET
    status = 'revoked',
    revoked_at = now()
WHERE
    id = $1
    AND user_id = $2
    AND TRUE
    AND status = 'active';

-- name: AuthAdminRevokeAllAdminSessionsForUser :exec
UPDATE admin_sessions
SET
    status = 'revoked',
    revoked_at = now()
WHERE
    user_id = $1
    AND status = 'active';

-- name: AuthAdminTouchSessionByRefreshToken :exec
UPDATE admin_sessions
SET last_used_at = now()
WHERE refresh_token_id = $1;

-- name: AuthInsertLoginAttempt :exec
INSERT INTO admin_login_attempts (
    email_normalized,
    user_id,
    ip_address,
    user_agent,
    success,
    failure_reason
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
);

-- name: AuthAdminRotateSessionRefreshToken :exec
UPDATE admin_sessions
SET
    refresh_token_id = $1,
    refresh_token_hash = $2,
    expires_at = $3,
    last_used_at = now()
WHERE refresh_token_id = $4
  AND user_id = $5
  AND status = 'active';

-- name: AuthAdminGetAdminSessionByUserAndID :one
SELECT
    id,
    user_id,
    refresh_token_id,
    refresh_token_hash,
    status,
    ip_address,
    user_agent,
    created_at,
    last_used_at,
    expires_at,
    revoked_at
FROM admin_sessions
WHERE
    id = $1
    AND user_id = $2
    AND TRUE
    AND status = 'active';

-- name: AuthAdminRevokeAdminSessionByRefreshTokenID :execrows
UPDATE admin_sessions
SET
    status = 'revoked',
    revoked_at = now()
WHERE
    refresh_token_id = $1
    AND user_id = $2
    AND status = 'active';
