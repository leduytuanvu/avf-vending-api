-- +goose Up
-- P2.3: additive indexes for provider webhook correlation, audit log timelines, and command ledger time scans.

CREATE INDEX IF NOT EXISTS ix_payment_provider_events_provider_ref_lookup ON payment_provider_events (provider, provider_ref, received_at DESC)
WHERE
    provider_ref IS NOT NULL
    AND btrim(provider_ref) <> '';

CREATE INDEX IF NOT EXISTS ix_audit_logs_organization_created_action ON audit_logs (organization_id, created_at DESC, action);

CREATE INDEX IF NOT EXISTS ix_command_ledger_machine_created_type ON command_ledger (machine_id, created_at DESC, command_type);

COMMENT ON INDEX ix_payment_provider_events_provider_ref_lookup IS 'P2.3: correlate inbound PSP events by stable provider reference.';
COMMENT ON INDEX ix_audit_logs_organization_created_action IS 'P2.3: tenant audit_logs timelines (legacy table).';
COMMENT ON INDEX ix_command_ledger_machine_created_type IS 'P2.3: command ledger history by machine and time.';

-- +goose Down

DROP INDEX IF EXISTS ix_command_ledger_machine_created_type;
DROP INDEX IF EXISTS ix_audit_logs_organization_created_action;
DROP INDEX IF EXISTS ix_payment_provider_events_provider_ref_lookup;
