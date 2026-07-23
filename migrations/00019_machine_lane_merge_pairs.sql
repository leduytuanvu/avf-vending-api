-- TCN double-wide lane merge pairs (planogram merge sync).
-- +goose Up
-- +goose StatementBegin

CREATE TABLE machine_lane_merge_pairs (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v7(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    left_slot_code text NOT NULL,
    right_slot_code text NOT NULL,
    cabinet_code text NOT NULL DEFAULT '',
    layout_key text NOT NULL DEFAULT 'default',
    layout_revision int NOT NULL DEFAULT 1 CHECK (layout_revision >= 1),
    revision int NOT NULL DEFAULT 1 CHECK (revision >= 1),
    operator_session_id uuid NULL REFERENCES machine_operator_sessions (id) ON DELETE SET NULL,
    merged_at timestamptz NOT NULL DEFAULT now(),
    split_at timestamptz NULL,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_machine_lane_merge_pairs_left_nonempty CHECK (btrim(left_slot_code) <> ''),
    CONSTRAINT ck_machine_lane_merge_pairs_right_nonempty CHECK (btrim(right_slot_code) <> '')
);

CREATE UNIQUE INDEX ux_machine_lane_merge_pairs_active_left ON machine_lane_merge_pairs (machine_id, left_slot_code)
WHERE
    is_active;

CREATE UNIQUE INDEX ux_machine_lane_merge_pairs_active_right ON machine_lane_merge_pairs (machine_id, right_slot_code)
WHERE
    is_active;

CREATE INDEX ix_machine_lane_merge_pairs_machine_active ON machine_lane_merge_pairs (machine_id)
WHERE
    is_active;

COMMENT ON TABLE machine_lane_merge_pairs IS 'Active TCN lane merge pairs (double-wide slots); split sets is_active=false.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS machine_lane_merge_pairs;

-- +goose StatementEnd
