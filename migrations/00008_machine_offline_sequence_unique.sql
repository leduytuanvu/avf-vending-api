-- +goose Up
-- Required by InsertMachineOfflineEvent ON CONFLICT (machine_id, offline_sequence).
CREATE UNIQUE INDEX IF NOT EXISTS ux_machine_offline_events_machine_sequence
    ON machine_offline_events (machine_id, offline_sequence);

-- +goose Down
DROP INDEX IF EXISTS ux_machine_offline_events_machine_sequence;
