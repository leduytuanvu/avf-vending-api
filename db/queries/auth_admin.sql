-- name: AuthGetAccountByOrgEmail :one
SELECT *
FROM platform_auth_accounts
WHERE organization_id = $1
  AND lower(email) = lower($2)
  AND status = 'active';

-- name: AuthLookupAccountByOrgEmailAnyStatus :one
SELECT *
FROM platform_auth_accounts
WHERE organization_id = $1
  AND lower(email) = lower($2);

-- name: AuthGetAccountByID :one
SELECT *
FROM platform_auth_accounts
WHERE id = $1
  AND status = 'active';

-- name: AuthInsertRefreshToken :exec
INSERT INTO auth_refresh_tokens (id, account_id, token_hash, expires_at, ip_address, user_agent)
VALUES ($1, $2, $3, $4, $5, $6);

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

-- name: AuthAdminGetAccountByOrgAndID :one
SELECT *
FROM platform_auth_accounts
WHERE id = $1
  AND organization_id = $2;

-- name: AuthAdminListAccounts :many
SELECT *
FROM platform_auth_accounts
WHERE organization_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: AuthAdminCountAccounts :one
SELECT count(*)::bigint
FROM platform_auth_accounts
WHERE organization_id = $1;

-- name: AuthAdminInsertAccount :one
INSERT INTO platform_auth_accounts (organization_id, email, password_hash, roles, status)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: AuthAdminUpdateAccount :one
UPDATE platform_auth_accounts
SET
    email = $3,
    roles = $4,
    status = $5,
    updated_at = now()
WHERE id = $1
  AND organization_id = $2
RETURNING *;

-- name: AuthAdminSetPasswordHash :exec
UPDATE platform_auth_accounts
SET password_hash = $2,
    updated_at = now()
WHERE id = $1;

-- name: AuthRecordLoginSuccess :exec
UPDATE platform_auth_accounts
SET failed_login_count = 0,
    locked_until = NULL,
    last_login_at = now(),
    status = CASE WHEN status = 'locked' THEN 'active' ELSE status END,
    updated_at = now()
WHERE id = $1;

-- name: AuthRecordLoginFailure :exec
UPDATE platform_auth_accounts
SET failed_login_count = failed_login_count + 1,
    locked_until = CASE WHEN failed_login_count + 1 >= $2 THEN now() + ($3::bigint * interval '1 second') ELSE locked_until END,
    status = CASE WHEN failed_login_count + 1 >= $2 THEN 'locked' ELSE status END,
    updated_at = now()
WHERE id = $1;

-- name: AuthClearExpiredLock :exec
UPDATE platform_auth_accounts
SET failed_login_count = 0,
    locked_until = NULL,
    status = 'active',
    updated_at = now()
WHERE id = $1
  AND status = 'locked'
  AND locked_until IS NOT NULL
  AND locked_until <= now();

-- name: AuthInsertPasswordResetToken :exec
INSERT INTO password_reset_tokens (id, user_id, organization_id, token_hash, expires_at, status)
VALUES ($1, $2, $3, $4, $5, 'active');

-- name: AuthGetPasswordResetTokenByHash :one
SELECT id, user_id, organization_id, token_hash, expires_at, used_at, created_at, status
FROM password_reset_tokens
WHERE token_hash = $1
  AND status = 'active'
  AND expires_at > now();

-- name: AuthMarkPasswordResetTokenUsed :exec
UPDATE password_reset_tokens
SET
    used_at = now(),
    status = 'used'
WHERE id = $1
  AND status = 'active';

-- name: AuthAdminCountActiveOrgAdmins :one
SELECT count(*)::bigint
FROM platform_auth_accounts
WHERE organization_id = $1
  AND status = 'active'
  AND 'org_admin'::text = ANY (roles);

-- name: AuthAdminCountActiveOrgAdminsExcluding :one
SELECT count(*)::bigint
FROM platform_auth_accounts
WHERE organization_id = $1
  AND id <> $2
  AND status = 'active'
  AND 'org_admin'::text = ANY (roles);
