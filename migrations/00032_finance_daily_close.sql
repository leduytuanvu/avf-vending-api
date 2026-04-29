-- Immutable finance daily close snapshot plus adjustment ledger hooks for corrections.

-- +goose Up
CREATE TABLE finance_daily_closes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    close_date date NOT NULL,
    timezone text NOT NULL,
    site_id uuid REFERENCES sites (id) ON DELETE SET NULL,
    machine_id uuid REFERENCES machines (id) ON DELETE SET NULL,
    idempotency_key text NOT NULL,
    gross_sales_minor bigint NOT NULL DEFAULT 0 CHECK (gross_sales_minor >= 0),
    discount_minor bigint NOT NULL DEFAULT 0 CHECK (discount_minor >= 0),
    refund_minor bigint NOT NULL DEFAULT 0 CHECK (refund_minor >= 0),
    net_minor bigint NOT NULL,
    cash_minor bigint NOT NULL DEFAULT 0 CHECK (cash_minor >= 0),
    qr_wallet_minor bigint NOT NULL DEFAULT 0 CHECK (qr_wallet_minor >= 0),
    failed_minor bigint NOT NULL DEFAULT 0 CHECK (failed_minor >= 0),
    pending_minor bigint NOT NULL DEFAULT 0 CHECK (pending_minor >= 0),
    created_at timestamptz NOT NULL DEFAULT now (),
    CONSTRAINT ux_finance_daily_closes_org_idem UNIQUE (organization_id, idempotency_key)
);

CREATE UNIQUE INDEX ux_finance_daily_closes_scope ON finance_daily_closes (
    organization_id,
    close_date,
    timezone,
    COALESCE(site_id, '00000000-0000-0000-0000-000000000000'::uuid),
    COALESCE(machine_id, '00000000-0000-0000-0000-000000000000'::uuid)
);

CREATE INDEX ix_finance_daily_closes_org_date ON finance_daily_closes (organization_id, close_date DESC);

COMMENT ON TABLE finance_daily_closes IS 'Immutable org/day/timezone (optional site/machine scope) snapshot; corrections via finance_daily_close_adjustments.';

CREATE TABLE finance_daily_close_adjustments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    daily_close_id uuid NOT NULL REFERENCES finance_daily_closes (id) ON DELETE CASCADE,
    reason text NOT NULL,
    delta_net_minor bigint NOT NULL DEFAULT 0,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now ()
);

CREATE INDEX ix_finance_daily_close_adjustments_close ON finance_daily_close_adjustments (daily_close_id);

COMMENT ON TABLE finance_daily_close_adjustments IS 'Post-close corrections; immutable daily_close rows are never updated in place.';

-- +goose Down
DROP INDEX IF EXISTS ix_finance_daily_close_adjustments_close;

DROP TABLE IF EXISTS finance_daily_close_adjustments;

COMMENT ON TABLE finance_daily_closes IS NULL;

DROP INDEX IF EXISTS ix_finance_daily_closes_org_date;

DROP INDEX IF EXISTS ux_finance_daily_closes_scope;

DROP TABLE IF EXISTS finance_daily_closes;
