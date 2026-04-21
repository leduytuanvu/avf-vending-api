-- +goose Up
-- +goose StatementBegin

ALTER TABLE machine_cabinets
ADD COLUMN cabinet_index int NOT NULL DEFAULT 0;

ALTER TABLE machine_cabinets
ADD COLUMN slot_capacity int NULL;

ALTER TABLE machine_cabinets
ADD COLUMN status text NOT NULL DEFAULT 'active';

ALTER TABLE machine_cabinets
ADD CONSTRAINT ck_machine_cabinets_slot_capacity_nonneg CHECK (slot_capacity IS NULL OR slot_capacity >= 0);

ALTER TABLE machine_cabinets
ADD CONSTRAINT ck_machine_cabinets_status CHECK (status IN ('active', 'inactive', 'maintenance'));

COMMENT ON COLUMN machine_cabinets.cabinet_index IS 'Stable ordering key per machine (0-based); backfilled from sort_order for existing rows.';

COMMENT ON COLUMN machine_cabinets.slot_capacity IS 'Optional max slots for this cabinet; null when unknown or unconstrained.';

COMMENT ON COLUMN machine_cabinets.status IS 'Operational status for this cabinet; default active.';

UPDATE machine_cabinets
SET
    cabinet_index = sort_order
WHERE
    true;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE machine_cabinets DROP CONSTRAINT IF EXISTS ck_machine_cabinets_status;

ALTER TABLE machine_cabinets DROP CONSTRAINT IF EXISTS ck_machine_cabinets_slot_capacity_nonneg;

ALTER TABLE machine_cabinets DROP COLUMN IF EXISTS status;

ALTER TABLE machine_cabinets DROP COLUMN IF EXISTS slot_capacity;

ALTER TABLE machine_cabinets DROP COLUMN IF EXISTS cabinet_index;

-- +goose StatementEnd
