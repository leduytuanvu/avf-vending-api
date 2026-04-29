-- +goose Up
-- P2.5 compliance retention/privacy hardening.
-- Additive legal-hold flags prevent retention workers from purging audit/payment evidence under investigation.

ALTER TABLE audit_events
    ADD COLUMN IF NOT EXISTS legal_hold boolean NOT NULL DEFAULT false;

ALTER TABLE audit_logs
    ADD COLUMN IF NOT EXISTS legal_hold boolean NOT NULL DEFAULT false;

ALTER TABLE payment_provider_events
    ADD COLUMN IF NOT EXISTS legal_hold boolean NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS ix_audit_events_legal_hold_created
    ON audit_events (legal_hold, created_at);

CREATE INDEX IF NOT EXISTS ix_audit_logs_legal_hold_created
    ON audit_logs (legal_hold, created_at);

CREATE INDEX IF NOT EXISTS ix_payment_provider_events_legal_hold_received
    ON payment_provider_events (legal_hold, received_at);

COMMENT ON COLUMN audit_events.legal_hold IS 'When true, retention cleanup must not purge this enterprise audit event.';
COMMENT ON COLUMN audit_logs.legal_hold IS 'When true, retention cleanup must not purge this legacy audit log row.';
COMMENT ON COLUMN payment_provider_events.legal_hold IS 'When true, retention cleanup must not purge this PSP webhook evidence.';

-- +goose Down
DROP INDEX IF EXISTS ix_payment_provider_events_legal_hold_received;
DROP INDEX IF EXISTS ix_audit_logs_legal_hold_created;
DROP INDEX IF EXISTS ix_audit_events_legal_hold_created;

ALTER TABLE payment_provider_events
    DROP COLUMN IF EXISTS legal_hold;

ALTER TABLE audit_logs
    DROP COLUMN IF EXISTS legal_hold;

ALTER TABLE audit_events
    DROP COLUMN IF EXISTS legal_hold;
