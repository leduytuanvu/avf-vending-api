-- +goose Up
CREATE UNIQUE INDEX IF NOT EXISTS uniq_platform_auth_accounts_email_lower
    ON platform_auth_accounts (lower(email));

-- +goose Down
DROP INDEX IF EXISTS uniq_platform_auth_accounts_email_lower;
