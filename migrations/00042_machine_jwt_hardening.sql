-- +goose Up
ALTER TABLE machines
    ADD COLUMN IF NOT EXISTS credential_revoked_at timestamptz,
    ADD COLUMN IF NOT EXISTS credential_rotated_at timestamptz,
    ADD COLUMN IF NOT EXISTS credential_last_used_at timestamptz;

UPDATE machines
SET credential_rotated_at = COALESCE(credential_rotated_at, updated_at, created_at)
WHERE credential_rotated_at IS NULL;

ALTER TABLE machine_runtime_refresh_tokens
    ADD COLUMN IF NOT EXISTS last_used_at timestamptz,
    ADD COLUMN IF NOT EXISTS rotated_at timestamptz;

-- +goose Down
ALTER TABLE machine_runtime_refresh_tokens
    DROP COLUMN IF EXISTS rotated_at,
    DROP COLUMN IF EXISTS last_used_at;

ALTER TABLE machines
    DROP COLUMN IF EXISTS credential_last_used_at,
    DROP COLUMN IF EXISTS credential_rotated_at,
    DROP COLUMN IF EXISTS credential_revoked_at;
