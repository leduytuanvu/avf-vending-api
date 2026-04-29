-- +goose Up
-- +goose StatementBegin

ALTER TABLE ota_machine_results DROP CONSTRAINT IF EXISTS ota_machine_results_status_check;

ALTER TABLE ota_machine_results
    ADD CONSTRAINT ota_machine_results_status_check CHECK (
        status IN ('pending', 'dispatched', 'acked', 'downloaded', 'installed', 'success', 'failed')
    );

ALTER TABLE diagnostic_bundle_manifests
    ADD COLUMN IF NOT EXISTS organization_id uuid REFERENCES organizations (id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS request_id uuid,
    ADD COLUMN IF NOT EXISTS command_id uuid REFERENCES command_ledger (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'available' CHECK (status IN ('available', 'failed'));

UPDATE diagnostic_bundle_manifests d
SET organization_id = m.organization_id
FROM machines m
WHERE d.machine_id = m.id
  AND d.organization_id IS NULL;

CREATE INDEX IF NOT EXISTS ix_diagnostic_bundle_manifests_org_machine_created
    ON diagnostic_bundle_manifests (organization_id, machine_id, created_at DESC);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'ux_diagnostic_bundle_manifests_machine_request'
    ) THEN
        ALTER TABLE diagnostic_bundle_manifests
            ADD CONSTRAINT ux_diagnostic_bundle_manifests_machine_request UNIQUE (machine_id, request_id);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS ix_ota_machine_results_machine_status
    ON ota_machine_results (machine_id, status, updated_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS ix_ota_machine_results_machine_status;
ALTER TABLE diagnostic_bundle_manifests DROP CONSTRAINT IF EXISTS ux_diagnostic_bundle_manifests_machine_request;
DROP INDEX IF EXISTS ix_diagnostic_bundle_manifests_org_machine_created;

ALTER TABLE diagnostic_bundle_manifests
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS command_id,
    DROP COLUMN IF EXISTS request_id,
    DROP COLUMN IF EXISTS organization_id;

ALTER TABLE ota_machine_results DROP CONSTRAINT IF EXISTS ota_machine_results_status_check;

ALTER TABLE ota_machine_results
    ADD CONSTRAINT ota_machine_results_status_check CHECK (
        status IN ('pending', 'dispatched', 'acked', 'success', 'failed')
    );

-- +goose StatementEnd
