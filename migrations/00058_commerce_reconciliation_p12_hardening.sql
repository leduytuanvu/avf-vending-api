-- +goose Up
-- P1.2: extend reconciliation cases for payment/vend/refund hardening (machine scope, extra case/status labels).

ALTER TABLE commerce_reconciliation_cases
    ADD COLUMN IF NOT EXISTS machine_id uuid REFERENCES machines (id) ON DELETE SET NULL;

-- Expand case_type labels (amount/currency mismatch from webhook; late webhook on terminal order).
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
        'webhook_after_terminal_order'
    )
);

ALTER TABLE commerce_reconciliation_cases DROP CONSTRAINT IF EXISTS commerce_reconciliation_cases_status_check;
ALTER TABLE commerce_reconciliation_cases ADD CONSTRAINT commerce_reconciliation_cases_status_check CHECK (
    status IN ('open', 'reviewing', 'resolved', 'dismissed', 'ignored', 'escalated')
);

DROP INDEX IF EXISTS ux_commerce_reconciliation_cases_open_identity;
CREATE UNIQUE INDEX ux_commerce_reconciliation_cases_open_identity
    ON commerce_reconciliation_cases (
        organization_id,
        case_type,
        COALESCE(order_id, '00000000-0000-0000-0000-000000000000'::uuid),
        COALESCE(payment_id, '00000000-0000-0000-0000-000000000000'::uuid),
        COALESCE(vend_session_id, '00000000-0000-0000-0000-000000000000'::uuid),
        COALESCE(refund_id, '00000000-0000-0000-0000-000000000000'::uuid),
        COALESCE(provider_event_id, 0)
    )
    WHERE status IN ('open', 'reviewing', 'escalated');

UPDATE commerce_reconciliation_cases crc
SET machine_id = o.machine_id
FROM orders o
WHERE crc.order_id = o.id
    AND crc.machine_id IS NULL;

-- +goose Down
DROP INDEX IF EXISTS ux_commerce_reconciliation_cases_open_identity;

CREATE UNIQUE INDEX ux_commerce_reconciliation_cases_open_identity
    ON commerce_reconciliation_cases (
        organization_id,
        case_type,
        COALESCE(order_id, '00000000-0000-0000-0000-000000000000'::uuid),
        COALESCE(payment_id, '00000000-0000-0000-0000-000000000000'::uuid),
        COALESCE(vend_session_id, '00000000-0000-0000-0000-000000000000'::uuid),
        COALESCE(refund_id, '00000000-0000-0000-0000-000000000000'::uuid),
        COALESCE(provider_event_id, 0)
    )
    WHERE status IN ('open', 'reviewing');

UPDATE commerce_reconciliation_cases SET status = 'resolved' WHERE status IN ('ignored', 'escalated');

ALTER TABLE commerce_reconciliation_cases DROP CONSTRAINT IF EXISTS commerce_reconciliation_cases_case_type_check;
ALTER TABLE commerce_reconciliation_cases ADD CONSTRAINT commerce_reconciliation_cases_case_type_check CHECK (
    case_type IN (
        'payment_paid_vend_not_started',
        'payment_paid_vend_failed',
        'vend_started_no_terminal_ack',
        'refund_pending_too_long',
        'webhook_provider_mismatch',
        'duplicate_provider_event',
        'duplicate_payment'
    )
);

ALTER TABLE commerce_reconciliation_cases DROP CONSTRAINT IF EXISTS commerce_reconciliation_cases_status_check;
ALTER TABLE commerce_reconciliation_cases ADD CONSTRAINT commerce_reconciliation_cases_status_check CHECK (
    status IN ('open', 'reviewing', 'resolved', 'dismissed')
);

ALTER TABLE commerce_reconciliation_cases DROP COLUMN IF EXISTS machine_id;
