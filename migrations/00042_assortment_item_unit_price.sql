-- Named product lists: optional per-membership list price (does not change price books).
-- +goose Up
-- +goose StatementBegin

ALTER TABLE assortment_items
    ADD COLUMN IF NOT EXISTS unit_price_minor bigint;

ALTER TABLE assortment_items
    DROP CONSTRAINT IF EXISTS ck_assortment_items_unit_price_minor_nonneg;

ALTER TABLE assortment_items
    ADD CONSTRAINT ck_assortment_items_unit_price_minor_nonneg CHECK (
        unit_price_minor IS NULL
        OR unit_price_minor >= 0
    );

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE assortment_items
    DROP CONSTRAINT IF EXISTS ck_assortment_items_unit_price_minor_nonneg;

ALTER TABLE assortment_items
    DROP COLUMN IF EXISTS unit_price_minor;

-- +goose StatementEnd
