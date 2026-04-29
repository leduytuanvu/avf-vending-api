-- +goose Up
-- +goose StatementBegin
-- Org-scoped email uniqueness: enforced since migration 00014 via ux_platform_auth_accounts_org_email.
-- This migration is idempotent for databases that already applied 00014.
CREATE UNIQUE INDEX IF NOT EXISTS ux_platform_auth_accounts_org_email ON platform_auth_accounts (organization_id, lower(email));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Index name shared with historical migration; leave in place to avoid breaking older deployments.
SELECT 1;
-- +goose StatementEnd
