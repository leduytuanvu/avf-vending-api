-- +goose Up
-- +goose StatementBegin
-- Outbox publish pipeline: bounded retries, backoff scheduling, dead-letter marker, and operator metrics.
ALTER TABLE outbox_events
    ADD COLUMN IF NOT EXISTS publish_attempt_count integer NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_publish_error text,
    ADD COLUMN IF NOT EXISTS last_publish_attempt_at timestamptz,
    ADD COLUMN IF NOT EXISTS next_publish_after timestamptz,
    ADD COLUMN IF NOT EXISTS dead_lettered_at timestamptz;

COMMENT ON COLUMN outbox_events.publish_attempt_count IS 'Increments on each failed publish (broker error or transport rejection before published_at is set).';
COMMENT ON COLUMN outbox_events.last_publish_error IS 'Truncated last publish-side error for operators.';
COMMENT ON COLUMN outbox_events.last_publish_attempt_at IS 'Wall time of the last publish attempt.';
COMMENT ON COLUMN outbox_events.next_publish_after IS 'Eligible for ListOutboxUnpublished when NULL or <= now(); set after failures for deterministic backoff.';
COMMENT ON COLUMN outbox_events.dead_lettered_at IS 'Set when publish_attempt_count exceeds policy; row is excluded from dispatch until manual intervention.';

CREATE INDEX IF NOT EXISTS ix_outbox_pending_due ON outbox_events (created_at, id)
WHERE
    published_at IS NULL
    AND dead_lettered_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS ix_outbox_pending_due;

ALTER TABLE outbox_events DROP COLUMN IF EXISTS dead_lettered_at;

ALTER TABLE outbox_events DROP COLUMN IF EXISTS next_publish_after;

ALTER TABLE outbox_events DROP COLUMN IF EXISTS last_publish_attempt_at;

ALTER TABLE outbox_events DROP COLUMN IF EXISTS last_publish_error;

ALTER TABLE outbox_events DROP COLUMN IF EXISTS publish_attempt_count;

-- +goose StatementEnd
