-- Allow PERIODIC_5M snapshot reason for five-minute runtime captures.
-- +goose Up
-- +goose StatementBegin

ALTER TABLE machine_layout_snapshot_history
    DROP CONSTRAINT IF EXISTS machine_layout_snapshot_history_snapshot_reason_check;

ALTER TABLE machine_layout_snapshot_history
    ADD CONSTRAINT machine_layout_snapshot_history_snapshot_reason_check CHECK (
        snapshot_reason IN (
            'PERIODIC_30M',
            'PERIODIC_5M',
            'MANUAL_SYNC',
            'RECONNECT',
            'ACTIVATION',
            'LEGACY_REPORT'
        )
    );

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE machine_layout_snapshot_history
    DROP CONSTRAINT IF EXISTS machine_layout_snapshot_history_snapshot_reason_check;

ALTER TABLE machine_layout_snapshot_history
    ADD CONSTRAINT machine_layout_snapshot_history_snapshot_reason_check CHECK (
        snapshot_reason IN (
            'PERIODIC_30M',
            'MANUAL_SYNC',
            'RECONNECT',
            'ACTIVATION',
            'LEGACY_REPORT'
        )
    );

-- +goose StatementEnd
