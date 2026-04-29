-- +goose Up
-- P2.5: additive capacity indexes for hot reporting, provider correlation, worker scans, and audit time queries.

CREATE INDEX IF NOT EXISTS ix_orders_organization_machine_created_at ON orders (organization_id, machine_id, created_at DESC);

CREATE INDEX IF NOT EXISTS ix_payment_attempts_provider_reference ON payment_attempts (provider_reference)
WHERE
    provider_reference IS NOT NULL
    AND btrim(provider_reference) <> '';

CREATE INDEX IF NOT EXISTS ix_outbox_events_status_next_publish ON outbox_events (status, next_publish_after, id)
WHERE
    published_at IS NULL
    AND dead_lettered_at IS NULL;

CREATE INDEX IF NOT EXISTS ix_audit_events_organization_occurred_action ON audit_events (organization_id, occurred_at DESC, action);

CREATE INDEX IF NOT EXISTS ix_machine_command_attempts_machine_status_sent ON machine_command_attempts (machine_id, status, sent_at DESC);

CREATE INDEX IF NOT EXISTS ix_device_telemetry_machine_event_received ON device_telemetry_events (machine_id, event_type, received_at DESC);

COMMENT ON INDEX ix_orders_organization_machine_created_at IS 'P2.5: tenant + machine reporting windows on orders.created_at.';
COMMENT ON INDEX ix_payment_attempts_provider_reference IS 'P2.5: correlate payment attempts by PSP reference.';
COMMENT ON INDEX ix_outbox_events_status_next_publish IS 'P2.5: due outbox rows by status and next retry time.';
COMMENT ON INDEX ix_audit_events_organization_occurred_action IS 'P2.5: tenant audit timelines by occurred_at.';
COMMENT ON INDEX ix_machine_command_attempts_machine_status_sent IS 'P2.5: command transport diagnostics by machine, status, time.';
COMMENT ON INDEX ix_device_telemetry_machine_event_received IS 'P2.5: device telemetry drill-down by machine, event_type, time.';

-- +goose Down

DROP INDEX IF EXISTS ix_device_telemetry_machine_event_received;
DROP INDEX IF EXISTS ix_machine_command_attempts_machine_status_sent;
DROP INDEX IF EXISTS ix_audit_events_organization_occurred_action;
DROP INDEX IF EXISTS ix_outbox_events_status_next_publish;
DROP INDEX IF EXISTS ix_payment_attempts_provider_reference;
DROP INDEX IF EXISTS ix_orders_organization_machine_created_at;
