-- +goose Up

CREATE TABLE planogram_templates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    snapshot jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_planogram_templates_org ON planogram_templates (organization_id, created_at DESC);

CREATE TABLE machine_planogram_drafts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    status text NOT NULL CHECK (
        status IN ('editing', 'validated')
    ),
    snapshot jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now (),
    updated_at timestamptz NOT NULL DEFAULT now (),
    CONSTRAINT fk_machine_planogram_drafts_org_machine FOREIGN KEY (organization_id, machine_id) REFERENCES machines (organization_id, id) ON DELETE CASCADE
);

CREATE INDEX ix_machine_planogram_drafts_machine ON machine_planogram_drafts (machine_id, updated_at DESC);

CREATE TABLE machine_planogram_versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    version_no int NOT NULL,
    snapshot jsonb NOT NULL,
    source_draft_id uuid REFERENCES machine_planogram_drafts (id) ON DELETE SET NULL,
    published_at timestamptz NOT NULL DEFAULT now (),
    published_by uuid REFERENCES platform_auth_accounts (id) ON DELETE SET NULL,
    CONSTRAINT ux_machine_planogram_versions_machine_version UNIQUE (machine_id, version_no),
    CONSTRAINT fk_machine_planogram_versions_org_machine FOREIGN KEY (organization_id, machine_id) REFERENCES machines (organization_id, id) ON DELETE CASCADE
);

CREATE INDEX ix_machine_planogram_versions_machine_published ON machine_planogram_versions (machine_id, published_at DESC);

CREATE TABLE machine_planogram_slots (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    version_id uuid NOT NULL REFERENCES machine_planogram_versions (id) ON DELETE CASCADE,
    cabinet_code text NOT NULL,
    layout_key text NOT NULL,
    layout_revision int NOT NULL,
    slot_code text NOT NULL,
    legacy_slot_index int NULL,
    product_id uuid NULL,
    max_quantity int NOT NULL,
    price_minor bigint NOT NULL
);

CREATE INDEX ix_machine_planogram_slots_version ON machine_planogram_slots (version_id);

ALTER TABLE machines
ADD COLUMN published_planogram_version_id uuid REFERENCES machine_planogram_versions (id) ON DELETE SET NULL;

ALTER TABLE machine_current_snapshot
ADD COLUMN last_acknowledged_config_revision INT NULL;

ALTER TABLE machine_current_snapshot
ADD COLUMN last_acknowledged_planogram_version_id UUID NULL;

-- +goose Down

ALTER TABLE machine_current_snapshot DROP COLUMN IF EXISTS last_acknowledged_planogram_version_id;

ALTER TABLE machine_current_snapshot DROP COLUMN IF EXISTS last_acknowledged_config_revision;

ALTER TABLE machines DROP COLUMN IF EXISTS published_planogram_version_id;

DROP TABLE IF EXISTS machine_planogram_slots;

DROP TABLE IF EXISTS machine_planogram_versions;

DROP TABLE IF EXISTS machine_planogram_drafts;

DROP TABLE IF EXISTS planogram_templates;
