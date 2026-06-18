-- Additive device-config reported state on machine_current_snapshot (effective config + per-field ACK map).
-- +goose Up
-- +goose StatementBegin
ALTER TABLE machine_current_snapshot
ADD COLUMN IF NOT EXISTS effective_device_config jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE machine_current_snapshot
ADD COLUMN IF NOT EXISTS device_config_field_ack jsonb NOT NULL DEFAULT '{}'::jsonb;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE machine_current_snapshot DROP COLUMN IF EXISTS device_config_field_ack;
ALTER TABLE machine_current_snapshot DROP COLUMN IF EXISTS effective_device_config;
-- +goose StatementEnd
