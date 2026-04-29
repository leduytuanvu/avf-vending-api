-- +goose Up
CREATE TABLE IF NOT EXISTS commerce_reconciliation_cases (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    case_type text NOT NULL CHECK (
        case_type IN (
            'payment_paid_vend_not_started',
            'payment_paid_vend_failed',
            'vend_started_no_terminal_ack',
            'refund_pending_too_long',
            'webhook_provider_mismatch',
            'duplicate_provider_event',
            'duplicate_payment'
        )
    ),
    status text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'reviewing', 'resolved', 'dismissed')),
    severity text NOT NULL DEFAULT 'warning' CHECK (severity IN ('info', 'warning', 'critical')),
    order_id uuid REFERENCES orders (id) ON DELETE SET NULL,
    payment_id uuid REFERENCES payments (id) ON DELETE SET NULL,
    vend_session_id uuid REFERENCES vend_sessions (id) ON DELETE SET NULL,
    refund_id uuid REFERENCES refunds (id) ON DELETE SET NULL,
    provider text,
    provider_event_id bigint REFERENCES payment_provider_events (id) ON DELETE SET NULL,
    reason text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    first_detected_at timestamptz NOT NULL DEFAULT now(),
    last_detected_at timestamptz NOT NULL DEFAULT now(),
    resolved_at timestamptz,
    resolved_by uuid REFERENCES platform_auth_accounts (id) ON DELETE SET NULL,
    resolution_note text
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_commerce_reconciliation_cases_open_identity
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

CREATE INDEX IF NOT EXISTS ix_commerce_reconciliation_cases_org_status
    ON commerce_reconciliation_cases (organization_id, status, last_detected_at DESC);

CREATE INDEX IF NOT EXISTS ix_commerce_reconciliation_cases_payment
    ON commerce_reconciliation_cases (payment_id, last_detected_at DESC)
    WHERE payment_id IS NOT NULL;

COMMENT ON TABLE commerce_reconciliation_cases IS 'Operator-visible payment/vend/refund reconciliation queue. Redis never stores authoritative case state.';

-- +goose Down
DROP TABLE IF EXISTS commerce_reconciliation_cases;
