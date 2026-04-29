-- +goose Up
-- Business-time audit anchor (defaults to insert time for existing rows).

ALTER TABLE audit_events
ADD COLUMN IF NOT EXISTS occurred_at timestamptz;

UPDATE audit_events
SET
    occurred_at = created_at
WHERE
    occurred_at IS NULL;

ALTER TABLE audit_events
ALTER COLUMN occurred_at SET DEFAULT now(),
ALTER COLUMN occurred_at SET NOT NULL;

-- +goose Down

ALTER TABLE audit_events DROP COLUMN IF EXISTS occurred_at;
