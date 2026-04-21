-- +goose Up
-- +goose StatementBegin

ALTER TABLE inventory_events
ADD COLUMN cabinet_code text;

ALTER TABLE inventory_events
ADD COLUMN technician_id uuid REFERENCES technicians (id) ON DELETE SET NULL;

ALTER TABLE inventory_events
ADD COLUMN reason_code text;

ALTER TABLE inventory_events
ADD COLUMN quantity_before int;

ALTER TABLE inventory_events
ADD COLUMN unit_price_minor bigint NOT NULL DEFAULT 0;

ALTER TABLE inventory_events
ADD COLUMN currency text NOT NULL DEFAULT 'USD';

ALTER TABLE inventory_events
ADD COLUMN recorded_at timestamptz;

UPDATE inventory_events
SET
    recorded_at = occurred_at
WHERE
    recorded_at IS NULL;

UPDATE inventory_events
SET
    quantity_before = NULLIF((metadata ->> 'quantity_before'), '')::int
WHERE
    quantity_before IS NULL
    AND metadata ? 'quantity_before';

ALTER TABLE inventory_events
ALTER COLUMN recorded_at SET NOT NULL;

ALTER TABLE inventory_events
ALTER COLUMN recorded_at SET DEFAULT now();

ALTER TABLE inventory_events
ADD CONSTRAINT ck_inventory_events_cabinet_code_nonempty CHECK (
    cabinet_code IS NULL
    OR btrim(cabinet_code) <> ''
);

COMMENT ON COLUMN inventory_events.cabinet_code IS 'Denormalized cabinet label for auditing (alongside machine_cabinet_id when set).';

COMMENT ON COLUMN inventory_events.technician_id IS 'Technician on operator_session when actor is TECHNICIAN; denormalized for ledger reads.';

COMMENT ON COLUMN inventory_events.reason_code IS 'API or domain reason (e.g. restock, cycle_count, manual_adjustment).';

COMMENT ON COLUMN inventory_events.quantity_before IS 'Stock immediately before this event for the slot.';

COMMENT ON COLUMN inventory_events.unit_price_minor IS 'Unit price in minor units at event time (legacy planogram slot price when applicable).';

COMMENT ON COLUMN inventory_events.currency IS 'ISO currency code for unit_price_minor.';

COMMENT ON COLUMN inventory_events.recorded_at IS 'When the row was appended (ingestion time); differs from occurred_at when backdated.';

CREATE TABLE refill_session_lines (
    id bigserial PRIMARY KEY,
    refill_session_id uuid NOT NULL REFERENCES refill_sessions (id) ON DELETE CASCADE,
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    cabinet_code text NOT NULL,
    slot_code text NOT NULL,
    product_id uuid,
    before_quantity int NOT NULL,
    added_quantity int NOT NULL,
    after_quantity int NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_refill_session_lines_codes_nonempty CHECK (
        btrim(cabinet_code) <> ''
        AND btrim(slot_code) <> ''
    ),
    CONSTRAINT ck_refill_session_lines_nonneg CHECK (
        before_quantity >= 0
        AND after_quantity >= 0
    ),
    CONSTRAINT fk_refill_session_lines_org_product FOREIGN KEY (organization_id, product_id) REFERENCES products (organization_id, id) ON DELETE SET NULL
);

CREATE INDEX ix_refill_session_lines_session ON refill_session_lines (refill_session_id, created_at DESC);

COMMENT ON TABLE refill_session_lines IS 'Per-slot deltas recorded during a refill session; append-only.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS refill_session_lines;

ALTER TABLE inventory_events DROP CONSTRAINT IF EXISTS ck_inventory_events_cabinet_code_nonempty;

ALTER TABLE inventory_events DROP COLUMN IF EXISTS recorded_at;

ALTER TABLE inventory_events DROP COLUMN IF EXISTS currency;

ALTER TABLE inventory_events DROP COLUMN IF EXISTS unit_price_minor;

ALTER TABLE inventory_events DROP COLUMN IF EXISTS quantity_before;

ALTER TABLE inventory_events DROP COLUMN IF EXISTS reason_code;

ALTER TABLE inventory_events DROP COLUMN IF EXISTS technician_id;

ALTER TABLE inventory_events DROP COLUMN IF EXISTS cabinet_code;

-- +goose StatementEnd
