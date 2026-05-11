-- +goose Up
-- P1.2: payment webhook ingress metadata, provider settlements, disputes, reconciliation correlation keys.

-- Raw ingress / signature bookkeeping on PSP notifications.
ALTER TABLE payment_provider_events DROP CONSTRAINT IF EXISTS chk_payment_provider_events_validation_status;
ALTER TABLE payment_provider_events ADD CONSTRAINT chk_payment_provider_events_validation_status CHECK (
    validation_status IN ('hmac_verified', 'unsigned_development', 'invalid_signature')
);

ALTER TABLE payment_provider_events
    ADD COLUMN IF NOT EXISTS organization_id uuid REFERENCES organizations (id) ON DELETE SET NULL;
ALTER TABLE payment_provider_events
    ADD COLUMN IF NOT EXISTS signature_valid boolean NOT NULL DEFAULT true;
ALTER TABLE payment_provider_events
    ADD COLUMN IF NOT EXISTS applied_at timestamptz;
ALTER TABLE payment_provider_events
    ADD COLUMN IF NOT EXISTS ingress_status text NOT NULL DEFAULT 'applied'
        CONSTRAINT chk_payment_provider_events_ingress_status CHECK (
            ingress_status IN ('received', 'applied', 'failed', 'replay_skipped')
        );
ALTER TABLE payment_provider_events
    ADD COLUMN IF NOT EXISTS ingress_error text;

UPDATE payment_provider_events e
SET organization_id = o.organization_id
FROM payments p
JOIN orders o ON o.id = p.order_id
WHERE e.payment_id = p.id
    AND e.organization_id IS NULL;

UPDATE payment_provider_events
SET applied_at = received_at
WHERE applied_at IS NULL
    AND ingress_status = 'applied';

COMMENT ON COLUMN payment_provider_events.signature_valid IS 'Whether HTTP-layer signature verification succeeded before persistence.';
COMMENT ON COLUMN payment_provider_events.applied_at IS 'When webhook processing successfully finished (payment state / side effects committed).';
COMMENT ON COLUMN payment_provider_events.ingress_status IS 'Ingress/processing outcome for audit and replay diagnostics.';
COMMENT ON COLUMN payment_provider_events.ingress_error IS 'When ingress_status is failed, short operator-safe error text.';

CREATE INDEX IF NOT EXISTS ix_payment_provider_events_org_received
    ON payment_provider_events (organization_id, received_at DESC)
    WHERE organization_id IS NOT NULL;

-- Operator queue: allow multiple open cases of the same type when correlation_key differs (e.g. settlement batches).
ALTER TABLE commerce_reconciliation_cases
    ADD COLUMN IF NOT EXISTS correlation_key text NOT NULL DEFAULT '';

DROP INDEX IF EXISTS ux_commerce_reconciliation_cases_open_identity;
CREATE UNIQUE INDEX ux_commerce_reconciliation_cases_open_identity
    ON commerce_reconciliation_cases (
        organization_id,
        case_type,
        COALESCE(order_id, '00000000-0000-0000-0000-000000000000'::uuid),
        COALESCE(payment_id, '00000000-0000-0000-0000-000000000000'::uuid),
        COALESCE(vend_session_id, '00000000-0000-0000-0000-000000000000'::uuid),
        COALESCE(refund_id, '00000000-0000-0000-0000-000000000000'::uuid),
        COALESCE(provider_event_id, 0),
        correlation_key
    )
    WHERE status IN ('open', 'reviewing', 'escalated');

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
        'settlement_amount_mismatch'
    )
);

CREATE TABLE IF NOT EXISTS payment_provider_settlements (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    provider text NOT NULL,
    provider_settlement_id text NOT NULL,
    gross_amount_minor bigint NOT NULL,
    fee_amount_minor bigint NOT NULL DEFAULT 0,
    net_amount_minor bigint NOT NULL,
    currency char(3) NOT NULL,
    settlement_date date NOT NULL,
    transaction_refs jsonb NOT NULL DEFAULT '[]'::jsonb,
    status text NOT NULL DEFAULT 'imported' CHECK (
        status IN ('imported', 'reconciled', 'mismatch_flagged')
    ),
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now (),
    updated_at timestamptz NOT NULL DEFAULT now (),
    CONSTRAINT ux_payment_provider_settlements_org_provider_ext UNIQUE (organization_id, provider, provider_settlement_id)
);

CREATE INDEX IF NOT EXISTS ix_payment_provider_settlements_org_date
    ON payment_provider_settlements (organization_id, settlement_date DESC, created_at DESC);

CREATE TABLE IF NOT EXISTS payment_disputes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    provider text NOT NULL,
    provider_dispute_id text NOT NULL,
    payment_id uuid REFERENCES payments (id) ON DELETE SET NULL,
    order_id uuid REFERENCES orders (id) ON DELETE SET NULL,
    amount_minor bigint NOT NULL,
    currency char(3) NOT NULL,
    reason text,
    status text NOT NULL DEFAULT 'opened' CHECK (
        status IN ('opened', 'under_review', 'won', 'lost', 'closed')
    ),
    opened_at timestamptz NOT NULL DEFAULT now (),
    resolved_at timestamptz,
    resolved_by uuid REFERENCES platform_auth_accounts (id) ON DELETE SET NULL,
    resolution_note text,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now (),
    updated_at timestamptz NOT NULL DEFAULT now (),
    CONSTRAINT ux_payment_disputes_org_provider_ext UNIQUE (organization_id, provider, provider_dispute_id)
);

CREATE INDEX IF NOT EXISTS ix_payment_disputes_org_status
    ON payment_disputes (organization_id, status, opened_at DESC);

COMMENT ON TABLE payment_provider_settlements IS 'Imported PSP settlement reports for finance reconciliation.';
COMMENT ON TABLE payment_disputes IS 'Chargeback/dispute foundation; links to internal order/payment when known.';

DROP VIEW IF EXISTS payment_reconciliation_cases;

CREATE OR REPLACE VIEW payment_reconciliation_cases AS
SELECT
    crc.id,
    crc.organization_id,
    crc.machine_id,
    crc.order_id,
    crc.payment_id,
    crc.provider,
    crc.case_type,
    crc.severity,
    crc.status,
    crc.reason,
    crc.metadata,
    crc.correlation_key,
    crc.first_detected_at AS created_at,
    crc.last_detected_at AS updated_at,
    crc.resolved_at,
    crc.resolved_by
FROM commerce_reconciliation_cases crc;

-- +goose Down
DROP VIEW IF EXISTS payment_reconciliation_cases;

CREATE OR REPLACE VIEW payment_reconciliation_cases AS
SELECT
    crc.id,
    crc.organization_id,
    crc.machine_id,
    crc.order_id,
    crc.payment_id,
    crc.provider,
    crc.case_type,
    crc.severity,
    crc.status,
    crc.reason,
    crc.metadata,
    crc.first_detected_at AS created_at,
    crc.last_detected_at AS updated_at,
    crc.resolved_at,
    crc.resolved_by
FROM commerce_reconciliation_cases crc;

DROP TABLE IF EXISTS payment_disputes;
DROP TABLE IF EXISTS payment_provider_settlements;

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

ALTER TABLE commerce_reconciliation_cases DROP COLUMN IF EXISTS correlation_key;

DROP INDEX IF EXISTS ix_payment_provider_events_org_received;
ALTER TABLE payment_provider_events DROP COLUMN IF EXISTS ingress_error;
ALTER TABLE payment_provider_events DROP COLUMN IF EXISTS ingress_status;
ALTER TABLE payment_provider_events DROP COLUMN IF EXISTS applied_at;
ALTER TABLE payment_provider_events DROP COLUMN IF EXISTS signature_valid;
ALTER TABLE payment_provider_events DROP COLUMN IF EXISTS organization_id;

ALTER TABLE payment_provider_events DROP CONSTRAINT IF EXISTS chk_payment_provider_events_validation_status;
ALTER TABLE payment_provider_events ADD CONSTRAINT chk_payment_provider_events_validation_status CHECK (
    validation_status IN ('hmac_verified', 'unsigned_development')
);
