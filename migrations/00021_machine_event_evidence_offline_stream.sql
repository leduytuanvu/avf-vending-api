-- Durable semantic evidence ledger + stream-aware offline mutation identity.
-- +goose Up
-- +goose StatementBegin

ALTER TABLE machine_offline_events
    ADD COLUMN IF NOT EXISTS request_id text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS stream_id text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS payload_fingerprint text NOT NULL DEFAULT '';

ALTER TABLE machine_offline_events
    DROP CONSTRAINT IF EXISTS machine_offline_events_processing_status_check;

ALTER TABLE machine_offline_events
    ADD CONSTRAINT machine_offline_events_processing_status_check CHECK (
        processing_status IN (
            'pending',
            'processing',
            'processed',
            'succeeded',
            'failed',
            'duplicate',
            'replayed',
            'rejected',
            'manual_reconciliation'
        )
    );

DROP INDEX IF EXISTS ux_machine_offline_events_machine_sequence;

CREATE UNIQUE INDEX IF NOT EXISTS ux_machine_offline_events_machine_stream_sequence
    ON machine_offline_events (machine_id, stream_id, offline_sequence);

DROP INDEX IF EXISTS ix_machine_offline_events_retention_terminal_received_at;

CREATE INDEX ix_machine_offline_events_retention_terminal_received_at
    ON machine_offline_events (received_at ASC)
WHERE
    processing_status IN (
        'processed',
        'succeeded',
        'duplicate',
        'replayed'
    );

CREATE TABLE IF NOT EXISTS machine_event_evidence (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v7(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    event_id text NOT NULL,
    event_type text NOT NULL,
    schema_version integer NOT NULL DEFAULT 1,
    category text NOT NULL DEFAULT '',
    severity text NOT NULL DEFAULT '',
    source text NOT NULL DEFAULT 'device',
    stream_id text NOT NULL DEFAULT '',
    client_sequence bigint NOT NULL DEFAULT 0,
    boot_id text NOT NULL DEFAULT '',
    occurred_at timestamptz NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now(),
    monotonic_elapsed_ms bigint NOT NULL DEFAULT 0,
    order_id text NOT NULL DEFAULT '',
    payment_id text NOT NULL DEFAULT '',
    vend_attempt_id text NOT NULL DEFAULT '',
    correlation_id text NOT NULL DEFAULT '',
    operator_session_id text NOT NULL DEFAULT '',
    request_id text NOT NULL DEFAULT '',
    cause text NOT NULL DEFAULT '',
    recovery_action text NOT NULL DEFAULT '',
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    payload_fingerprint text NOT NULL,
    processing_status text NOT NULL DEFAULT 'accepted' CHECK (
        processing_status IN (
            'accepted',
            'duplicate',
            'conflict',
            'rejected',
            'unrecognized'
        )
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_machine_event_evidence_machine_event
    ON machine_event_evidence (machine_id, event_id);

CREATE INDEX IF NOT EXISTS ix_machine_event_evidence_machine_occurred
    ON machine_event_evidence (machine_id, occurred_at DESC);

CREATE INDEX IF NOT EXISTS ix_machine_event_evidence_machine_received
    ON machine_event_evidence (machine_id, received_at DESC);

CREATE INDEX IF NOT EXISTS ix_machine_event_evidence_order
    ON machine_event_evidence (machine_id, order_id)
WHERE
    btrim(order_id) <> '';

CREATE INDEX IF NOT EXISTS ix_machine_event_evidence_payment
    ON machine_event_evidence (machine_id, payment_id)
WHERE
    btrim(payment_id) <> '';

CREATE INDEX IF NOT EXISTS ix_machine_event_evidence_vend
    ON machine_event_evidence (machine_id, vend_attempt_id)
WHERE
    btrim(vend_attempt_id) <> '';

CREATE INDEX IF NOT EXISTS ix_machine_event_evidence_correlation
    ON machine_event_evidence (machine_id, correlation_id)
WHERE
    btrim(correlation_id) <> '';

COMMENT ON TABLE machine_event_evidence IS 'Append-only durable semantic device evidence. Unique per (machine_id, event_id). ACK means this row is committed.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS machine_event_evidence;

DROP INDEX IF EXISTS ux_machine_offline_events_machine_stream_sequence;

CREATE UNIQUE INDEX IF NOT EXISTS ux_machine_offline_events_machine_sequence
    ON machine_offline_events (machine_id, offline_sequence);

ALTER TABLE machine_offline_events
    DROP CONSTRAINT IF EXISTS machine_offline_events_processing_status_check;

ALTER TABLE machine_offline_events
    ADD CONSTRAINT machine_offline_events_processing_status_check CHECK (
        processing_status IN (
            'pending',
            'processing',
            'processed',
            'succeeded',
            'failed',
            'duplicate',
            'replayed',
            'rejected'
        )
    );

ALTER TABLE machine_offline_events
    DROP COLUMN IF EXISTS payload_fingerprint,
    DROP COLUMN IF EXISTS stream_id,
    DROP COLUMN IF EXISTS request_id;

-- +goose StatementEnd
