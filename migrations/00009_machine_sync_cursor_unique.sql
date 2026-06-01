-- +goose Up
-- Required by UpsertMachineSyncCursor ON CONFLICT (machine_id, stream_name).
CREATE UNIQUE INDEX IF NOT EXISTS ux_machine_sync_cursors_machine_stream
    ON machine_sync_cursors (machine_id, stream_name);

-- +goose Down
DROP INDEX IF EXISTS ux_machine_sync_cursors_machine_stream;
