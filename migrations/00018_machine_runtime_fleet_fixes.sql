-- Machine runtime fleet fixes: sale_enabled backfill + one-current-session guard.
-- +goose Up
-- +goose StatementBegin

UPDATE machines
SET sale_enabled = true, updated_at = now()
WHERE status = 'active' AND sale_enabled = false;

CREATE UNIQUE INDEX ux_machine_runtime_app_sessions_one_current
    ON machine_runtime_app_sessions (machine_id)
    WHERE ended_at IS NULL
        AND status IN ('STARTING', 'ONLINE', 'STALE', 'OFFLINE', 'BLOCKED', 'MAINTENANCE');

COMMENT ON INDEX ux_machine_runtime_app_sessions_one_current IS
    'At most one open app runtime session per machine.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS ux_machine_runtime_app_sessions_one_current;

-- +goose StatementEnd
