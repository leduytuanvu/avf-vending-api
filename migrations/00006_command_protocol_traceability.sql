-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- command_ledger: protocol / timeout / audit metadata (existing rows default)
-- ---------------------------------------------------------------------------
ALTER TABLE command_ledger
    ADD COLUMN protocol_type text,
    ADD COLUMN deadline_at timestamptz,
    ADD COLUMN timeout_at timestamptz,
    ADD COLUMN attempt_count int NOT NULL DEFAULT 0,
    ADD COLUMN last_attempt_at timestamptz,
    ADD COLUMN route_key text,
    ADD COLUMN source_system text,
    ADD COLUMN source_event_id text;

COMMENT ON COLUMN command_ledger.protocol_type IS 'Transport/protocol family, e.g. mqtt, dex, mcb, vendor_specific.';
COMMENT ON COLUMN command_ledger.deadline_at IS 'Business SLA deadline for command outcome.';
COMMENT ON COLUMN command_ledger.timeout_at IS 'Transport-layer timeout for acknowledgement.';
COMMENT ON COLUMN command_ledger.attempt_count IS 'Number of send attempts tracked in machine_command_attempts.';
COMMENT ON COLUMN command_ledger.last_attempt_at IS 'Timestamp of the latest machine_command_attempts row.';
COMMENT ON COLUMN command_ledger.route_key IS 'Broker shard / topic suffix for routing.';
COMMENT ON COLUMN command_ledger.source_system IS 'Upstream producer (outbox, webhook, admin UI, etc.).';
COMMENT ON COLUMN command_ledger.source_event_id IS 'Opaque id from source_system for cross-system trace.';

CREATE INDEX ix_command_ledger_machine_created ON command_ledger (machine_id, created_at DESC);
CREATE INDEX ix_command_ledger_correlation_id ON command_ledger (correlation_id)
    WHERE correlation_id IS NOT NULL;

COMMENT ON TABLE command_ledger IS 'Authoritative machine command rows (sequence = device monotonic id). Trace: ledger -> machine_command_attempts -> transport/raw/ack -> device_command_receipts; correlate with vend_sessions / orders via correlation_id and time.';

-- ---------------------------------------------------------------------------
-- machine_modules
-- ---------------------------------------------------------------------------
CREATE TABLE machine_modules (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    module_kind text NOT NULL CHECK (
        module_kind IN (
            'vend_motor',
            'bill_validator',
            'coin',
            'board',
            'remote',
            'display',
            'sensor',
            'other'
        )
    ),
    module_code text,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_machine_modules_module_code_nonempty CHECK (module_code IS NULL OR btrim(module_code) <> '')
);

CREATE UNIQUE INDEX ux_machine_modules_machine_kind_code ON machine_modules (machine_id, module_kind, module_code)
    WHERE module_code IS NOT NULL;

CREATE UNIQUE INDEX ux_machine_modules_machine_kind_default ON machine_modules (machine_id, module_kind)
    WHERE module_code IS NULL;

CREATE INDEX ix_machine_modules_machine_id ON machine_modules (machine_id);

COMMENT ON TABLE machine_modules IS 'Physical or logical sub-units on a machine (coin, motor bank, etc.).';

-- ---------------------------------------------------------------------------
-- machine_transport_sessions
-- ---------------------------------------------------------------------------
CREATE TABLE machine_transport_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    protocol_type text NOT NULL,
    transport_type text NOT NULL,
    client_id text,
    bridge_id text,
    connected_at timestamptz NOT NULL,
    disconnected_at timestamptz,
    disconnect_reason text,
    session_metadata jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX ix_machine_transport_sessions_machine_connected ON machine_transport_sessions (machine_id, connected_at DESC);
CREATE INDEX ix_machine_transport_sessions_active ON machine_transport_sessions (machine_id)
    WHERE disconnected_at IS NULL;

COMMENT ON COLUMN machine_transport_sessions.transport_type IS 'e.g. mqtt, websocket, serial_bridge.';
COMMENT ON TABLE machine_transport_sessions IS 'One logical connection from edge to cloud for correlation of attempts and raw frames.';

-- ---------------------------------------------------------------------------
-- machine_command_attempts (per-send / retry)
-- ---------------------------------------------------------------------------
CREATE TABLE machine_command_attempts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    command_id uuid NOT NULL REFERENCES command_ledger (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    transport_session_id uuid REFERENCES machine_transport_sessions (id) ON DELETE SET NULL,
    attempt_no int NOT NULL CHECK (attempt_no >= 1),
    sent_at timestamptz NOT NULL,
    ack_deadline_at timestamptz,
    acked_at timestamptz,
    result_received_at timestamptz,
    status text NOT NULL CHECK (
        status IN (
            'pending',
            'sent',
            'ack_timeout',
            'nack',
            'completed',
            'failed',
            'duplicate',
            'late'
        )
    ),
    timeout_reason text,
    protocol_pack_no bigint,
    sequence_no bigint,
    correlation_id uuid,
    request_payload_json jsonb,
    raw_request bytea,
    raw_response bytea,
    latency_ms int,
    CONSTRAINT ux_machine_command_attempts_command_attempt UNIQUE (command_id, attempt_no)
);

CREATE INDEX ix_machine_command_attempts_command_attempt ON machine_command_attempts (command_id, attempt_no);
CREATE INDEX ix_machine_command_attempts_machine_sent ON machine_command_attempts (machine_id, sent_at DESC);
CREATE INDEX ix_machine_command_attempts_transport_sent ON machine_command_attempts (transport_session_id, sent_at DESC);
CREATE INDEX ix_machine_command_attempts_correlation ON machine_command_attempts (correlation_id)
    WHERE correlation_id IS NOT NULL;

COMMENT ON TABLE machine_command_attempts IS 'Per-send attempt for a command_ledger row; machine_id denormalized for index locality—must match parent command row (enforced in application).';
COMMENT ON COLUMN machine_command_attempts.raw_request IS 'Prefer bytea for binary protocols; use request_payload_json when parsed.';
COMMENT ON COLUMN machine_command_attempts.raw_response IS 'Raw wire-level response bytes when applicable.';

-- ---------------------------------------------------------------------------
-- device_command_receipts: link receipt to attempt
-- ---------------------------------------------------------------------------
ALTER TABLE device_command_receipts
    ADD COLUMN command_attempt_id uuid,
    ADD CONSTRAINT fk_device_command_receipts_command_attempt FOREIGN KEY (command_attempt_id)
        REFERENCES machine_command_attempts (id) ON DELETE SET NULL;

CREATE INDEX ix_device_command_receipts_machine_received ON device_command_receipts (machine_id, received_at DESC);
CREATE INDEX ix_device_command_receipts_correlation ON device_command_receipts (correlation_id)
    WHERE correlation_id IS NOT NULL;
CREATE INDEX ix_device_command_receipts_command_attempt ON device_command_receipts (command_attempt_id)
    WHERE command_attempt_id IS NOT NULL;

COMMENT ON COLUMN device_command_receipts.command_attempt_id IS 'Optional link to the machine_command_attempts row this receipt answers.';

-- ---------------------------------------------------------------------------
-- vend_sessions: tie outcome to motor attempt when known
-- ---------------------------------------------------------------------------
ALTER TABLE vend_sessions
    ADD COLUMN final_command_attempt_id uuid,
    ADD CONSTRAINT fk_vend_sessions_final_command_attempt FOREIGN KEY (final_command_attempt_id)
        REFERENCES machine_command_attempts (id) ON DELETE SET NULL;

CREATE INDEX ix_vend_sessions_final_command_attempt ON vend_sessions (final_command_attempt_id)
    WHERE final_command_attempt_id IS NOT NULL;

COMMENT ON COLUMN vend_sessions.correlation_id IS 'Cross-link to command_ledger.correlation_id and orders for payment→vend traces.';
COMMENT ON COLUMN vend_sessions.final_command_attempt_id IS 'Set when vend outcome is tied to a specific command attempt; NULL when inferred without command trace.';

-- ---------------------------------------------------------------------------
-- device_messages_raw (immutable append-only; no UPDATE policy in app)
-- ---------------------------------------------------------------------------
CREATE TABLE device_messages_raw (
    id bigserial PRIMARY KEY,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    module_id uuid REFERENCES machine_modules (id) ON DELETE SET NULL,
    transport_session_id uuid REFERENCES machine_transport_sessions (id) ON DELETE SET NULL,
    direction text NOT NULL CHECK (direction IN ('inbound', 'outbound')),
    protocol_type text NOT NULL,
    message_type text NOT NULL,
    correlation_id uuid,
    pack_no bigint,
    sequence_no bigint,
    payload_json jsonb,
    raw_payload bytea,
    message_hash bytea NOT NULL,
    occurred_at timestamptz NOT NULL
);

CREATE INDEX ix_device_messages_raw_machine_occurred ON device_messages_raw (machine_id, occurred_at DESC);
CREATE INDEX ix_device_messages_raw_correlation_occurred ON device_messages_raw (correlation_id, occurred_at DESC)
    WHERE correlation_id IS NOT NULL;
CREATE INDEX ix_device_messages_raw_transport_occurred ON device_messages_raw (transport_session_id, occurred_at DESC)
    WHERE transport_session_id IS NOT NULL;
CREATE INDEX ix_device_messages_raw_machine_proto_seq ON device_messages_raw (machine_id, protocol_type, pack_no, sequence_no)
    WHERE pack_no IS NOT NULL;
CREATE INDEX ix_device_messages_raw_message_hash ON device_messages_raw (machine_id, message_hash, occurred_at);

COMMENT ON TABLE device_messages_raw IS 'Immutable raw protocol log; prefer raw_payload bytea when JSON is not representative. Application: INSERT + SELECT only (no UPDATE). Dedup analysis via message_hash (non-unique).';
COMMENT ON COLUMN device_messages_raw.message_hash IS 'SHA-256 digest (32 bytes) of canonical wire bytes for forensics.';

-- ---------------------------------------------------------------------------
-- protocol_ack_events
-- ---------------------------------------------------------------------------
CREATE TABLE protocol_ack_events (
    id bigserial PRIMARY KEY,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    command_attempt_id uuid REFERENCES machine_command_attempts (id) ON DELETE SET NULL,
    raw_message_id bigint REFERENCES device_messages_raw (id) ON DELETE SET NULL,
    device_receipt_id bigint REFERENCES device_command_receipts (id) ON DELETE SET NULL,
    event_type text NOT NULL CHECK (event_type IN ('ack', 'nack', 'timeout', 'retry_scheduled', 'inferred')),
    occurred_at timestamptz NOT NULL,
    latency_ms int,
    details jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX ix_protocol_ack_events_attempt_occurred ON protocol_ack_events (command_attempt_id, occurred_at);
CREATE INDEX ix_protocol_ack_events_machine_occurred ON protocol_ack_events (machine_id, occurred_at DESC);
CREATE INDEX ix_protocol_ack_events_raw_message ON protocol_ack_events (raw_message_id)
    WHERE raw_message_id IS NOT NULL;

COMMENT ON TABLE protocol_ack_events IS 'Low-level ack/nack/timeout for retry analysis; join to attempts, raw rows, or device_command_receipts.';
COMMENT ON TABLE device_command_receipts IS 'Device-reported outcome for a command sequence; optional command_attempt_id links to the send being acknowledged.';

COMMENT ON TABLE vend_sessions IS 'Field debug: payment ok but vend unclear—join orders/payments to machine_command_attempts and device_messages_raw by correlation_id and time window.';

/*
Field-debugging scenarios (SQL / joins):
- Payment ok, vend unclear: vend_sessions → orders/payments → machine_command_attempts / device_messages_raw by correlation_id and time window.
- Ack without completion: protocol_ack_events + machine_command_attempts with acked_at set and result_received_at null past SLA.
- Duplicate / late / replayed: device_messages_raw immutability + message_hash + protocol_ack_events.event_type and attempt status duplicate|late.
*/

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS protocol_ack_events;

DROP TABLE IF EXISTS device_messages_raw;

ALTER TABLE vend_sessions DROP CONSTRAINT IF EXISTS fk_vend_sessions_final_command_attempt;
ALTER TABLE vend_sessions DROP COLUMN IF EXISTS final_command_attempt_id;

DROP INDEX IF EXISTS ix_device_command_receipts_command_attempt;
DROP INDEX IF EXISTS ix_device_command_receipts_correlation;
DROP INDEX IF EXISTS ix_device_command_receipts_machine_received;

ALTER TABLE device_command_receipts DROP CONSTRAINT IF EXISTS fk_device_command_receipts_command_attempt;
ALTER TABLE device_command_receipts DROP COLUMN IF EXISTS command_attempt_id;

DROP TABLE IF EXISTS machine_command_attempts;
DROP TABLE IF EXISTS machine_transport_sessions;
DROP TABLE IF EXISTS machine_modules;

DROP INDEX IF EXISTS ix_command_ledger_correlation_id;
DROP INDEX IF EXISTS ix_command_ledger_machine_created;

ALTER TABLE command_ledger DROP COLUMN IF EXISTS protocol_type;
ALTER TABLE command_ledger DROP COLUMN IF EXISTS deadline_at;
ALTER TABLE command_ledger DROP COLUMN IF EXISTS timeout_at;
ALTER TABLE command_ledger DROP COLUMN IF EXISTS attempt_count;
ALTER TABLE command_ledger DROP COLUMN IF EXISTS last_attempt_at;
ALTER TABLE command_ledger DROP COLUMN IF EXISTS route_key;
ALTER TABLE command_ledger DROP COLUMN IF EXISTS source_system;
ALTER TABLE command_ledger DROP COLUMN IF EXISTS source_event_id;

-- +goose StatementEnd
