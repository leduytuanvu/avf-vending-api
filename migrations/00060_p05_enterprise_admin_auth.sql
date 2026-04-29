-- P0.5 Enterprise admin auth: MFA factors, durable sessions mirror, login attempts audit,
-- password_reset_tokens naming + lifecycle columns, refresh token metadata.

BEGIN;

ALTER TABLE auth_refresh_tokens ADD COLUMN IF NOT EXISTS ip_address text;
ALTER TABLE auth_refresh_tokens ADD COLUMN IF NOT EXISTS user_agent text;

CREATE TABLE IF NOT EXISTS admin_mfa_factors (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES platform_auth_accounts (id) ON DELETE CASCADE,
    factor_type text NOT NULL CHECK (factor_type = 'totp'),
    secret_ciphertext bytea NOT NULL,
    status text NOT NULL CHECK (
        status IN ('pending', 'active', 'disabled')
    ),
    created_at timestamptz NOT NULL DEFAULT now(),
    verified_at timestamptz,
    disabled_at timestamptz,
    CONSTRAINT ux_admin_mfa_factors_user_factor UNIQUE (user_id, factor_type)
);

CREATE INDEX IF NOT EXISTS ix_admin_mfa_factors_org ON admin_mfa_factors (organization_id);

CREATE TABLE IF NOT EXISTS admin_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES platform_auth_accounts (id) ON DELETE CASCADE,
    refresh_token_id uuid NOT NULL UNIQUE REFERENCES auth_refresh_tokens (id) ON DELETE CASCADE,
    refresh_token_hash bytea NOT NULL,
    status text NOT NULL CHECK (status IN ('active', 'revoked', 'expired')),
    ip_address text,
    user_agent text,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz
);

CREATE INDEX IF NOT EXISTS ix_admin_sessions_org_user ON admin_sessions (organization_id, user_id);

CREATE TABLE IF NOT EXISTS admin_login_attempts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    organization_id uuid REFERENCES organizations (id) ON DELETE SET NULL,
    email_normalized text NOT NULL,
    user_id uuid REFERENCES platform_auth_accounts (id) ON DELETE SET NULL,
    ip_address text,
    user_agent text,
    success boolean NOT NULL,
    failure_reason text,
    occurred_at timestamptz NOT NULL DEFAULT now ()
);

CREATE INDEX IF NOT EXISTS ix_admin_login_attempts_occurred ON admin_login_attempts (occurred_at DESC);

ALTER TABLE auth_password_reset_tokens RENAME TO password_reset_tokens;

ALTER TABLE password_reset_tokens RENAME COLUMN account_id TO user_id;

ALTER TABLE password_reset_tokens
ADD COLUMN IF NOT EXISTS organization_id uuid REFERENCES organizations (id) ON DELETE CASCADE;

UPDATE password_reset_tokens pr
SET
    organization_id = a.organization_id
FROM platform_auth_accounts a
WHERE
    pr.user_id = a.id
    AND pr.organization_id IS NULL;

ALTER TABLE password_reset_tokens
ALTER COLUMN organization_id
SET NOT NULL;

ALTER TABLE password_reset_tokens ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'active';

UPDATE password_reset_tokens
SET
    status = CASE
        WHEN used_at IS NOT NULL THEN 'used'
        ELSE 'active'
    END
WHERE
    TRUE;

ALTER TABLE password_reset_tokens
ADD CONSTRAINT ck_password_reset_tokens_status CHECK (
    status IN ('active', 'used', 'expired', 'revoked')
);

ALTER TABLE password_reset_tokens ADD COLUMN IF NOT EXISTS revoked_at timestamptz;

DROP INDEX IF EXISTS ux_auth_password_reset_tokens_active_hash;

CREATE UNIQUE INDEX ux_password_reset_tokens_active_hash ON password_reset_tokens (token_hash)
WHERE
    status = 'active';

CREATE INDEX ix_password_reset_tokens_user_created ON password_reset_tokens (user_id, created_at DESC);

COMMIT;
