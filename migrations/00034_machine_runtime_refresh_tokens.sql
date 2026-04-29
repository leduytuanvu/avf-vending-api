-- +goose Up
CREATE TABLE machine_runtime_refresh_tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    token_hash bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz
);

CREATE UNIQUE INDEX ux_machine_runtime_refresh_token_hash ON machine_runtime_refresh_tokens (token_hash);

CREATE UNIQUE INDEX ux_machine_runtime_refresh_one_active ON machine_runtime_refresh_tokens (machine_id)
WHERE
    revoked_at IS NULL;

CREATE INDEX ix_machine_runtime_refresh_machine ON machine_runtime_refresh_tokens (machine_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS ix_machine_runtime_refresh_machine;

DROP INDEX IF EXISTS ux_machine_runtime_refresh_one_active;

DROP INDEX IF EXISTS ux_machine_runtime_refresh_token_hash;

DROP TABLE IF EXISTS machine_runtime_refresh_tokens;
