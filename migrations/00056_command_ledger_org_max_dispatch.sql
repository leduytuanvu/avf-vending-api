-- +goose Up
-- P0.9: tenant scope + dispatch attempt ceiling for MQTT command ledger.

ALTER TABLE command_ledger
ADD COLUMN IF NOT EXISTS organization_id uuid REFERENCES organizations (id) ON DELETE CASCADE;

UPDATE command_ledger cl
SET
    organization_id = m.organization_id
FROM machines m
WHERE
    cl.machine_id = m.id
    AND cl.organization_id IS NULL;

ALTER TABLE command_ledger
ALTER COLUMN organization_id SET NOT NULL;

ALTER TABLE command_ledger
ADD COLUMN IF NOT EXISTS max_dispatch_attempts integer NOT NULL DEFAULT 5;

ALTER TABLE command_ledger
ADD CONSTRAINT ck_command_ledger_max_dispatch_attempts CHECK (
    max_dispatch_attempts >= 1
    AND max_dispatch_attempts <= 100
);

COMMENT ON COLUMN command_ledger.organization_id IS 'Denormalized tenant scope for command_ledger rows (matches machines.organization_id).';
COMMENT ON COLUMN command_ledger.max_dispatch_attempts IS 'Maximum machine_command_attempts rows allowed per command_id before dispatch is refused (poison guard).';

-- +goose Down

ALTER TABLE command_ledger DROP CONSTRAINT IF EXISTS ck_command_ledger_max_dispatch_attempts;

ALTER TABLE command_ledger DROP COLUMN IF EXISTS max_dispatch_attempts;

ALTER TABLE command_ledger DROP COLUMN IF EXISTS organization_id;
