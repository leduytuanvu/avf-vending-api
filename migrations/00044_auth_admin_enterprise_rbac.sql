-- +goose Up
ALTER TABLE platform_auth_accounts
    ADD COLUMN IF NOT EXISTS failed_login_count integer NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS locked_until timestamptz,
    ADD COLUMN IF NOT EXISTS last_login_at timestamptz,
    ADD COLUMN IF NOT EXISTS invited_at timestamptz;

DO $$
DECLARE
    constraint_name text;
BEGIN
    SELECT c.conname
    INTO constraint_name
    FROM pg_constraint c
    JOIN pg_class t ON t.oid = c.conrelid
    JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY (c.conkey)
    WHERE t.relname = 'platform_auth_accounts'
      AND a.attname = 'status'
      AND c.contype = 'c'
    LIMIT 1;

    IF constraint_name IS NOT NULL THEN
        EXECUTE format('ALTER TABLE platform_auth_accounts DROP CONSTRAINT %I', constraint_name);
    END IF;
END $$;

ALTER TABLE platform_auth_accounts
    ADD CONSTRAINT chk_platform_auth_accounts_status
    CHECK (status IN ('active', 'disabled', 'locked', 'invited'));

CREATE TABLE IF NOT EXISTS auth_password_reset_tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id uuid NOT NULL REFERENCES platform_auth_accounts (id) ON DELETE CASCADE,
    token_hash bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_auth_password_reset_tokens_active_hash
    ON auth_password_reset_tokens (token_hash)
    WHERE used_at IS NULL;

CREATE INDEX IF NOT EXISTS ix_auth_password_reset_tokens_account_created
    ON auth_password_reset_tokens (account_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS ix_auth_password_reset_tokens_account_created;
DROP INDEX IF EXISTS ux_auth_password_reset_tokens_active_hash;
DROP TABLE IF EXISTS auth_password_reset_tokens;

ALTER TABLE platform_auth_accounts
    DROP CONSTRAINT IF EXISTS chk_platform_auth_accounts_status;

ALTER TABLE platform_auth_accounts
    ADD CONSTRAINT platform_auth_accounts_status_check
    CHECK (status IN ('active', 'disabled'));

ALTER TABLE platform_auth_accounts
    DROP COLUMN IF EXISTS invited_at,
    DROP COLUMN IF EXISTS last_login_at,
    DROP COLUMN IF EXISTS locked_until,
    DROP COLUMN IF EXISTS failed_login_count;
