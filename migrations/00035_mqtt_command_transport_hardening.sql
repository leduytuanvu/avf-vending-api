-- P0.8 MQTT command transport: expired attempt status, optional per-machine MQTT credential refs (no secrets in-row).

-- +goose Up
ALTER TABLE machine_command_attempts
    DROP CONSTRAINT IF EXISTS machine_command_attempts_status_check;

ALTER TABLE machine_command_attempts
    ADD CONSTRAINT machine_command_attempts_status_check CHECK (
        status IN (
            'pending',
            'sent',
            'ack_timeout',
            'expired',
            'nack',
            'completed',
            'failed',
            'duplicate',
            'late'
        )
    );

CREATE TABLE IF NOT EXISTS machine_mqtt_credentials (
    machine_id uuid PRIMARY KEY REFERENCES machines (id) ON DELETE CASCADE,
    broker_scope text NOT NULL DEFAULT 'default',
    username text,
    secret_ref text,
    updated_at timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE machine_mqtt_credentials IS 'Optional per-machine MQTT username; secret_ref is an opaque pointer to a secret manager (never store broker passwords in this table). Wire-up to publishers is environment-specific — see docs.';

CREATE INDEX IF NOT EXISTS ix_machine_mqtt_credentials_scope ON machine_mqtt_credentials (broker_scope);

-- +goose Down
DROP INDEX IF EXISTS ix_machine_mqtt_credentials_scope;

DROP TABLE IF EXISTS machine_mqtt_credentials;

UPDATE machine_command_attempts
SET status = 'ack_timeout'
WHERE status = 'expired';

ALTER TABLE machine_command_attempts
    DROP CONSTRAINT IF EXISTS machine_command_attempts_status_check;

ALTER TABLE machine_command_attempts
    ADD CONSTRAINT machine_command_attempts_status_check CHECK (
        status IN (
            'pending',
            'sent',
            'ack_timeout',
            'nack',
            'completed',
            'failed',
            'duplicate',
            'late'
        )
    );
