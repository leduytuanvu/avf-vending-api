-- Named product lists: common vs custom assortment kind.
-- +goose Up
-- +goose StatementBegin

ALTER TABLE assortments
    ADD COLUMN IF NOT EXISTS kind text NOT NULL DEFAULT 'custom';

ALTER TABLE assortments
    DROP CONSTRAINT IF EXISTS ck_assortments_kind;

ALTER TABLE assortments
    ADD CONSTRAINT ck_assortments_kind CHECK (kind IN ('common', 'custom'));

CREATE UNIQUE INDEX IF NOT EXISTS ux_assortments_one_common ON assortments (kind)
WHERE
    kind = 'common';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS ux_assortments_one_common;

ALTER TABLE assortments
    DROP CONSTRAINT IF EXISTS ck_assortments_kind;

ALTER TABLE assortments
    DROP COLUMN IF EXISTS kind;

-- +goose StatementEnd
