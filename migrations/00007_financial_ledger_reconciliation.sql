-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- Settlement batches (referenced by payments / refunds)
-- ---------------------------------------------------------------------------
CREATE TABLE settlement_batches (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    provider text NOT NULL,
    period_start date NOT NULL,
    period_end date NOT NULL,
    status text NOT NULL CHECK (status IN ('draft', 'open', 'processing', 'posted', 'failed', 'cancelled')),
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_settlement_batches_provider_period ON settlement_batches (provider, period_start, period_end);

COMMENT ON TABLE settlement_batches IS 'PSP settlement window; link payments via settlement_batch_id when batched.';

-- ---------------------------------------------------------------------------
-- Machine reconciliation sessions (cash + digital expected vs actual)
-- ---------------------------------------------------------------------------
CREATE TABLE machine_reconciliation_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    business_date date NOT NULL,
    opened_at timestamptz NOT NULL,
    closed_at timestamptz,
    expected_cash_amount_minor bigint NOT NULL DEFAULT 0,
    actual_cash_amount_minor bigint NOT NULL DEFAULT 0,
    expected_digital_amount_minor bigint NOT NULL DEFAULT 0,
    actual_digital_amount_minor bigint NOT NULL DEFAULT 0,
    variance_amount_minor bigint NOT NULL DEFAULT 0,
    status text NOT NULL CHECK (status IN ('open', 'closed', 'variance_review', 'cancelled'))
);

CREATE UNIQUE INDEX ux_machine_reconciliation_sessions_open_per_day ON machine_reconciliation_sessions (machine_id, business_date)
    WHERE status = 'open';

CREATE INDEX ix_machine_reconciliation_sessions_machine_date ON machine_reconciliation_sessions (machine_id, business_date DESC);

COMMENT ON COLUMN machine_reconciliation_sessions.business_date IS 'Operator calendar day in organization TZ; store date only—resolve TZ in application.';
COMMENT ON COLUMN machine_reconciliation_sessions.variance_amount_minor IS 'actual - expected under session convention when closed.';

-- ---------------------------------------------------------------------------
-- Cash collections (physical pull / bag swap)
-- ---------------------------------------------------------------------------
CREATE TABLE cash_collections (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    collected_at timestamptz NOT NULL,
    amount_minor bigint NOT NULL CHECK (amount_minor >= 0),
    currency char(3) NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    reconciliation_status text NOT NULL DEFAULT 'pending' CHECK (
        reconciliation_status IN ('pending', 'matched', 'mismatch', 'waived')
    ),
    reconciled_by text,
    reconciled_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_cash_collections_machine_collected ON cash_collections (machine_id, collected_at DESC);
CREATE INDEX ix_cash_collections_org_collected ON cash_collections (organization_id, collected_at DESC);
CREATE INDEX ix_cash_collections_unreconciled ON cash_collections (machine_id, collected_at DESC)
    WHERE reconciliation_status <> 'matched';

COMMENT ON TABLE cash_collections IS 'Physical cash removed from machine; reconcile against expected vault from cash_events.';

-- ---------------------------------------------------------------------------
-- Cash events (immutable hopper activity)
-- ---------------------------------------------------------------------------
CREATE TABLE cash_events (
    id bigserial PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    event_type text NOT NULL CHECK (
        event_type IN ('insert', 'dispense_change', 'reject', 'audit_adjust', 'transfer', 'other')
    ),
    amount_minor bigint NOT NULL,
    currency char(3) NOT NULL,
    occurred_at timestamptz NOT NULL,
    correlation_id uuid,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    reconciliation_session_id uuid REFERENCES machine_reconciliation_sessions (id) ON DELETE SET NULL
);

CREATE INDEX ix_cash_events_org_occurred ON cash_events (organization_id, occurred_at DESC);
CREATE INDEX ix_cash_events_machine_occurred ON cash_events (machine_id, occurred_at DESC);
CREATE INDEX ix_cash_events_session ON cash_events (reconciliation_session_id)
    WHERE reconciliation_session_id IS NOT NULL;
CREATE INDEX ix_cash_events_correlation ON cash_events (correlation_id, occurred_at DESC)
    WHERE correlation_id IS NOT NULL;

COMMENT ON TABLE cash_events IS 'Append-only cash movement log; application INSERT-only. amount_minor semantics per event_type in metadata or ops runbook.';

-- ---------------------------------------------------------------------------
-- Payments / refunds: reconciliation + settlement
-- ---------------------------------------------------------------------------
ALTER TABLE payments
    ADD COLUMN reconciliation_status text NOT NULL DEFAULT 'pending' CHECK (
        reconciliation_status IN ('pending', 'matched', 'mismatch', 'not_required')
    ),
    ADD COLUMN settlement_status text NOT NULL DEFAULT 'unsettled' CHECK (
        settlement_status IN ('unsettled', 'batched', 'settled', 'written_off')
    ),
    ADD COLUMN settlement_batch_id uuid REFERENCES settlement_batches (id) ON DELETE SET NULL;

CREATE INDEX ix_payments_reconciliation_queue ON payments (provider, updated_at DESC)
    WHERE reconciliation_status <> 'matched';
CREATE INDEX ix_payments_settlement_batch ON payments (settlement_batch_id)
    WHERE settlement_batch_id IS NOT NULL;

COMMENT ON COLUMN payments.reconciliation_status IS 'Provider vs internal ledger alignment; use payment_reconciliations for detail.';
COMMENT ON COLUMN payments.settlement_status IS 'PSP settlement lifecycle; settlement_batch_id when batched.';

ALTER TABLE refunds
    ADD COLUMN reconciliation_status text NOT NULL DEFAULT 'pending' CHECK (
        reconciliation_status IN ('pending', 'matched', 'mismatch', 'not_required')
    ),
    ADD COLUMN settlement_status text NOT NULL DEFAULT 'unsettled' CHECK (
        settlement_status IN ('unsettled', 'batched', 'settled', 'written_off')
    ),
    ADD COLUMN settlement_batch_id uuid REFERENCES settlement_batches (id) ON DELETE SET NULL;

CREATE INDEX ix_refunds_reconciliation_queue ON refunds (payment_id, created_at DESC)
    WHERE reconciliation_status <> 'matched';
CREATE INDEX ix_refunds_settlement_batch ON refunds (settlement_batch_id)
    WHERE settlement_batch_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- Payment provider events (webhook / API mirror)
-- ---------------------------------------------------------------------------
CREATE TABLE payment_provider_events (
    id bigserial PRIMARY KEY,
    payment_id uuid REFERENCES payments (id) ON DELETE SET NULL,
    provider text NOT NULL,
    provider_ref text,
    provider_amount_minor bigint,
    currency char(3),
    event_type text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    received_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ux_payment_provider_events_provider_ref ON payment_provider_events (provider, provider_ref)
    WHERE provider_ref IS NOT NULL AND btrim(provider_ref) <> '';

CREATE INDEX ix_payment_provider_events_payment ON payment_provider_events (payment_id, received_at DESC)
    WHERE payment_id IS NOT NULL;
CREATE INDEX ix_payment_provider_events_received ON payment_provider_events (provider, received_at DESC);

COMMENT ON TABLE payment_provider_events IS 'Raw PSP notifications; payment_id nullable for orphan webhooks until correlated.';

-- ---------------------------------------------------------------------------
-- Payment reconciliations
-- ---------------------------------------------------------------------------
CREATE TABLE payment_reconciliations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id uuid NOT NULL REFERENCES payments (id) ON DELETE CASCADE,
    provider text NOT NULL,
    provider_ref text NOT NULL,
    provider_amount_minor bigint NOT NULL,
    internal_amount_minor bigint NOT NULL,
    currency char(3) NOT NULL,
    reconciled_at timestamptz NOT NULL,
    status text NOT NULL CHECK (status IN ('matched', 'mismatch', 'pending')),
    mismatch_reason text,
    CONSTRAINT ux_payment_reconciliations_provider_ref_payment UNIQUE (provider, provider_ref, payment_id)
);

CREATE INDEX ix_payment_reconciliations_payment_time ON payment_reconciliations (payment_id, reconciled_at DESC);
CREATE INDEX ix_payment_reconciliations_unmatched ON payment_reconciliations (provider, reconciled_at DESC)
    WHERE status IN ('pending', 'mismatch');

-- ---------------------------------------------------------------------------
-- Cash reconciliations
-- ---------------------------------------------------------------------------
CREATE TABLE cash_reconciliations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    cash_session_id uuid,
    cash_collection_id uuid REFERENCES cash_collections (id) ON DELETE SET NULL,
    expected_amount_minor bigint NOT NULL,
    counted_amount_minor bigint NOT NULL,
    variance_amount_minor bigint NOT NULL,
    reconciled_at timestamptz NOT NULL,
    status text NOT NULL CHECK (status IN ('matched', 'mismatch', 'pending', 'review')),
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX ix_cash_reconciliations_machine_time ON cash_reconciliations (machine_id, reconciled_at DESC);
CREATE INDEX ix_cash_reconciliations_unmatched ON cash_reconciliations (machine_id, reconciled_at DESC)
    WHERE status IN ('pending', 'mismatch', 'review');

COMMENT ON COLUMN cash_reconciliations.cash_session_id IS 'Reserved for future cash_sessions table; no FK until introduced.';

-- ---------------------------------------------------------------------------
-- Financial ledger (append-only; application INSERT-only — enforce via GRANT/policy)
-- ---------------------------------------------------------------------------
CREATE TABLE financial_ledger_entries (
    id bigserial PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid REFERENCES machines (id) ON DELETE SET NULL,
    site_id uuid REFERENCES sites (id) ON DELETE SET NULL,
    order_id uuid REFERENCES orders (id) ON DELETE SET NULL,
    payment_id uuid REFERENCES payments (id) ON DELETE SET NULL,
    refund_id uuid REFERENCES refunds (id) ON DELETE SET NULL,
    cash_event_id bigint REFERENCES cash_events (id) ON DELETE SET NULL,
    cash_collection_id uuid REFERENCES cash_collections (id) ON DELETE SET NULL,
    entry_type text NOT NULL CHECK (
        entry_type IN (
            'order_created',
            'payment_authorized',
            'payment_captured',
            'payment_failed',
            'refund_issued',
            'cash_inserted',
            'change_dispensed',
            'cash_collected',
            'variance_recorded',
            'adjustment',
            'other'
        )
    ),
    signed_amount_minor bigint NOT NULL,
    currency char(3) NOT NULL,
    occurred_at timestamptz NOT NULL,
    reference_type text,
    reference_id uuid,
    correlation_id uuid,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX ix_financial_ledger_entries_org_time ON financial_ledger_entries (organization_id, occurred_at DESC);
CREATE INDEX ix_financial_ledger_entries_machine_time ON financial_ledger_entries (machine_id, occurred_at DESC)
    WHERE machine_id IS NOT NULL;
CREATE INDEX ix_financial_ledger_entries_payment ON financial_ledger_entries (payment_id)
    WHERE payment_id IS NOT NULL;
CREATE INDEX ix_financial_ledger_entries_order ON financial_ledger_entries (order_id)
    WHERE order_id IS NOT NULL;
CREATE INDEX ix_financial_ledger_entries_cash_event ON financial_ledger_entries (cash_event_id)
    WHERE cash_event_id IS NOT NULL;
CREATE INDEX ix_financial_ledger_entries_cash_collection ON financial_ledger_entries (cash_collection_id)
    WHERE cash_collection_id IS NOT NULL;
CREATE INDEX ix_financial_ledger_entries_correlation ON financial_ledger_entries (correlation_id, occurred_at DESC)
    WHERE correlation_id IS NOT NULL;

COMMENT ON TABLE financial_ledger_entries IS 'Append-only monetary fact stream; no updated_at. Application: INSERT only (revoke UPDATE for app role or enforce in repo).';
COMMENT ON COLUMN financial_ledger_entries.signed_amount_minor IS 'Signed minor units: positive = economic benefit to org (e.g. captured payment), negative = outflow (refund, change); document per entry_type in app.';
COMMENT ON COLUMN financial_ledger_entries.reference_type IS 'Polymorphic pointer when no dedicated FK column; prefer order_id/payment_id/cash_event_id when possible.';

/*
Finance traceability:
- Traverse ledger by order_id / payment_id / cash_event_id / machine_id + join machine_reconciliation_sessions on business_date.
- Digital mismatches: payment_reconciliations.status / mismatch_reason + payments.reconciliation_status.
- Cash mismatches: cash_reconciliations + ledger entry_type variance_recorded without relying on mutable payment state alone.
*/

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS financial_ledger_entries;

DROP TABLE IF EXISTS cash_reconciliations;

DROP TABLE IF EXISTS payment_reconciliations;

DROP TABLE IF EXISTS payment_provider_events;

DROP INDEX IF EXISTS ix_payments_reconciliation_queue;
DROP INDEX IF EXISTS ix_payments_settlement_batch;

ALTER TABLE payments
    DROP CONSTRAINT IF EXISTS payments_settlement_batch_id_fkey,
    DROP COLUMN IF EXISTS settlement_batch_id,
    DROP COLUMN IF EXISTS settlement_status,
    DROP COLUMN IF EXISTS reconciliation_status;

DROP INDEX IF EXISTS ix_refunds_reconciliation_queue;
DROP INDEX IF EXISTS ix_refunds_settlement_batch;

ALTER TABLE refunds
    DROP CONSTRAINT IF EXISTS refunds_settlement_batch_id_fkey,
    DROP COLUMN IF EXISTS settlement_batch_id,
    DROP COLUMN IF EXISTS settlement_status,
    DROP COLUMN IF EXISTS reconciliation_status;

DROP TABLE IF EXISTS cash_events;

DROP TABLE IF EXISTS cash_collections;

DROP TABLE IF EXISTS machine_reconciliation_sessions;

DROP TABLE IF EXISTS settlement_batches;

-- +goose StatementEnd
