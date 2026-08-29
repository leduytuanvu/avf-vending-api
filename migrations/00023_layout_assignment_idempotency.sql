-- Payload-bound idempotency for SERVER layout assignment commands.
-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS layout_assignment_idempotency (
    scope_id text NOT NULL,
    idempotency_key text NOT NULL,
    request_hash text NOT NULL,
    response_json jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ux_layout_assignment_idempotency_scope_key UNIQUE (scope_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS ix_layout_assignment_idempotency_created_at
    ON layout_assignment_idempotency (created_at DESC);

COMMENT ON TABLE layout_assignment_idempotency IS 'Replay store for PUT layout-assignments/server; scope_id is machine UUID text.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS layout_assignment_idempotency;

-- +goose StatementEnd
