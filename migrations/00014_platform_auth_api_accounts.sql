-- +goose Up
-- +goose StatementBegin

CREATE TABLE platform_auth_accounts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    email text NOT NULL,
    password_hash text NOT NULL,
    roles text[] NOT NULL DEFAULT '{}'::text[],
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ux_platform_auth_accounts_org_email ON platform_auth_accounts (organization_id, lower(email));

CREATE INDEX ix_platform_auth_accounts_organization_id ON platform_auth_accounts (organization_id);

CREATE TABLE auth_refresh_tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id uuid NOT NULL REFERENCES platform_auth_accounts (id) ON DELETE CASCADE,
    token_hash bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz
);

CREATE INDEX ix_auth_refresh_tokens_account_created ON auth_refresh_tokens (account_id, created_at DESC);
CREATE UNIQUE INDEX ux_auth_refresh_tokens_active_hash ON auth_refresh_tokens (token_hash)
WHERE revoked_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS auth_refresh_tokens;
DROP TABLE IF EXISTS platform_auth_accounts;
-- +goose StatementEnd
