-- +goose Up
-- +goose StatementBegin
-- P1.2: explicit outbox lifecycle + worker lease columns for multi-replica safe dispatch.
ALTER TABLE outbox_events
    ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'pending',
    ADD COLUMN IF NOT EXISTS locked_by text,
    ADD COLUMN IF NOT EXISTS locked_until timestamptz;

ALTER TABLE outbox_events
    DROP CONSTRAINT IF EXISTS chk_outbox_events_status;

ALTER TABLE outbox_events
    ADD CONSTRAINT chk_outbox_events_status CHECK (
        status IN (
            'pending',
            'publishing',
            'published',
            'failed',
            'dead_letter'
        )
    );

COMMENT ON COLUMN outbox_events.status IS 'pending: eligible; publishing: lease held; published: terminal success; failed: retry scheduled; dead_letter: quarantined.';
COMMENT ON COLUMN outbox_events.locked_by IS 'Worker instance id holding a short publish lease.';
COMMENT ON COLUMN outbox_events.locked_until IS 'Lease expiry; stale locks are reclaimed by the next dispatcher cycle.';

UPDATE outbox_events
SET
    status = CASE
        WHEN published_at IS NOT NULL THEN 'published'
        WHEN dead_lettered_at IS NOT NULL THEN 'dead_letter'
        WHEN publish_attempt_count > 0
        AND published_at IS NULL
        AND dead_lettered_at IS NULL THEN 'failed'
        ELSE 'pending'
    END
WHERE
    TRUE;

CREATE INDEX IF NOT EXISTS ix_outbox_lease_candidates ON outbox_events (created_at, id)
WHERE
    published_at IS NULL
    AND dead_lettered_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS ix_outbox_lease_candidates;

ALTER TABLE outbox_events DROP CONSTRAINT IF EXISTS chk_outbox_events_status;

ALTER TABLE outbox_events DROP COLUMN IF EXISTS locked_until;

ALTER TABLE outbox_events DROP COLUMN IF EXISTS locked_by;

ALTER TABLE outbox_events DROP COLUMN IF EXISTS status;

-- +goose StatementEnd
