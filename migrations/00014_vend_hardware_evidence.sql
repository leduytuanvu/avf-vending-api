-- Vend hardware evidence correlation + verification_status on vend_sessions (protocol hardening).
-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS vend_hardware_evidence (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v7(),
    order_id uuid NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    vend_session_id uuid NOT NULL REFERENCES vend_sessions (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE RESTRICT,
    slot_index int NOT NULL,
    vend_attempt_id uuid NOT NULL,
    correlation_id uuid NOT NULL,
    command_id text NOT NULL,
    evidence_digest text NOT NULL DEFAULT '',
    raw jsonb NOT NULL DEFAULT '{}'::jsonb,
    dedupe_key text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_vend_hardware_evidence_dedupe ON vend_hardware_evidence (dedupe_key);

CREATE INDEX IF NOT EXISTS ix_vend_hardware_evidence_order ON vend_hardware_evidence (order_id, created_at DESC);

CREATE INDEX IF NOT EXISTS ix_vend_hardware_evidence_vend_session ON vend_hardware_evidence (vend_session_id);

ALTER TABLE vend_sessions
ADD COLUMN IF NOT EXISTS verification_status text NOT NULL DEFAULT 'unverified';

ALTER TABLE vend_sessions DROP CONSTRAINT IF EXISTS chk_vend_sessions_verification_status;

ALTER TABLE vend_sessions
ADD CONSTRAINT chk_vend_sessions_verification_status CHECK (
    verification_status IN ('unverified', 'verified', 'hardware_unverified')
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE vend_sessions DROP CONSTRAINT IF EXISTS chk_vend_sessions_verification_status;

ALTER TABLE vend_sessions DROP COLUMN IF EXISTS verification_status;

DROP TABLE IF EXISTS vend_hardware_evidence;
-- +goose StatementEnd
