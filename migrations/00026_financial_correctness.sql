-- +goose Up
-- Financial correctness: winner arbitration, cash consent/allocation, extended ledger.

ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS winning_payment_id uuid REFERENCES payments (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS winning_claimed_at timestamptz;

CREATE UNIQUE INDEX IF NOT EXISTS ux_orders_winning_payment_id
    ON orders (winning_payment_id)
    WHERE winning_payment_id IS NOT NULL;

ALTER TABLE payments
    ADD COLUMN IF NOT EXISTS outcome text NOT NULL DEFAULT 'pending' CHECK (
        outcome IN ('pending', 'winner', 'superseded', 'refund_required', 'refunded')
    ),
    ADD COLUMN IF NOT EXISTS attempt_seq int NOT NULL DEFAULT 1 CHECK (attempt_seq >= 1),
    ADD COLUMN IF NOT EXISTS supersedes_payment_id uuid REFERENCES payments (id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS ix_payments_order_outcome ON payments (order_id, outcome);

CREATE TABLE IF NOT EXISTS cash_acceptance_events (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v7(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE RESTRICT,
    order_id uuid REFERENCES orders (id) ON DELETE SET NULL,
    device_event_id text NOT NULL,
    denomination_minor bigint NOT NULL CHECK (denomination_minor > 0),
    credit_source text NOT NULL DEFAULT 'unknown',
    currency char(3) NOT NULL,
    accepted_at timestamptz NOT NULL,
    raw_metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ux_cash_acceptance_events_machine_device UNIQUE (machine_id, device_event_id)
);

CREATE INDEX IF NOT EXISTS ix_cash_acceptance_events_order ON cash_acceptance_events (order_id, accepted_at DESC)
    WHERE order_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS cash_allocations (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v7(),
    order_id uuid NOT NULL REFERENCES orders (id) ON DELETE RESTRICT,
    payment_id uuid REFERENCES payments (id) ON DELETE SET NULL,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE RESTRICT,
    amount_minor bigint NOT NULL CHECK (amount_minor >= 0),
    pre_order_credit_minor bigint NOT NULL DEFAULT 0 CHECK (pre_order_credit_minor >= 0),
    post_order_inserted_minor bigint NOT NULL DEFAULT 0 CHECK (post_order_inserted_minor >= 0),
    consent_source text NOT NULL CHECK (
        consent_source IN ('explicit_confirm', 'implicit_post_order', 'operator', 'unknown')
    ),
    consented_at timestamptz,
    currency char(3) NOT NULL,
    idempotency_key text,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ux_cash_allocations_order_idempotency UNIQUE (order_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS ix_cash_allocations_order ON cash_allocations (order_id);

CREATE TABLE IF NOT EXISTS cash_change_events (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v7(),
    order_id uuid NOT NULL REFERENCES orders (id) ON DELETE RESTRICT,
    payment_id uuid REFERENCES payments (id) ON DELETE SET NULL,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE RESTRICT,
    change_due_minor bigint NOT NULL DEFAULT 0 CHECK (change_due_minor >= 0),
    change_dispensed_minor bigint NOT NULL DEFAULT 0 CHECK (change_dispensed_minor >= 0),
    outcome text NOT NULL CHECK (
        outcome IN ('delivered', 'delivered_after_fault', 'not_delivered', 'ambiguous', 'none')
    ),
    liability_minor bigint NOT NULL DEFAULT 0 CHECK (liability_minor >= 0),
    currency char(3) NOT NULL,
    idempotency_key text,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ux_cash_change_events_order_idempotency UNIQUE (order_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS ix_cash_change_events_order ON cash_change_events (order_id);

-- Extend financial_ledger_entries entry_type check.
ALTER TABLE financial_ledger_entries DROP CONSTRAINT IF EXISTS financial_ledger_entries_entry_type_check;
ALTER TABLE financial_ledger_entries ADD CONSTRAINT financial_ledger_entries_entry_type_check CHECK (
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
        'other',
        'cash_accepted',
        'cash_allocated',
        'change_due',
        'change_liability',
        'capture_superseded'
    )
);

-- Extend reconciliation case types for financial correctness.
ALTER TABLE commerce_reconciliation_cases DROP CONSTRAINT IF EXISTS commerce_reconciliation_cases_case_type_check;
ALTER TABLE commerce_reconciliation_cases ADD CONSTRAINT commerce_reconciliation_cases_case_type_check CHECK (
    case_type IN (
        'payment_paid_vend_not_started',
        'payment_paid_vend_failed',
        'vend_started_no_terminal_ack',
        'refund_pending_too_long',
        'webhook_provider_mismatch',
        'duplicate_provider_event',
        'duplicate_payment',
        'webhook_amount_currency_mismatch',
        'webhook_after_terminal_order',
        'settlement_amount_mismatch',
        'duplicate_payment_ambiguous_winner',
        'late_capture_refund_required',
        'legacy_cash_confirm_unknown_consent',
        'cash_gross_mismatch',
        'change_liability_unresolved'
    )
);

-- Backfill single-captured-payment winners; ambiguous orders left null for manual review.
WITH single_captured AS (
    SELECT order_id, min(id::text)::uuid AS payment_id, max(updated_at) AS claimed_at
    FROM payments
    WHERE state = 'captured'
    GROUP BY order_id
    HAVING count(*) = 1
)
UPDATE orders o
SET
    winning_payment_id = sc.payment_id,
    winning_claimed_at = sc.claimed_at
FROM single_captured sc
WHERE o.id = sc.order_id
    AND o.winning_payment_id IS NULL;

UPDATE payments p
SET outcome = 'winner'
FROM orders o
WHERE o.winning_payment_id = p.id
    AND p.outcome = 'pending';

-- Open reconciliation cases for orders with multiple captured payments.
INSERT INTO commerce_reconciliation_cases (
    case_type,
    severity,
    status,
    order_id,
    payment_id,
    reason,
    correlation_key,
    metadata
)
SELECT
    'duplicate_payment_ambiguous_winner',
    'critical',
    'open',
    dup.order_id,
    dup.payment_id,
    'Multiple captured payments on order; winning_payment_id not backfilled',
    'financial_correctness:ambiguous_winner:' || dup.order_id::text,
    jsonb_build_object('captured_count', dup.cnt)
FROM (
    SELECT order_id, min(id::text)::uuid AS payment_id, count(*) AS cnt
    FROM payments
    WHERE state = 'captured'
    GROUP BY order_id
    HAVING count(*) > 1
) dup
WHERE NOT EXISTS (
    SELECT 1
    FROM commerce_reconciliation_cases crc
    WHERE crc.correlation_key = 'financial_correctness:ambiguous_winner:' || dup.order_id::text
        AND crc.status IN ('open', 'reviewing', 'escalated')
);

-- +goose Down
DELETE FROM commerce_reconciliation_cases
WHERE case_type = 'duplicate_payment_ambiguous_winner';

UPDATE payments SET outcome = 'pending' WHERE outcome = 'winner';
UPDATE orders SET winning_payment_id = NULL, winning_claimed_at = NULL;

DROP TABLE IF EXISTS cash_change_events;
DROP TABLE IF EXISTS cash_allocations;
DROP TABLE IF EXISTS cash_acceptance_events;

DROP INDEX IF EXISTS ix_payments_order_outcome;
ALTER TABLE payments DROP COLUMN IF EXISTS supersedes_payment_id;
ALTER TABLE payments DROP COLUMN IF EXISTS attempt_seq;
ALTER TABLE payments DROP COLUMN IF EXISTS outcome;

DROP INDEX IF EXISTS ux_orders_winning_payment_id;
ALTER TABLE orders DROP COLUMN IF EXISTS winning_claimed_at;
ALTER TABLE orders DROP COLUMN IF EXISTS winning_payment_id;

ALTER TABLE financial_ledger_entries DROP CONSTRAINT IF EXISTS financial_ledger_entries_entry_type_check;
ALTER TABLE financial_ledger_entries ADD CONSTRAINT financial_ledger_entries_entry_type_check CHECK (
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
);
