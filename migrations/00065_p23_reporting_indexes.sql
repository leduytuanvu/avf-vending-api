-- +goose Up
-- P2.3: reporting window scans on orders and reconciliation cases (additive indexes).

CREATE INDEX IF NOT EXISTS ix_orders_company_created_at ON orders (scope_id, created_at DESC);

CREATE INDEX IF NOT EXISTS ix_commerce_reconciliation_cases_org_first_detected ON commerce_reconciliation_cases (scope_id, first_detected_at DESC);

COMMENT ON INDEX ix_orders_company_created_at IS 'Company reporting: filter orders by company and created_at window.';
COMMENT ON INDEX ix_commerce_reconciliation_cases_org_first_detected IS 'Company BI: filter reconciliation cases by company and first_detected_at window.';

-- +goose Down

DROP INDEX IF EXISTS ix_commerce_reconciliation_cases_org_first_detected;
DROP INDEX IF EXISTS ix_orders_company_created_at;
