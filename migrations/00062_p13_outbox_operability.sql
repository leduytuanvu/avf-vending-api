-- P1.3: operator-visible metadata + optional per-row publish ceilings for transactional outbox.

-- +goose Up
ALTER TABLE outbox_events
    ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS max_publish_attempts integer NOT NULL DEFAULT 24;

CREATE INDEX IF NOT EXISTS ix_outbox_dead_letter_recent ON outbox_events (dead_lettered_at DESC, id DESC)
WHERE
    dead_lettered_at IS NOT NULL;

COMMENT ON COLUMN outbox_events.updated_at IS 'Last durable mutation on this row (publish lifecycle or admin replay/DLQ).';

COMMENT ON COLUMN outbox_events.max_publish_attempts IS 'Ceiling before Postgres DLQ; worker uses this value when > 0, otherwise recovery policy default.';

-- +goose Down
COMMENT ON COLUMN outbox_events.max_publish_attempts IS NULL;

COMMENT ON COLUMN outbox_events.updated_at IS NULL;

DROP INDEX IF EXISTS ix_outbox_dead_letter_recent;

ALTER TABLE outbox_events DROP COLUMN IF EXISTS max_publish_attempts;

ALTER TABLE outbox_events DROP COLUMN IF EXISTS updated_at;
