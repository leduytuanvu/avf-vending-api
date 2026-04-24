-- +goose Up
-- +goose StatementBegin

-- P1 catalog: barcode, soft-delete flags, tags, richer product images.
ALTER TABLE products
    ADD COLUMN IF NOT EXISTS barcode text;

CREATE UNIQUE INDEX IF NOT EXISTS ux_products_org_barcode_lower
    ON products (organization_id, lower(trim(barcode)))
    WHERE barcode IS NOT NULL AND length(trim(barcode)) > 0;

ALTER TABLE brands
    ADD COLUMN IF NOT EXISTS active boolean NOT NULL DEFAULT true;

ALTER TABLE categories
    ADD COLUMN IF NOT EXISTS active boolean NOT NULL DEFAULT true;

CREATE TABLE IF NOT EXISTS tags (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    slug text NOT NULL,
    name text NOT NULL,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_tags_org_slug_lower ON tags (organization_id, lower(slug));

CREATE INDEX IF NOT EXISTS ix_tags_organization_id ON tags (organization_id);

CREATE TABLE IF NOT EXISTS product_tags (
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    product_id uuid NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    tag_id uuid NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
    PRIMARY KEY (product_id, tag_id)
);

CREATE INDEX IF NOT EXISTS ix_product_tags_org ON product_tags (organization_id);

ALTER TABLE product_images
    ADD COLUMN IF NOT EXISTS thumb_cdn_url text,
    ADD COLUMN IF NOT EXISTS content_hash text,
    ADD COLUMN IF NOT EXISTS width int,
    ADD COLUMN IF NOT EXISTS height int,
    ADD COLUMN IF NOT EXISTS mime_type text;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE product_images
    DROP COLUMN IF EXISTS mime_type,
    DROP COLUMN IF EXISTS height,
    DROP COLUMN IF EXISTS width,
    DROP COLUMN IF EXISTS content_hash,
    DROP COLUMN IF EXISTS thumb_cdn_url;

DROP TABLE IF EXISTS product_tags;
DROP INDEX IF EXISTS ux_tags_org_slug_lower;
DROP TABLE IF EXISTS tags;

ALTER TABLE categories DROP COLUMN IF EXISTS active;
ALTER TABLE brands DROP COLUMN IF EXISTS active;

DROP INDEX IF EXISTS ux_products_org_barcode_lower;
ALTER TABLE products DROP COLUMN IF EXISTS barcode;
-- +goose StatementEnd
