-- +goose Up
-- P2.2: index for terminal offline replay retention scans (bounded DELETE by received_at).

CREATE INDEX IF NOT EXISTS ix_machine_offline_events_retention_terminal_received_at ON machine_offline_events (received_at ASC)
WHERE
    processing_status IN (
        'processed',
        'succeeded',
        'failed',
        'duplicate',
        'replayed',
        'rejected'
    );

COMMENT ON INDEX ix_machine_offline_events_retention_terminal_received_at IS 'Retention pruning for aged terminal machine_offline_events (excludes pending/processing).';

-- +goose Down
DROP INDEX IF EXISTS ix_machine_offline_events_retention_terminal_received_at;
