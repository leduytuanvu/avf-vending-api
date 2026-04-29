-- +goose Up
-- +goose StatementBegin

-- Sites: enterprise fields (code unique per org when non-empty).
ALTER TABLE sites
ADD COLUMN IF NOT EXISTS code text NOT NULL DEFAULT '';

ALTER TABLE sites
ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'active';

ALTER TABLE sites
ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

ALTER TABLE sites DROP CONSTRAINT IF EXISTS sites_status_check;

ALTER TABLE sites
ADD CONSTRAINT sites_status_check CHECK (status IN ('active', 'inactive'));

UPDATE sites
SET
    code = substr(replace(id::text, '-', ''), 1, 12)
WHERE
    code = '';

CREATE UNIQUE INDEX IF NOT EXISTS ux_sites_org_code_lower ON sites (organization_id, lower(code))
WHERE
    btrim(code) <> '';

-- Machines: customer code, model, credential rotation, last-seen; optional human-readable machine code per org.
ALTER TABLE machines
ADD COLUMN IF NOT EXISTS code text NOT NULL DEFAULT '';

ALTER TABLE machines
ADD COLUMN IF NOT EXISTS model text;

ALTER TABLE machines
ADD COLUMN IF NOT EXISTS credential_version bigint NOT NULL DEFAULT 0;

ALTER TABLE machines
ADD COLUMN IF NOT EXISTS last_seen_at timestamptz NULL;

UPDATE machines
SET
    code = substr(replace(id::text, '-', ''), 1, 12)
WHERE
    code = '';

CREATE UNIQUE INDEX IF NOT EXISTS ux_machines_org_code_lower ON machines (organization_id, lower(code))
WHERE
    btrim(code) <> '';

CREATE UNIQUE INDEX IF NOT EXISTS ux_machines_serial_global_lower ON machines (lower(trim(serial_number)))
WHERE
    btrim(serial_number) <> '';

-- Technicians lifecycle.
ALTER TABLE technicians
ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'active';

ALTER TABLE technicians
ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

ALTER TABLE technicians DROP CONSTRAINT IF EXISTS technicians_status_check;

ALTER TABLE technicians
ADD CONSTRAINT technicians_status_check CHECK (status IN ('active', 'inactive'));

-- Technician assignments: tenant column + lifecycle status (existing rows use valid_to window).
ALTER TABLE technician_machine_assignments
ADD COLUMN IF NOT EXISTS organization_id uuid REFERENCES organizations (id) ON DELETE CASCADE;

UPDATE technician_machine_assignments tma
SET
    organization_id = t.organization_id
FROM
    technicians t
WHERE
    t.id = tma.technician_id
    AND tma.organization_id IS NULL;

ALTER TABLE technician_machine_assignments ALTER COLUMN organization_id SET NOT NULL;

ALTER TABLE technician_machine_assignments
ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'active';

ALTER TABLE technician_machine_assignments
ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

ALTER TABLE technician_machine_assignments DROP CONSTRAINT IF EXISTS technician_machine_assignments_status_check;

ALTER TABLE technician_machine_assignments
ADD CONSTRAINT technician_machine_assignments_status_check CHECK (status IN ('active', 'released'));

CREATE INDEX IF NOT EXISTS ix_tma_organization_id ON technician_machine_assignments (organization_id);

-- Replacement audit linkage (one successor per retired asset).
CREATE TABLE machine_lineage (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    prior_machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE RESTRICT,
    successor_machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE RESTRICT,
    reason text,
    created_at timestamptz NOT NULL DEFAULT now (),
    CONSTRAINT ux_machine_lineage_prior UNIQUE (prior_machine_id),
    CONSTRAINT ux_machine_lineage_successor UNIQUE (successor_machine_id),
    CONSTRAINT ck_machine_lineage_distinct CHECK (prior_machine_id <> successor_machine_id)
);

CREATE INDEX ix_machine_lineage_org ON machine_lineage (organization_id);

COMMENT ON TABLE machine_lineage IS 'Audit trail when a machine asset is replaced; prior is retired, successor continues operations.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS machine_lineage;

DROP INDEX IF EXISTS ix_tma_organization_id;

ALTER TABLE technician_machine_assignments DROP COLUMN IF EXISTS updated_at;

ALTER TABLE technician_machine_assignments DROP COLUMN IF EXISTS status;

ALTER TABLE technician_machine_assignments DROP COLUMN IF EXISTS organization_id;

ALTER TABLE technicians DROP COLUMN IF EXISTS updated_at;

ALTER TABLE technicians DROP COLUMN IF EXISTS status;

DROP INDEX IF EXISTS ux_machines_serial_global_lower;

DROP INDEX IF EXISTS ux_machines_org_machine_code_lower;

ALTER TABLE machines DROP COLUMN IF EXISTS last_seen_at;

ALTER TABLE machines DROP COLUMN IF EXISTS credential_version;

ALTER TABLE machines DROP COLUMN IF EXISTS model;

ALTER TABLE machines DROP COLUMN IF EXISTS code;

DROP INDEX IF EXISTS ux_sites_org_code_lower;

ALTER TABLE sites DROP COLUMN IF EXISTS updated_at;

ALTER TABLE sites DROP COLUMN IF EXISTS status;

ALTER TABLE sites DROP COLUMN IF EXISTS code;

-- +goose StatementEnd
