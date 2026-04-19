-- name: AuthGetAccountByOrgEmail :one
SELECT id, organization_id, email, password_hash, roles, status, created_at, updated_at
FROM platform_auth_accounts
WHERE organization_id = $1
  AND lower(email) = lower($2)
  AND status = 'active';

-- name: AuthGetAccountByID :one
SELECT id, organization_id, email, password_hash, roles, status, created_at, updated_at
FROM platform_auth_accounts
WHERE id = $1
  AND status = 'active';

-- name: AuthInsertRefreshToken :exec
INSERT INTO auth_refresh_tokens (id, account_id, token_hash, expires_at)
VALUES ($1, $2, $3, $4);

-- name: AuthGetRefreshTokenByHash :one
SELECT id, account_id, token_hash, expires_at, revoked_at, created_at, last_used_at
FROM auth_refresh_tokens
WHERE token_hash = $1
  AND revoked_at IS NULL
  AND expires_at > now();

-- name: AuthTouchRefreshToken :exec
UPDATE auth_refresh_tokens
SET last_used_at = now()
WHERE id = $1;

-- name: AuthRevokeRefreshToken :exec
UPDATE auth_refresh_tokens
SET revoked_at = now()
WHERE id = $1;

-- name: AuthRevokeAllRefreshForAccount :exec
UPDATE auth_refresh_tokens
SET revoked_at = now()
WHERE account_id = $1
  AND revoked_at IS NULL;
