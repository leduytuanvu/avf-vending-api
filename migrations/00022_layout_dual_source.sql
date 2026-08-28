-- Dual SERVER/LOCAL layout assignment, authoritative grid dimensions, and migration audit.
-- +goose Up
-- +goose StatementBegin

-- Authoritative scalar dimensions on cabinet slot layouts (nullable until backfill).
ALTER TABLE machine_slot_layouts
    ADD COLUMN IF NOT EXISTS grid_rows int,
    ADD COLUMN IF NOT EXISTS grid_cols int;

ALTER TABLE machine_slot_layouts
    DROP CONSTRAINT IF EXISTS ck_machine_slot_layouts_grid_rows_range;

ALTER TABLE machine_slot_layouts
    ADD CONSTRAINT ck_machine_slot_layouts_grid_rows_range CHECK (
        grid_rows IS NULL OR (grid_rows >= 1 AND grid_rows <= 26)
    ) NOT VALID;

ALTER TABLE machine_slot_layouts
    DROP CONSTRAINT IF EXISTS ck_machine_slot_layouts_grid_cols_range;

ALTER TABLE machine_slot_layouts
    ADD CONSTRAINT ck_machine_slot_layouts_grid_cols_range CHECK (
        grid_cols IS NULL OR (grid_cols >= 1 AND grid_cols <= 12)
    ) NOT VALID;

-- Enterprise planogram version source + dimensions.
ALTER TABLE machine_planogram_versions
    ADD COLUMN IF NOT EXISTS layout_source text NOT NULL DEFAULT 'SERVER',
    ADD COLUMN IF NOT EXISTS grid_rows int,
    ADD COLUMN IF NOT EXISTS grid_cols int,
    ADD COLUMN IF NOT EXISTS fingerprint text,
    ADD COLUMN IF NOT EXISTS org_layout_version_id uuid;

ALTER TABLE machine_planogram_versions
    DROP CONSTRAINT IF EXISTS ck_machine_planogram_versions_layout_source;

ALTER TABLE machine_planogram_versions
    ADD CONSTRAINT ck_machine_planogram_versions_layout_source CHECK (
        layout_source IN ('SERVER', 'LOCAL')
    );

ALTER TABLE machine_planogram_versions
    DROP CONSTRAINT IF EXISTS ck_machine_planogram_versions_grid_rows_range;

ALTER TABLE machine_planogram_versions
    ADD CONSTRAINT ck_machine_planogram_versions_grid_rows_range CHECK (
        grid_rows IS NULL OR (grid_rows >= 1 AND grid_rows <= 26)
    ) NOT VALID;

ALTER TABLE machine_planogram_versions
    DROP CONSTRAINT IF EXISTS ck_machine_planogram_versions_grid_cols_range;

ALTER TABLE machine_planogram_versions
    ADD CONSTRAINT ck_machine_planogram_versions_grid_cols_range CHECK (
        grid_cols IS NULL OR (grid_cols >= 1 AND grid_cols <= 12)
    ) NOT VALID;

-- Immutable org-level published template versions (one version serves many machines).
CREATE TABLE IF NOT EXISTS planogram_template_versions (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v7(),
    template_id uuid NOT NULL REFERENCES planogram_templates (id) ON DELETE CASCADE,
    version_no int NOT NULL,
    grid_rows int NOT NULL CHECK (grid_rows >= 1 AND grid_rows <= 26),
    grid_cols int NOT NULL CHECK (grid_cols >= 1 AND grid_cols <= 12),
    snapshot jsonb NOT NULL,
    fingerprint text NOT NULL,
    published_at timestamptz NOT NULL DEFAULT now(),
    published_by uuid REFERENCES platform_auth_accounts (id) ON DELETE SET NULL,
    CONSTRAINT ux_planogram_template_versions_template_version UNIQUE (template_id, version_no)
);

CREATE INDEX IF NOT EXISTS ix_planogram_template_versions_template ON planogram_template_versions (template_id, published_at DESC);

-- Current assignment per (machine, source); history via is_current=false.
CREATE TABLE IF NOT EXISTS machine_layout_assignments (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v7(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    source text NOT NULL CHECK (source IN ('SERVER', 'LOCAL')),
    layout_id uuid,
    layout_version_id uuid REFERENCES machine_planogram_versions (id) ON DELETE SET NULL,
    org_layout_version_id uuid REFERENCES planogram_template_versions (id) ON DELETE SET NULL,
    revision int NOT NULL CHECK (revision >= 1),
    grid_rows int NOT NULL CHECK (grid_rows >= 1 AND grid_rows <= 26),
    grid_cols int NOT NULL CHECK (grid_cols >= 1 AND grid_cols <= 12),
    fingerprint text NOT NULL,
    is_current boolean NOT NULL DEFAULT true,
    effective_from timestamptz NOT NULL DEFAULT now(),
    effective_to timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    created_by uuid REFERENCES platform_auth_accounts (id) ON DELETE SET NULL,
    CONSTRAINT ck_machine_layout_assignments_server_version CHECK (
        source <> 'SERVER' OR layout_version_id IS NOT NULL
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_mla_current_machine_source ON machine_layout_assignments (machine_id, source)
WHERE
    is_current;

CREATE INDEX IF NOT EXISTS ix_machine_layout_assignments_machine ON machine_layout_assignments (machine_id, created_at DESC);

-- Desired vs reported layout state (one row per machine).
CREATE TABLE IF NOT EXISTS machine_layout_state (
    machine_id uuid PRIMARY KEY REFERENCES machines (id) ON DELETE CASCADE,
    desired_source text CHECK (desired_source IS NULL OR desired_source IN ('SERVER', 'LOCAL')),
    desired_assignment_id uuid REFERENCES machine_layout_assignments (id) ON DELETE SET NULL,
    desired_layout_version_id uuid REFERENCES machine_planogram_versions (id) ON DELETE SET NULL,
    desired_revision int,
    desired_fingerprint text,
    desired_updated_at timestamptz,
    reported_source text CHECK (reported_source IS NULL OR reported_source IN ('SERVER', 'LOCAL')),
    reported_assignment_id uuid REFERENCES machine_layout_assignments (id) ON DELETE SET NULL,
    reported_layout_version_id uuid REFERENCES machine_planogram_versions (id) ON DELETE SET NULL,
    reported_revision int,
    reported_fingerprint text,
    reported_at timestamptz,
    reported_device_instance_id text,
    apply_failure_reason text,
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Device-authored LOCAL layout mirror (read-only for admin).
CREATE TABLE IF NOT EXISTS machine_local_layout_mirror (
    machine_id uuid PRIMARY KEY REFERENCES machines (id) ON DELETE CASCADE,
    local_layout_id uuid NOT NULL,
    revision int NOT NULL CHECK (revision >= 1),
    grid_rows int NOT NULL CHECK (grid_rows >= 1 AND grid_rows <= 26),
    grid_cols int NOT NULL CHECK (grid_cols >= 1 AND grid_cols <= 12),
    slots jsonb NOT NULL DEFAULT '[]'::jsonb,
    fingerprint text NOT NULL,
    reported_at timestamptz NOT NULL DEFAULT now(),
    device_instance_id text NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Legacy dimension migration audit (PROVEN / INFERRED_SAFE / REQUIRES_REVIEW).
CREATE TABLE IF NOT EXISTS layout_dimension_migration_audit (
    machine_slot_layout_id uuid PRIMARY KEY REFERENCES machine_slot_layouts (id) ON DELETE CASCADE,
    class text NOT NULL CHECK (class IN ('PROVEN', 'INFERRED_SAFE', 'REQUIRES_REVIEW')),
    evidence jsonb NOT NULL DEFAULT '{}'::jsonb,
    audited_at timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE machine_layout_assignments IS 'Current/historical layout assignment per machine and source (SERVER|LOCAL).';
COMMENT ON TABLE machine_layout_state IS 'Desired vs reported active layout; sync_status derived at read time.';
COMMENT ON TABLE machine_local_layout_mirror IS 'Last device-reported LOCAL layout snapshot; admin read-only.';
COMMENT ON TABLE layout_dimension_migration_audit IS 'Classification of legacy layout_spec dimensions before NOT NULL enforcement.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS layout_dimension_migration_audit;
DROP TABLE IF EXISTS machine_local_layout_mirror;
DROP TABLE IF EXISTS machine_layout_state;
DROP TABLE IF EXISTS machine_layout_assignments;
DROP TABLE IF EXISTS planogram_template_versions;

ALTER TABLE machine_planogram_versions
    DROP CONSTRAINT IF EXISTS ck_machine_planogram_versions_grid_cols_range,
    DROP CONSTRAINT IF EXISTS ck_machine_planogram_versions_grid_rows_range,
    DROP CONSTRAINT IF EXISTS ck_machine_planogram_versions_layout_source;

ALTER TABLE machine_planogram_versions
    DROP COLUMN IF EXISTS org_layout_version_id,
    DROP COLUMN IF EXISTS fingerprint,
    DROP COLUMN IF EXISTS grid_cols,
    DROP COLUMN IF EXISTS grid_rows,
    DROP COLUMN IF EXISTS layout_source;

ALTER TABLE machine_slot_layouts
    DROP CONSTRAINT IF EXISTS ck_machine_slot_layouts_grid_cols_range,
    DROP CONSTRAINT IF EXISTS ck_machine_slot_layouts_grid_rows_range;

ALTER TABLE machine_slot_layouts
    DROP COLUMN IF EXISTS grid_cols,
    DROP COLUMN IF EXISTS grid_rows;

-- +goose StatementEnd
