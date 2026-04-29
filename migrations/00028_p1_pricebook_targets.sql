-- +goose Up
-- +goose StatementBegin

ALTER TABLE price_books
    ADD COLUMN IF NOT EXISTS active boolean NOT NULL DEFAULT true;

ALTER TABLE price_books
    ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

COMMENT ON COLUMN price_books.active IS 'When false, the book is deactivated and ignored for pricing resolution.';
COMMENT ON COLUMN price_books.updated_at IS 'Last mutation timestamp for admin PATCH/item/target updates.';

-- Assignment of organization-scoped price books to specific machines or sites (many-to-many).
CREATE TABLE price_book_targets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    price_book_id uuid NOT NULL,
    site_id uuid,
    machine_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fk_price_book_targets_org_book FOREIGN KEY (organization_id, price_book_id)
        REFERENCES price_books (organization_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_price_book_targets_org_site FOREIGN KEY (organization_id, site_id)
        REFERENCES sites (organization_id, id),
    CONSTRAINT fk_price_book_targets_org_machine FOREIGN KEY (organization_id, machine_id)
        REFERENCES machines (organization_id, id),
    CONSTRAINT ck_price_book_targets_exactly_one CHECK (
        (site_id IS NOT NULL AND machine_id IS NULL)
        OR (machine_id IS NOT NULL AND site_id IS NULL)
    )
);

CREATE UNIQUE INDEX ux_price_book_targets_book_machine
    ON price_book_targets (price_book_id, machine_id)
    WHERE machine_id IS NOT NULL;

CREATE UNIQUE INDEX ux_price_book_targets_book_site
    ON price_book_targets (price_book_id, site_id)
    WHERE site_id IS NOT NULL;

CREATE INDEX ix_price_book_targets_organization_id ON price_book_targets (organization_id);
CREATE INDEX ix_price_book_targets_book ON price_book_targets (organization_id, price_book_id);

COMMENT ON TABLE price_book_targets IS 'Links organization-scoped price books to a machine or site for layered pricing; precedence vs scoped rows is resolved at preview time.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS price_book_targets;

ALTER TABLE price_books DROP COLUMN IF EXISTS updated_at;
ALTER TABLE price_books DROP COLUMN IF EXISTS active;

-- +goose StatementEnd
