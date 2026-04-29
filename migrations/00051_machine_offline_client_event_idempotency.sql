-- +goose Up

ALTER TABLE machine_offline_events DROP CONSTRAINT IF EXISTS machine_offline_events_processing_status_check;

ALTER TABLE machine_offline_events ADD CONSTRAINT machine_offline_events_processing_status_check CHECK (
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

CREATE UNIQUE INDEX IF NOT EXISTS ux_machine_offline_client_event_id ON machine_offline_events (
    organization_id,
    machine_id,
    client_event_id
)
WHERE
    btrim(client_event_id) <> '';

-- +goose Down

DROP INDEX IF EXISTS ux_machine_offline_client_event_id;

ALTER TABLE machine_offline_events DROP CONSTRAINT IF EXISTS machine_offline_events_processing_status_check;

ALTER TABLE machine_offline_events ADD CONSTRAINT machine_offline_events_processing_status_check CHECK (
    processing_status IN ('pending', 'processing', 'succeeded', 'failed', 'replayed', 'rejected')
);
