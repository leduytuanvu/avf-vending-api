-- +goose Up
-- P2.3: reporting window scans on orders and reconciliation cases (additive indexes).

CREATE INDEX IF NOT EXISTS ix_orders_organization_created_at ON orders (organization_id, created_at DESC);

CREATE INDEX IF NOT EXISTS ix_commerce_reconciliation_cases_org_first_detected ON commerce_reconciliation_cases (organization_id, first_detected_at DESC);

COMMENT ON INDEX ix_orders_organization_created_at IS 'Tenant reporting: filter orders by organization and created_at window.';
COMMENT ON INDEX ix_commerce_reconciliation_cases_org_first_detected IS 'Tenant BI: filter reconciliation cases by organization and first_detected_at window.';

-- +goose Down

DROP INDEX IF EXISTS ix_commerce_reconciliation_cases_org_first_detected;
DROP INDEX IF EXISTS ix_orders_organization_created_at;
