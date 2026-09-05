-- +goose Up
-- Local-first pricing provenance on quotes and reconciliation reference totals.

ALTER TABLE checkout_quotes
    ADD COLUMN IF NOT EXISTS pricing_source text NOT NULL DEFAULT 'server_priced' CHECK (
        pricing_source IN ('server_priced', 'machine_local_verified', 'machine_local_unverified')
    ),
    ADD COLUMN IF NOT EXISTS machine_pricing_revision bigint,
    ADD COLUMN IF NOT EXISTS machine_pricing_snapshot jsonb,
    ADD COLUMN IF NOT EXISTS server_reference_payable_minor bigint;

COMMENT ON COLUMN checkout_quotes.pricing_source IS 'How quote payable was determined.';
COMMENT ON COLUMN checkout_quotes.machine_pricing_revision IS 'Machine local pricing revision when pricing_source is machine_local_*.';
COMMENT ON COLUMN checkout_quotes.machine_pricing_snapshot IS 'Immutable machine pricing snapshot JSON for audit.';
COMMENT ON COLUMN checkout_quotes.server_reference_payable_minor IS 'Server catalog payable at quote time for reconciliation.';

ALTER TABLE checkout_quote_lines
    ADD COLUMN IF NOT EXISTS machine_unit_price_minor bigint,
    ADD COLUMN IF NOT EXISTS server_reference_unit_price_minor bigint;

COMMENT ON COLUMN checkout_quote_lines.machine_unit_price_minor IS 'Machine-declared unit price when pricing snapshot present.';
COMMENT ON COLUMN checkout_quote_lines.server_reference_unit_price_minor IS 'Server catalog unit price at quote time for reconciliation.';

ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS server_reference_total_minor bigint;

COMMENT ON COLUMN orders.server_reference_total_minor IS 'Server catalog total at order time for reconciliation.';

CREATE INDEX IF NOT EXISTS idx_orders_pricing_source_unverified
    ON orders (machine_id, created_at DESC)
    WHERE pricing_source = 'machine_local_unverified';

-- +goose Down
DROP INDEX IF EXISTS idx_orders_pricing_source_unverified;

ALTER TABLE orders
    DROP COLUMN IF EXISTS server_reference_total_minor;

ALTER TABLE checkout_quote_lines
    DROP COLUMN IF EXISTS server_reference_unit_price_minor,
    DROP COLUMN IF EXISTS machine_unit_price_minor;

ALTER TABLE checkout_quotes
    DROP COLUMN IF EXISTS server_reference_payable_minor,
    DROP COLUMN IF EXISTS machine_pricing_snapshot,
    DROP COLUMN IF EXISTS machine_pricing_revision,
    DROP COLUMN IF EXISTS pricing_source;
