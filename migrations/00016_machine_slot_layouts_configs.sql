-- +goose Up
-- +goose StatementBegin

-- Slot layout revisions and versioned slot configs (current row per machine_id + slot_code via partial unique index).

CREATE TABLE IF NOT EXISTS machine_slot_layouts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    machine_cabinet_id uuid NOT NULL REFERENCES machine_cabinets (id) ON DELETE CASCADE,
    layout_key text NOT NULL,
    revision int NOT NULL DEFAULT 1,
    layout_spec jsonb NOT NULL DEFAULT '{}'::jsonb,
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_machine_slot_layouts_layout_key_nonempty CHECK (btrim(layout_key) <> ''),
    CONSTRAINT ck_machine_slot_layouts_revision_positive CHECK (revision >= 1),
    CONSTRAINT fk_machine_slot_layouts_org_machine FOREIGN KEY (organization_id, machine_id) REFERENCES machines (organization_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_machine_slot_layouts_machine_cabinet FOREIGN KEY (machine_cabinet_id) REFERENCES machine_cabinets (id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_machine_slot_layouts_machine_cabinet_key_revision ON machine_slot_layouts (machine_id, machine_cabinet_id, layout_key, revision);

CREATE INDEX IF NOT EXISTS ix_machine_slot_layouts_machine_cabinet ON machine_slot_layouts (machine_id, machine_cabinet_id, created_at DESC);

CREATE INDEX IF NOT EXISTS ix_machine_slot_layouts_org ON machine_slot_layouts (organization_id, created_at DESC);

CREATE TABLE IF NOT EXISTS machine_slot_configs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    machine_cabinet_id uuid NOT NULL REFERENCES machine_cabinets (id) ON DELETE CASCADE,
    machine_slot_layout_id uuid NOT NULL REFERENCES machine_slot_layouts (id) ON DELETE RESTRICT,
    slot_code text NOT NULL,
    slot_index int CHECK (
        slot_index IS NULL
        OR slot_index >= 0
    ),
    product_id uuid,
    max_quantity int NOT NULL DEFAULT 0 CHECK (max_quantity >= 0),
    price_minor bigint NOT NULL DEFAULT 0 CHECK (price_minor >= 0),
    effective_from timestamptz NOT NULL DEFAULT now(),
    effective_to timestamptz,
    is_current boolean NOT NULL DEFAULT false,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_machine_slot_configs_slot_code_nonempty CHECK (btrim(slot_code) <> ''),
    CONSTRAINT fk_machine_slot_configs_org_machine FOREIGN KEY (organization_id, machine_id) REFERENCES machines (organization_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_machine_slot_configs_machine_cabinet FOREIGN KEY (machine_cabinet_id) REFERENCES machine_cabinets (id) ON DELETE CASCADE,
    CONSTRAINT fk_machine_slot_configs_org_product FOREIGN KEY (organization_id, product_id) REFERENCES products (organization_id, id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_machine_slot_configs_current_machine_slot ON machine_slot_configs (machine_id, slot_code)
WHERE
    is_current;

CREATE INDEX IF NOT EXISTS ix_machine_slot_configs_machine_current ON machine_slot_configs (machine_id)
WHERE
    is_current;

CREATE INDEX IF NOT EXISTS ix_machine_slot_configs_layout ON machine_slot_configs (machine_slot_layout_id);

CREATE INDEX IF NOT EXISTS ix_machine_slot_configs_machine_cabinet_current ON machine_slot_configs (machine_cabinet_id)
WHERE
    is_current;

COMMENT ON TABLE machine_slot_layouts IS 'Cabinet-scoped slot grid / wiring metadata; layout_spec holds structured slot definitions.';

COMMENT ON TABLE machine_slot_configs IS 'Per-slot merchandising config; history via is_current / effective_*; at most one is_current row per (machine_id, slot_code).';

COMMENT ON INDEX ux_machine_slot_configs_current_machine_slot IS 'Partial unique: one current config row per physical slot_code on a machine.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS machine_slot_configs;

DROP TABLE IF EXISTS machine_slot_layouts;

-- +goose StatementEnd
