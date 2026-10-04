-- Allow dispatch failure statuses written by PushOfflineEvents (machine_contract_grpc.go).
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
            'failed_retryable',
            'failed_terminal',
            'duplicate',
            'replayed',
            'rejected'
        )
    );

DROP INDEX IF EXISTS ix_machine_offline_events_retention_terminal_received_at;

CREATE INDEX ix_machine_offline_events_retention_terminal_received_at ON machine_offline_events (received_at ASC)
WHERE
    processing_status IN (
        'processed',
        'succeeded',
        'failed',
        'failed_retryable',
        'failed_terminal',
        'duplicate',
        'replayed',
        'rejected'
    );
