-- +goose Up
ALTER TABLE sites
    ADD COLUMN IF NOT EXISTS contact_info jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE machines
    ADD COLUMN IF NOT EXISTS cabinet_type text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS activated_at timestamptz,
    ADD COLUMN IF NOT EXISTS revoked_at timestamptz,
    ADD COLUMN IF NOT EXISTS rotated_at timestamptz;

-- +goose StatementBegin
DO $$
DECLARE
    constraint_name text;
BEGIN
    SELECT c.conname
    INTO constraint_name
    FROM pg_constraint c
    JOIN pg_class t ON t.oid = c.conrelid
    JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY (c.conkey)
    WHERE t.relname = 'machines'
      AND a.attname = 'status'
      AND c.contype = 'c'
    LIMIT 1;

    IF constraint_name IS NOT NULL THEN
        EXECUTE format('ALTER TABLE machines DROP CONSTRAINT %I', constraint_name);
    END IF;
END $$;
-- +goose StatementEnd

UPDATE machines
SET status = CASE status
    WHEN 'provisioning' THEN 'draft'
    WHEN 'online' THEN 'active'
    WHEN 'offline' THEN 'active'
    ELSE status
END
WHERE status IN ('provisioning', 'online', 'offline');

ALTER TABLE machines
    ADD CONSTRAINT machines_status_check
    CHECK (status IN ('draft', 'active', 'maintenance', 'suspended', 'retired', 'compromised', 'provisioning', 'online', 'offline'));

UPDATE machines
SET rotated_at = COALESCE(rotated_at, credential_rotated_at)
WHERE rotated_at IS NULL
  AND credential_rotated_at IS NOT NULL;

ALTER TABLE technician_machine_assignments
    ADD COLUMN IF NOT EXISTS scope text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS created_by uuid REFERENCES platform_auth_accounts (id) ON DELETE SET NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ux_tma_one_active_machine_technician
    ON technician_machine_assignments (organization_id, machine_id, technician_id)
    WHERE status = 'active' AND valid_to IS NULL;

-- Compatibility view for the P0.5 contract wording; writes still use technician_machine_assignments.
CREATE OR REPLACE VIEW machine_technician_assignments AS
SELECT
    id,
    organization_id,
    machine_id,
    technician_id AS user_id,
    role,
    NULLIF(scope, '') AS scope,
    valid_from AS active_from,
    valid_to AS active_until,
    created_by,
    created_at
FROM technician_machine_assignments;

-- +goose Down
DROP VIEW IF EXISTS machine_technician_assignments;
DROP INDEX IF EXISTS ux_tma_one_active_machine_technician;

ALTER TABLE technician_machine_assignments
    DROP COLUMN IF EXISTS created_by,
    DROP COLUMN IF EXISTS scope;

ALTER TABLE machines
    DROP CONSTRAINT IF EXISTS machines_status_check;

UPDATE machines
SET status = CASE status
    WHEN 'draft' THEN 'provisioning'
    WHEN 'active' THEN 'online'
    WHEN 'suspended' THEN 'maintenance'
    WHEN 'compromised' THEN 'maintenance'
    ELSE status
END
WHERE status IN ('draft', 'active', 'suspended', 'compromised');

ALTER TABLE machines
    ADD CONSTRAINT machines_status_check
    CHECK (status IN ('provisioning', 'online', 'offline', 'maintenance', 'retired'));

ALTER TABLE machines
    DROP COLUMN IF EXISTS rotated_at,
    DROP COLUMN IF EXISTS revoked_at,
    DROP COLUMN IF EXISTS activated_at,
    DROP COLUMN IF EXISTS cabinet_type;

ALTER TABLE sites
    DROP COLUMN IF EXISTS contact_info;