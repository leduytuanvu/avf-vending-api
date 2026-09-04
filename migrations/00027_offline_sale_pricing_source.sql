-- +goose Up
-- Offline sale pricing: preserve machine-declared amounts on replay instead of server re-pricing.

ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS pricing_source text NOT NULL DEFAULT 'server_priced' CHECK (
        pricing_source IN ('server_priced', 'machine_local_verified', 'machine_local_unverified')
    ),
    ADD COLUMN IF NOT EXISTS machine_pricing_revision bigint,
    ADD COLUMN IF NOT EXISTS machine_pricing_snapshot jsonb;

COMMENT ON COLUMN orders.pricing_source IS 'How order totals were determined: server_priced (default) or machine-local snapshot on offline replay.';
COMMENT ON COLUMN orders.machine_pricing_revision IS 'Machine local pricing revision when pricing_source is machine_local_*.';
COMMENT ON COLUMN orders.machine_pricing_snapshot IS 'Immutable machine pricing snapshot JSON for audit and reconciliation.';

-- +goose Down
ALTER TABLE orders
    DROP COLUMN IF EXISTS machine_pricing_snapshot,
    DROP COLUMN IF EXISTS machine_pricing_revision,
    DROP COLUMN IF EXISTS pricing_source;
