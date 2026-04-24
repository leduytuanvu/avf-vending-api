-- +goose Up
-- +goose StatementBegin

CREATE TABLE machine_activation_codes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    code_hash bytea NOT NULL,
    max_uses int NOT NULL DEFAULT 1 CHECK (max_uses > 0),
    uses int NOT NULL DEFAULT 0 CHECK (uses >= 0),
    expires_at timestamptz NOT NULL,
    notes text,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked', 'expired')),
    claimed_fingerprint_hash bytea,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ux_machine_activation_codes_hash ON machine_activation_codes (code_hash);

CREATE INDEX ix_machine_activation_codes_machine ON machine_activation_codes (machine_id, created_at DESC);

COMMENT ON TABLE machine_activation_codes IS 'Hashed kiosk activation codes; plaintext code is shown once at creation.';

CREATE TABLE critical_telemetry_event_status (
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    idempotency_key text NOT NULL,
    status text NOT NULL CHECK (
        status IN (
            'accepted',
            'processing',
            'processed',
            'failed_retryable',
            'failed_terminal'
        )
    ),
    event_type text,
    accepted_at timestamptz,
    processed_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now (),
    PRIMARY KEY (machine_id, idempotency_key)
);

CREATE INDEX ix_critical_telemetry_machine_status ON critical_telemetry_event_status (machine_id, status);

COMMENT ON TABLE critical_telemetry_event_status IS 'Device-visible projection status for critical telemetry idempotency keys (per machine).';

ALTER TABLE refunds
    ADD COLUMN IF NOT EXISTS currency char(3);

UPDATE refunds r
SET
    currency = p.currency
FROM
    payments p
WHERE
    r.payment_id = p.id
    AND r.currency IS NULL;

UPDATE refunds
SET
    currency = 'USD'
WHERE
    currency IS NULL;

ALTER TABLE refunds
    ALTER COLUMN currency SET NOT NULL;

ALTER TABLE refunds
    ADD COLUMN IF NOT EXISTS idempotency_key text;

ALTER TABLE refunds
    ADD COLUMN IF NOT EXISTS metadata jsonb NOT NULL DEFAULT '{}'::jsonb;

CREATE UNIQUE INDEX ux_refunds_order_idempotency ON refunds (order_id, idempotency_key)
WHERE
    idempotency_key IS NOT NULL
    AND btrim(idempotency_key) <> '';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS ux_refunds_order_idempotency;

ALTER TABLE refunds
    DROP COLUMN IF EXISTS metadata;

ALTER TABLE refunds
    DROP COLUMN IF EXISTS idempotency_key;

ALTER TABLE refunds
    DROP COLUMN IF EXISTS currency;

DROP TABLE IF EXISTS critical_telemetry_event_status;

DROP TABLE IF EXISTS machine_activation_codes;

-- +goose StatementEnd
