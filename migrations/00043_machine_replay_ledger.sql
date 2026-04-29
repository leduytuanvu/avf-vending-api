-- +goose Up
CREATE TABLE IF NOT EXISTS machine_idempotency_keys (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    operation text NOT NULL,
    idempotency_key text NOT NULL,
    request_hash bytea NOT NULL,
    response_snapshot jsonb,
    status text NOT NULL DEFAULT 'in_progress' CHECK (status IN ('in_progress', 'succeeded', 'failed', 'conflict', 'expired')),
    first_seen_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    trace_id text NOT NULL DEFAULT '',
    CONSTRAINT ux_machine_idempotency_key UNIQUE (organization_id, machine_id, operation, idempotency_key)
);

CREATE INDEX IF NOT EXISTS ix_machine_idempotency_expiry ON machine_idempotency_keys (expires_at);

CREATE TABLE IF NOT EXISTS machine_offline_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    offline_sequence bigint NOT NULL,
    event_type text NOT NULL,
    event_id text NOT NULL DEFAULT '',
    client_event_id text NOT NULL DEFAULT '',
    occurred_at timestamptz NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now(),
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    processing_status text NOT NULL DEFAULT 'pending' CHECK (processing_status IN ('pending', 'processing', 'succeeded', 'failed', 'replayed', 'rejected')),
    processing_error text NOT NULL DEFAULT '',
    idempotency_key text NOT NULL DEFAULT '',
    CONSTRAINT ux_machine_offline_sequence UNIQUE (organization_id, machine_id, offline_sequence)
);

CREATE INDEX IF NOT EXISTS ix_machine_offline_pending ON machine_offline_events (organization_id, machine_id, offline_sequence)
WHERE processing_status IN ('pending', 'processing');

CREATE INDEX IF NOT EXISTS ix_machine_offline_event_id ON machine_offline_events (organization_id, machine_id, event_id)
WHERE event_id <> '';

CREATE TABLE IF NOT EXISTS machine_sync_cursors (
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    stream_name text NOT NULL,
    last_sequence bigint NOT NULL DEFAULT 0,
    last_synced_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, machine_id, stream_name)
);

-- +goose Down
DROP TABLE IF EXISTS machine_sync_cursors;
DROP TABLE IF EXISTS machine_offline_events;
DROP TABLE IF EXISTS machine_idempotency_keys;
