-- +goose Up
-- +goose StatementBegin

ALTER TABLE organizations
ADD COLUMN default_timezone text NOT NULL DEFAULT 'UTC';

ALTER TABLE sites
ADD COLUMN timezone text NOT NULL DEFAULT 'UTC';

ALTER TABLE machines
ADD COLUMN timezone_override text NULL;

ALTER TABLE machine_current_snapshot
ADD COLUMN android_id text NULL;

ALTER TABLE machine_current_snapshot
ADD COLUMN sim_serial text NULL;

ALTER TABLE machine_current_snapshot
ADD COLUMN sim_iccid text NULL;

ALTER TABLE machine_current_snapshot
ADD COLUMN device_model text NULL;

ALTER TABLE machine_current_snapshot
ADD COLUMN os_version text NULL;

ALTER TABLE machine_current_snapshot
ADD COLUMN last_identity_at timestamptz NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE machine_current_snapshot DROP COLUMN IF EXISTS last_identity_at;

ALTER TABLE machine_current_snapshot DROP COLUMN IF EXISTS os_version;

ALTER TABLE machine_current_snapshot DROP COLUMN IF EXISTS device_model;

ALTER TABLE machine_current_snapshot DROP COLUMN IF EXISTS sim_iccid;

ALTER TABLE machine_current_snapshot DROP COLUMN IF EXISTS sim_serial;

ALTER TABLE machine_current_snapshot DROP COLUMN IF EXISTS android_id;

ALTER TABLE machines DROP COLUMN IF EXISTS timezone_override;

ALTER TABLE sites DROP COLUMN IF EXISTS timezone;

ALTER TABLE organizations DROP COLUMN IF EXISTS default_timezone;

-- +goose StatementEnd
