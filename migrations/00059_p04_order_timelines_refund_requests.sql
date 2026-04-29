-- +goose Up
-- P0.4: durable order timeline + refund review requests + compatibility view for reconciliation cases.

CREATE TABLE IF NOT EXISTS order_timelines (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    order_id uuid NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    event_type text NOT NULL,
    actor_type text NOT NULL CHECK (
        actor_type IN ('system', 'machine', 'operator', 'webhook', 'admin')
    ),
    actor_id text,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    occurred_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_order_timelines_org_order_occurred ON order_timelines (organization_id, order_id, occurred_at DESC);

COMMENT ON TABLE order_timelines IS 'Append-only commerce order lifecycle events (reconciliation actions, refunds, operator visibility).';

CREATE TABLE IF NOT EXISTS refund_requests (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    order_id uuid NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    payment_id uuid REFERENCES payments (id) ON DELETE SET NULL,
    refund_id uuid REFERENCES refunds (id) ON DELETE SET NULL,
    amount_minor bigint NOT NULL CHECK (amount_minor >= 0),
    currency char(3) NOT NULL,
    reason text,
    status text NOT NULL DEFAULT 'requested' CHECK (
        status IN ('requested', 'approved', 'rejected', 'processing', 'succeeded', 'failed')
    ),
    provider_refund_id text,
    requested_by uuid REFERENCES platform_auth_accounts (id) ON DELETE SET NULL,
    approved_by uuid REFERENCES platform_auth_accounts (id) ON DELETE SET NULL,
    idempotency_key text,
    created_at timestamptz NOT NULL DEFAULT now (),
    updated_at timestamptz NOT NULL DEFAULT now (),
    completed_at timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_refund_requests_org_idempotency ON refund_requests (organization_id, idempotency_key)
WHERE
    idempotency_key IS NOT NULL
    AND btrim(idempotency_key) <> '';

CREATE INDEX IF NOT EXISTS ix_refund_requests_org_created ON refund_requests (organization_id, created_at DESC);

CREATE INDEX IF NOT EXISTS ix_refund_requests_org_order ON refund_requests (organization_id, order_id, created_at DESC);

COMMENT ON TABLE refund_requests IS 'Human-initiated refund review rows linked to ledger refunds.refunds after PSP processing.';

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

COMMENT ON VIEW payment_reconciliation_cases IS 'Compatibility projection over commerce_reconciliation_cases (canonical table).';

-- +goose Down
DROP VIEW IF EXISTS payment_reconciliation_cases;

DROP TABLE IF EXISTS refund_requests;

DROP TABLE IF EXISTS order_timelines;
