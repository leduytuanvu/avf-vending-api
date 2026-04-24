-- +goose Up
-- +goose StatementBegin

-- Cash collection lifecycle: open session, close with counted vs expected (commerce-derived).

ALTER TABLE cash_collections
    ADD COLUMN IF NOT EXISTS opened_at timestamptz;

UPDATE cash_collections
SET
    opened_at = collected_at
WHERE
    opened_at IS NULL;

ALTER TABLE cash_collections
    ALTER COLUMN opened_at SET NOT NULL;

ALTER TABLE cash_collections
    ADD COLUMN IF NOT EXISTS closed_at timestamptz;

ALTER TABLE cash_collections
    ADD COLUMN IF NOT EXISTS lifecycle_status text NOT NULL DEFAULT 'closed';

ALTER TABLE cash_collections
    DROP CONSTRAINT IF EXISTS cash_collections_lifecycle_status_check;

ALTER TABLE cash_collections
    ADD CONSTRAINT cash_collections_lifecycle_status_check CHECK (
        lifecycle_status IN ('open', 'closed', 'cancelled')
    );

UPDATE cash_collections
SET
    closed_at = collected_at
WHERE
    lifecycle_status = 'closed'
    AND closed_at IS NULL;

ALTER TABLE cash_collections
    ADD COLUMN IF NOT EXISTS expected_amount_minor bigint NOT NULL DEFAULT 0;

ALTER TABLE cash_collections
    ADD COLUMN IF NOT EXISTS variance_amount_minor bigint NOT NULL DEFAULT 0;

ALTER TABLE cash_collections
    ADD COLUMN IF NOT EXISTS requires_review boolean NOT NULL DEFAULT false;

ALTER TABLE cash_collections
    ADD COLUMN IF NOT EXISTS close_request_hash bytea;

UPDATE cash_collections
SET
    expected_amount_minor = amount_minor,
    variance_amount_minor = 0,
    requires_review = false
WHERE
    lifecycle_status = 'closed'
    AND expected_amount_minor = 0
    AND amount_minor > 0;

ALTER TABLE cash_collections
    ALTER COLUMN amount_minor SET DEFAULT 0;

DROP INDEX IF EXISTS ux_cash_collections_machine_one_open;

CREATE UNIQUE INDEX ux_cash_collections_machine_one_open ON cash_collections (machine_id)
    WHERE
        lifecycle_status = 'open';

COMMENT ON TABLE cash_collections IS 'Field cash collection sessions: open then close with counted vs expected (commerce cash, no hardware payout).';

COMMENT ON COLUMN cash_collections.opened_at IS 'When the operator started the collection session (usually equals collected_at).';

COMMENT ON COLUMN cash_collections.closed_at IS 'When the session was closed with a physical count; null while open.';

COMMENT ON COLUMN cash_collections.amount_minor IS 'Physical count (counted cash) when closed; 0 while open.';

COMMENT ON COLUMN cash_collections.expected_amount_minor IS 'Backend-expected net cash in vault at close from commerce since previous closed collection.';

COMMENT ON COLUMN cash_collections.variance_amount_minor IS 'counted minus expected at close.';

COMMENT ON COLUMN cash_collections.requires_review IS 'True when abs(variance) exceeds configured review threshold.';

COMMENT ON COLUMN cash_collections.close_request_hash IS 'SHA-256 of canonical close payload for idempotent close and conflict detection.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS ux_cash_collections_machine_one_open;

ALTER TABLE cash_collections
    ALTER COLUMN amount_minor DROP DEFAULT;

ALTER TABLE cash_collections DROP COLUMN IF EXISTS close_request_hash;

ALTER TABLE cash_collections DROP COLUMN IF EXISTS requires_review;

ALTER TABLE cash_collections DROP COLUMN IF EXISTS variance_amount_minor;

ALTER TABLE cash_collections DROP COLUMN IF EXISTS expected_amount_minor;

ALTER TABLE cash_collections DROP CONSTRAINT IF EXISTS cash_collections_lifecycle_status_check;

ALTER TABLE cash_collections DROP COLUMN IF EXISTS lifecycle_status;

ALTER TABLE cash_collections DROP COLUMN IF EXISTS closed_at;

ALTER TABLE cash_collections DROP COLUMN IF EXISTS opened_at;

COMMENT ON TABLE cash_collections IS 'Physical cash removed from machine; reconcile against expected vault from cash_events.';

-- +goose StatementEnd
