-- Named machine layouts, snapshot history, and active-layout pointers (multi-layout architecture).
-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS machine_layouts (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v7(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    name text NOT NULL,
    status text NOT NULL DEFAULT 'READY' CHECK (status IN ('DRAFT', 'READY')),
    grid_rows int NOT NULL CHECK (grid_rows >= 1 AND grid_rows <= 26),
    grid_cols int NOT NULL CHECK (grid_cols >= 1 AND grid_cols <= 12),
    layout_revision int NOT NULL DEFAULT 1 CHECK (layout_revision >= 1),
    fingerprint text NOT NULL DEFAULT '',
    source_template_version_id uuid REFERENCES planogram_template_versions (id) ON DELETE SET NULL,
    is_archived boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_machine_layouts_name_nonempty CHECK (btrim(name) <> '')
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_machine_layouts_machine_name_active
    ON machine_layouts (machine_id, lower(btrim(name)))
    WHERE NOT is_archived;

CREATE INDEX IF NOT EXISTS ix_machine_layouts_machine
    ON machine_layouts (machine_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS machine_layout_slots (
    layout_id uuid NOT NULL REFERENCES machine_layouts (id) ON DELETE CASCADE,
    slot_code text NOT NULL,
    slot_ordinal int NOT NULL CHECK (slot_ordinal >= 1),
    slot_row int NULL,
    slot_column int NULL,
    physical_lane int NULL,
    board_address text NULL,
    channel_address text NULL,
    product_id uuid NULL,
    max_quantity int NOT NULL DEFAULT 0 CHECK (max_quantity >= 0 AND max_quantity <= 12),
    price_minor bigint NULL,
    current_inventory int NOT NULL DEFAULT 0 CHECK (current_inventory >= 0 AND current_inventory <= 12),
    enabled boolean NOT NULL DEFAULT true,
    operational_state text NOT NULL DEFAULT 'unassigned',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_machine_layout_slots PRIMARY KEY (layout_id, slot_code),
    CONSTRAINT ck_machine_layout_slots_code_nonempty CHECK (btrim(slot_code) <> '')
);

CREATE INDEX IF NOT EXISTS ix_machine_layout_slots_layout_ordinal
    ON machine_layout_slots (layout_id, slot_ordinal);

CREATE TABLE IF NOT EXISTS machine_layout_merge_pairs (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v7(),
    layout_id uuid NOT NULL REFERENCES machine_layouts (id) ON DELETE CASCADE,
    left_slot_code text NOT NULL,
    right_slot_code text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_machine_layout_merge_pairs_left_nonempty CHECK (btrim(left_slot_code) <> ''),
    CONSTRAINT ck_machine_layout_merge_pairs_right_nonempty CHECK (btrim(right_slot_code) <> ''),
    CONSTRAINT ux_machine_layout_merge_pairs_layout_left UNIQUE (layout_id, left_slot_code)
);

CREATE INDEX IF NOT EXISTS ix_machine_layout_merge_pairs_layout
    ON machine_layout_merge_pairs (layout_id);

CREATE TABLE IF NOT EXISTS machine_layout_device_state (
    machine_id uuid PRIMARY KEY REFERENCES machines (id) ON DELETE CASCADE,
    layout_id uuid NOT NULL REFERENCES machine_layouts (id) ON DELETE RESTRICT,
    latest_snapshot_id uuid NULL,
    latest_capture_sequence bigint NOT NULL DEFAULT 0,
    device_generation bigint NOT NULL DEFAULT 0,
    fingerprint text NOT NULL DEFAULT '',
    reported_at timestamptz NOT NULL DEFAULT now(),
    device_instance_id text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS machine_layout_snapshot_history (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v7(),
    snapshot_id uuid NOT NULL,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    layout_id uuid NOT NULL REFERENCES machine_layouts (id) ON DELETE RESTRICT,
    device_instance_id text NOT NULL,
    capture_sequence bigint NOT NULL,
    interval_key text NULL,
    captured_at timestamptz NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now(),
    device_generation bigint NOT NULL DEFAULT 0,
    base_server_revision int NULL,
    fingerprint text NOT NULL,
    snapshot_reason text NOT NULL DEFAULT 'PERIODIC_30M' CHECK (
        snapshot_reason IN ('PERIODIC_30M', 'MANUAL_SYNC', 'RECONNECT', 'ACTIVATION', 'LEGACY_REPORT')
    ),
    payload_version int NOT NULL DEFAULT 1,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ux_machine_layout_snapshot_history_snapshot_id UNIQUE (snapshot_id),
    CONSTRAINT ux_machine_layout_snapshot_history_device_sequence UNIQUE (machine_id, device_instance_id, capture_sequence)
);

CREATE INDEX IF NOT EXISTS ix_machine_layout_snapshot_history_machine_captured
    ON machine_layout_snapshot_history (machine_id, captured_at DESC);

CREATE INDEX IF NOT EXISTS ix_machine_layout_snapshot_history_machine_layout
    ON machine_layout_snapshot_history (machine_id, layout_id, captured_at DESC);

ALTER TABLE machines
    ADD COLUMN IF NOT EXISTS active_layout_id uuid REFERENCES machine_layouts (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS desired_active_layout_id uuid REFERENCES machine_layouts (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS reported_active_layout_id uuid REFERENCES machine_layouts (id) ON DELETE SET NULL;

COMMENT ON TABLE machine_layouts IS 'Named per-machine layout definitions (Layout 1, Summer, …); independent from SERVER/LOCAL sync source.';
COMMENT ON TABLE machine_layout_slots IS 'Structural + merchandising slot rows for a named machine layout.';
COMMENT ON TABLE machine_layout_merge_pairs IS 'Merge topology for a named layout (left+right adjacent pair).';
COMMENT ON TABLE machine_layout_device_state IS 'Latest device-reported state for the active layout (newest capture_sequence wins).';
COMMENT ON TABLE machine_layout_snapshot_history IS 'Immutable append-only device layout snapshots.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE machines
    DROP COLUMN IF EXISTS reported_active_layout_id,
    DROP COLUMN IF EXISTS desired_active_layout_id,
    DROP COLUMN IF EXISTS active_layout_id;

DROP TABLE IF EXISTS machine_layout_snapshot_history;
DROP TABLE IF EXISTS machine_layout_device_state;
DROP TABLE IF EXISTS machine_layout_merge_pairs;
DROP TABLE IF EXISTS machine_layout_slots;
DROP TABLE IF EXISTS machine_layouts;

-- +goose StatementEnd
