-- +goose Up
-- +goose StatementBegin

CREATE TABLE device_telemetry_events (
    id bigserial PRIMARY KEY,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    event_type text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    dedupe_key text,
    received_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ux_device_telemetry_dedupe ON device_telemetry_events (dedupe_key)
    WHERE dedupe_key IS NOT NULL AND btrim(dedupe_key) <> '';

CREATE INDEX ix_device_telemetry_machine_received ON device_telemetry_events (machine_id, received_at DESC);

CREATE TABLE device_command_receipts (
    id bigserial PRIMARY KEY,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    sequence bigint NOT NULL CHECK (sequence >= 0),
    status text NOT NULL,
    correlation_id uuid,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    dedupe_key text NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ux_device_command_receipts_dedupe UNIQUE (dedupe_key)
);

CREATE INDEX ix_device_command_receipts_machine_seq ON device_command_receipts (machine_id, sequence DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS device_command_receipts;
DROP TABLE IF EXISTS device_telemetry_events;
-- +goose StatementEnd
