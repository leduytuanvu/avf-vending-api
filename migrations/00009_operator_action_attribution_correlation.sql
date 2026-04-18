-- +goose Up
-- +goose StatementBegin
-- correlation_id is nullable; partial indexes keep hot paths small and avoid wide btree scans on NULLs.
ALTER TABLE machine_action_attributions
ADD COLUMN IF NOT EXISTS correlation_id uuid;

CREATE INDEX IF NOT EXISTS ix_machine_action_attributions_correlation ON machine_action_attributions (correlation_id, occurred_at DESC)
WHERE
    correlation_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS ix_machine_action_attributions_machine_correlation ON machine_action_attributions (machine_id, correlation_id, occurred_at DESC)
WHERE
    correlation_id IS NOT NULL;

COMMENT ON COLUMN machine_action_attributions.correlation_id IS 'Optional request/correlation id aligned with device and auth event tracing.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS ix_machine_action_attributions_machine_correlation;

DROP INDEX IF EXISTS ix_machine_action_attributions_correlation;

ALTER TABLE machine_action_attributions
DROP COLUMN IF EXISTS correlation_id;

-- +goose StatementEnd
