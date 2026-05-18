-- +goose Up
-- +goose StatementBegin
-- =============================================================================
-- 00076 marker: final legacy scope / org / tenant DDL is operator-run only.
--
-- CI migration safety blocks destructive statements in goose Up (see
-- docs/runbooks/migration-safety.md). The actual teardown + view rebuild lives in:
--   docs/runbooks/manual-db-cleanup/drop_legacy_scope_organization_tenant.sql
--
-- Fresh installs carry the canonical shape via db/schema and additive migrations;
-- existing deployments apply the manual script during an approved window.
-- =============================================================================

SELECT 1;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 1;

-- +goose StatementEnd
