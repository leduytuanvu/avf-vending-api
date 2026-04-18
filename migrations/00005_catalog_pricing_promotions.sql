-- +goose Up
-- +goose StatementBegin

CREATE EXTENSION IF NOT EXISTS btree_gist;

-- ---------------------------------------------------------------------------
-- Categories & brands (tenant-scoped catalog)
-- ---------------------------------------------------------------------------

CREATE TABLE categories (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    slug text NOT NULL,
    name text NOT NULL,
    parent_id uuid REFERENCES categories (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ux_categories_org_slug_lower ON categories (organization_id, lower(slug));
CREATE UNIQUE INDEX ux_categories_org_id ON categories (organization_id, id);

CREATE INDEX ix_categories_organization_id ON categories (organization_id);
CREATE INDEX ix_categories_parent_id ON categories (parent_id);

COMMENT ON TABLE categories IS 'Product taxonomy per organization; slug unique per org (case-insensitive).';

CREATE TABLE brands (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    slug text NOT NULL,
    name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ux_brands_org_slug_lower ON brands (organization_id, lower(slug));
CREATE UNIQUE INDEX ux_brands_org_id ON brands (organization_id, id);

CREATE INDEX ix_brands_organization_id ON brands (organization_id);

COMMENT ON TABLE brands IS 'Manufacturer / brand labels scoped to organization.';

-- ---------------------------------------------------------------------------
-- Product images (canonical imagery; products.attrs may hold legacy keys only)
-- ---------------------------------------------------------------------------

CREATE TABLE product_images (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id uuid NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    storage_key text NOT NULL,
    cdn_url text,
    alt_text text NOT NULL DEFAULT '',
    sort_order int NOT NULL DEFAULT 0,
    is_primary boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ux_product_images_one_primary_per_product ON product_images (product_id) WHERE is_primary;

CREATE UNIQUE INDEX ux_product_images_product_id_id ON product_images (product_id, id);

CREATE INDEX ix_product_images_product_id ON product_images (product_id);

COMMENT ON TABLE product_images IS 'Authoritative product imagery; use storage_key with object store. At most one is_primary per product (partial unique).';

-- ---------------------------------------------------------------------------
-- Products: catalog links, compliance hints, optional primary image pointer
-- ---------------------------------------------------------------------------

ALTER TABLE products
    ADD COLUMN category_id uuid,
    ADD COLUMN brand_id uuid,
    ADD COLUMN primary_image_id uuid,
    ADD COLUMN country_of_origin text,
    ADD COLUMN age_restricted boolean NOT NULL DEFAULT false,
    ADD COLUMN allergen_codes text[],
    ADD COLUMN nutritional_note text;

ALTER TABLE products
    ADD CONSTRAINT fk_products_org_category FOREIGN KEY (organization_id, category_id)
        REFERENCES categories (organization_id, id),
    ADD CONSTRAINT fk_products_org_brand FOREIGN KEY (organization_id, brand_id)
        REFERENCES brands (organization_id, id),
    ADD CONSTRAINT fk_products_primary_image FOREIGN KEY (id, primary_image_id)
        REFERENCES product_images (product_id, id) DEFERRABLE INITIALLY DEFERRED;

COMMENT ON COLUMN products.attrs IS 'Flexible JSON; authoritative imagery is product_images, not attrs.';
COMMENT ON COLUMN products.primary_image_id IS 'Optional pointer to primary product_images row; do not add products.image_url.';
COMMENT ON COLUMN products.country_of_origin IS 'Optional compliance; extended nutrition may live in attrs under stable keys.';

CREATE UNIQUE INDEX ux_products_org_id ON products (organization_id, id);

COMMENT ON CONSTRAINT fk_products_primary_image ON products IS 'primary_image_id must reference product_images for this same product row.';

-- ---------------------------------------------------------------------------
-- Price books: scope + precedence (machine > site > org, then priority, then effective_from)
-- ---------------------------------------------------------------------------

CREATE UNIQUE INDEX ux_sites_org_id ON sites (organization_id, id);
CREATE UNIQUE INDEX ux_machines_org_id ON machines (organization_id, id);

ALTER TABLE price_books
    ADD COLUMN scope_type text NOT NULL DEFAULT 'organization' CHECK (scope_type IN ('organization', 'site', 'machine')),
    ADD COLUMN site_id uuid,
    ADD COLUMN machine_id uuid,
    ADD COLUMN priority int NOT NULL DEFAULT 0;

ALTER TABLE price_books
    ADD CONSTRAINT ck_price_books_scope_shape CHECK (
        (scope_type = 'organization' AND site_id IS NULL AND machine_id IS NULL)
        OR (scope_type = 'site' AND site_id IS NOT NULL AND machine_id IS NULL)
        OR (scope_type = 'machine' AND machine_id IS NOT NULL AND site_id IS NULL)
    );

COMMENT ON COLUMN price_books.scope_type IS 'organization = default org book; site = location-specific; machine = device override book.';
COMMENT ON COLUMN price_books.priority IS 'Higher wins within same scope level when multiple books match.';

ALTER TABLE price_books
    ADD CONSTRAINT fk_price_books_org_site FOREIGN KEY (organization_id, site_id)
        REFERENCES sites (organization_id, id),
    ADD CONSTRAINT fk_price_books_org_machine FOREIGN KEY (organization_id, machine_id)
        REFERENCES machines (organization_id, id);

CREATE UNIQUE INDEX ux_price_books_org_id ON price_books (organization_id, id);

CREATE UNIQUE INDEX ux_price_books_org_scope_org_name_effective
    ON price_books (organization_id, lower(name), effective_from)
    WHERE scope_type = 'organization';

CREATE UNIQUE INDEX ux_price_books_org_scope_site_name_effective
    ON price_books (organization_id, site_id, lower(name), effective_from)
    WHERE scope_type = 'site';

CREATE UNIQUE INDEX ux_price_books_org_scope_machine_name_effective
    ON price_books (organization_id, machine_id, lower(name), effective_from)
    WHERE scope_type = 'machine';

-- ---------------------------------------------------------------------------
-- Price book items: tenant-safe link to book + product
-- ---------------------------------------------------------------------------

ALTER TABLE price_book_items
    ADD COLUMN organization_id uuid;

UPDATE price_book_items pbi
SET organization_id = pb.organization_id
FROM price_books pb
WHERE pbi.price_book_id = pb.id;

ALTER TABLE price_book_items
    ALTER COLUMN organization_id SET NOT NULL;

ALTER TABLE price_book_items
    ADD CONSTRAINT fk_price_book_items_org_book FOREIGN KEY (organization_id, price_book_id)
        REFERENCES price_books (organization_id, id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_price_book_items_org_product FOREIGN KEY (organization_id, product_id)
        REFERENCES products (organization_id, id) ON DELETE RESTRICT;

ALTER TABLE price_book_items DROP CONSTRAINT ux_price_book_items_book_product;

ALTER TABLE price_book_items
    ADD CONSTRAINT ux_price_book_items_org_book_product UNIQUE (organization_id, price_book_id, product_id);

CREATE INDEX ix_price_book_items_organization_id ON price_book_items (organization_id);

-- ---------------------------------------------------------------------------
-- Machine price overrides (no overlapping active windows per machine+product)
-- ---------------------------------------------------------------------------

CREATE TABLE machine_price_overrides (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    product_id uuid NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
    unit_price_minor bigint NOT NULL CHECK (unit_price_minor >= 0),
    currency char(3) NOT NULL,
    valid_from timestamptz NOT NULL,
    valid_to timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fk_machine_price_overrides_org_machine FOREIGN KEY (organization_id, machine_id)
        REFERENCES machines (organization_id, id),
    CONSTRAINT fk_machine_price_overrides_org_product FOREIGN KEY (organization_id, product_id)
        REFERENCES products (organization_id, id)
);

ALTER TABLE machine_price_overrides
    ADD CONSTRAINT ex_machine_price_overrides_no_overlap EXCLUDE USING gist (
        machine_id WITH =,
        product_id WITH =,
        tstzrange(valid_from, COALESCE(valid_to, 'infinity'::timestamptz), '[)') WITH &&
    );

CREATE INDEX ix_machine_price_overrides_machine_product_valid
    ON machine_price_overrides (machine_id, product_id, valid_from DESC);

CREATE INDEX ix_machine_price_overrides_organization_id ON machine_price_overrides (organization_id);

COMMENT ON TABLE machine_price_overrides IS 'Time-bounded per-machine product price; EXCLUDE prevents overlapping active intervals.';

-- ---------------------------------------------------------------------------
-- Promotions
-- ---------------------------------------------------------------------------

CREATE TABLE promotions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    name text NOT NULL,
    approval_status text NOT NULL DEFAULT 'draft' CHECK (
        approval_status IN ('draft', 'pending_approval', 'approved', 'rejected', 'archived')
    ),
    starts_at timestamptz NOT NULL,
    ends_at timestamptz NOT NULL,
    budget_limit_minor bigint,
    redemption_limit int CHECK (redemption_limit IS NULL OR redemption_limit >= 0),
    channel_scope text CHECK (channel_scope IS NULL OR channel_scope IN ('in_person', 'mobile', 'all')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ux_promotions_org_id ON promotions (organization_id, id);

CREATE INDEX ix_promotions_organization_id ON promotions (organization_id);
CREATE INDEX ix_promotions_org_approval ON promotions (organization_id, approval_status);

COMMENT ON TABLE promotions IS 'Marketing / pricing campaigns; approval_status is governance; runtime redemption enforced in app.';

CREATE TABLE promotion_rules (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    promotion_id uuid NOT NULL REFERENCES promotions (id) ON DELETE CASCADE,
    rule_type text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    priority int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ux_promotion_rules_promo_type_priority UNIQUE (promotion_id, rule_type, priority)
);

CREATE INDEX ix_promotion_rules_promotion_id ON promotion_rules (promotion_id);

COMMENT ON TABLE promotion_rules IS 'rule_type examples: pct_off, fixed_off, bogo, min_cart; payload shape is per rule_type (document in app).';

CREATE TABLE promotion_targets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    promotion_id uuid NOT NULL REFERENCES promotions (id) ON DELETE CASCADE,
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    target_type text NOT NULL CHECK (target_type IN ('product', 'category', 'machine', 'site', 'organization')),
    product_id uuid,
    category_id uuid,
    machine_id uuid,
    site_id uuid,
    organization_target_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fk_promotion_targets_org_promo FOREIGN KEY (organization_id, promotion_id)
        REFERENCES promotions (organization_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_pt_org_product FOREIGN KEY (organization_id, product_id)
        REFERENCES products (organization_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_pt_org_category FOREIGN KEY (organization_id, category_id)
        REFERENCES categories (organization_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_pt_org_machine FOREIGN KEY (organization_id, machine_id)
        REFERENCES machines (organization_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_pt_org_site FOREIGN KEY (organization_id, site_id)
        REFERENCES sites (organization_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_pt_organization_target FOREIGN KEY (organization_target_id)
        REFERENCES organizations (id) ON DELETE CASCADE,
    CONSTRAINT ck_promotion_targets_one_target CHECK (
        (target_type = 'product' AND product_id IS NOT NULL AND category_id IS NULL AND machine_id IS NULL AND site_id IS NULL AND organization_target_id IS NULL)
        OR (target_type = 'category' AND category_id IS NOT NULL AND product_id IS NULL AND machine_id IS NULL AND site_id IS NULL AND organization_target_id IS NULL)
        OR (target_type = 'machine' AND machine_id IS NOT NULL AND product_id IS NULL AND category_id IS NULL AND site_id IS NULL AND organization_target_id IS NULL)
        OR (target_type = 'site' AND site_id IS NOT NULL AND product_id IS NULL AND category_id IS NULL AND machine_id IS NULL AND organization_target_id IS NULL)
        OR (target_type = 'organization' AND organization_target_id IS NOT NULL AND product_id IS NULL AND category_id IS NULL AND machine_id IS NULL AND site_id IS NULL)
    )
);

CREATE INDEX ix_promotion_targets_promotion_id ON promotion_targets (promotion_id);
CREATE INDEX ix_promotion_targets_organization_id ON promotion_targets (organization_id);

COMMENT ON COLUMN promotion_targets.organization_target_id IS 'When target_type=organization, the organization this promotion applies to (often same as promotion.organization_id).';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS promotion_targets;
DROP TABLE IF EXISTS promotion_rules;
DROP TABLE IF EXISTS promotions;

DROP TABLE IF EXISTS machine_price_overrides;

DROP INDEX IF EXISTS ix_price_book_items_organization_id;

ALTER TABLE price_book_items DROP CONSTRAINT IF EXISTS ux_price_book_items_org_book_product;
ALTER TABLE price_book_items DROP CONSTRAINT IF EXISTS fk_price_book_items_org_book;
ALTER TABLE price_book_items DROP CONSTRAINT IF EXISTS fk_price_book_items_org_product;
ALTER TABLE price_book_items DROP COLUMN IF EXISTS organization_id;
ALTER TABLE price_book_items ADD CONSTRAINT ux_price_book_items_book_product UNIQUE (price_book_id, product_id);

ALTER TABLE price_books DROP CONSTRAINT IF EXISTS fk_price_books_org_site;
ALTER TABLE price_books DROP CONSTRAINT IF EXISTS fk_price_books_org_machine;
ALTER TABLE price_books DROP CONSTRAINT IF EXISTS ck_price_books_scope_shape;
ALTER TABLE price_books DROP COLUMN IF EXISTS scope_type;
ALTER TABLE price_books DROP COLUMN IF EXISTS site_id;
ALTER TABLE price_books DROP COLUMN IF EXISTS machine_id;
ALTER TABLE price_books DROP COLUMN IF EXISTS priority;

DROP INDEX IF EXISTS ux_price_books_org_scope_org_name_effective;
DROP INDEX IF EXISTS ux_price_books_org_scope_site_name_effective;
DROP INDEX IF EXISTS ux_price_books_org_scope_machine_name_effective;
DROP INDEX IF EXISTS ux_price_books_org_id;

DROP INDEX IF EXISTS ux_sites_org_id;
DROP INDEX IF EXISTS ux_machines_org_id;

ALTER TABLE products DROP CONSTRAINT IF EXISTS fk_products_primary_image;
ALTER TABLE products DROP CONSTRAINT IF EXISTS fk_products_org_category;
ALTER TABLE products DROP CONSTRAINT IF EXISTS fk_products_org_brand;
ALTER TABLE products DROP COLUMN IF EXISTS category_id;
ALTER TABLE products DROP COLUMN IF EXISTS brand_id;
ALTER TABLE products DROP COLUMN IF EXISTS primary_image_id;
ALTER TABLE products DROP COLUMN IF EXISTS country_of_origin;
ALTER TABLE products DROP COLUMN IF EXISTS age_restricted;
ALTER TABLE products DROP COLUMN IF EXISTS allergen_codes;
ALTER TABLE products DROP COLUMN IF EXISTS nutritional_note;

DROP INDEX IF EXISTS ux_products_org_id;

DROP INDEX IF EXISTS ux_brands_org_slug_lower;
DROP INDEX IF EXISTS ux_categories_org_slug_lower;
DROP TABLE IF EXISTS product_images;
DROP TABLE IF EXISTS brands;
DROP TABLE IF EXISTS categories;

-- +goose StatementEnd
