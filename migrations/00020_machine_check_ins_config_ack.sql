-- +goose Up
-- +goose StatementBegin

CREATE TABLE machine_check_ins (
    id bigserial PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    android_id text,
    sim_serial text,
    package_name text NOT NULL DEFAULT '',
    version_name text NOT NULL DEFAULT '',
    version_code bigint NOT NULL DEFAULT 0,
    android_release text NOT NULL DEFAULT '',
    sdk_int int NOT NULL DEFAULT 0,
    manufacturer text NOT NULL DEFAULT '',
    model text NOT NULL DEFAULT '',
    timezone text NOT NULL DEFAULT '',
    network_state text NOT NULL DEFAULT '',
    boot_id text NOT NULL DEFAULT '',
    occurred_at timestamptz NOT NULL,
    recorded_at timestamptz NOT NULL DEFAULT now(),
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT fk_machine_check_ins_org_machine FOREIGN KEY (organization_id, machine_id) REFERENCES machines (organization_id, id) ON DELETE CASCADE
);

CREATE INDEX ix_machine_check_ins_machine_occurred ON machine_check_ins (machine_id, occurred_at DESC);

CREATE INDEX ix_machine_check_ins_org_occurred ON machine_check_ins (organization_id, occurred_at DESC);

COMMENT ON TABLE machine_check_ins IS 'Append-only Android device boot/runtime check-ins; occurred_at is client business time with timezone.';

ALTER TABLE machine_current_snapshot
ADD COLUMN IF NOT EXISTS last_check_in_at timestamptz NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE machine_current_snapshot DROP COLUMN IF EXISTS last_check_in_at;

DROP TABLE IF EXISTS machine_check_ins;

-- +goose StatementEnd
