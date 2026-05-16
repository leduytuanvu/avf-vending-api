-- +goose Up
-- +goose StatementBegin

-- Single-company compatibility: add global uniqueness indexes expected by the runtime.
-- Destructive cleanup of legacy scope columns, constraints, and the companies table is NOT
-- applied automatically — operators must run the manual script after backup + approval:
--   docs/runbooks/manual-db-cleanup/single_company_scope_destructive_cleanup.sql
-- See docs/runbooks/manual-db-cleanup/README.md for prerequisites and verification.

CREATE UNIQUE INDEX IF NOT EXISTS uniq_regions_code_lower ON regions (lower(code));
CREATE UNIQUE INDEX IF NOT EXISTS uniq_sites_code_lower ON sites (lower(code)) WHERE btrim(code) <> '';
CREATE UNIQUE INDEX IF NOT EXISTS uniq_machines_serial_lower ON machines (lower(trim(serial_number))) WHERE btrim(serial_number) <> '';
CREATE UNIQUE INDEX IF NOT EXISTS uniq_machines_code_lower ON machines (lower(code)) WHERE btrim(code) <> '';
CREATE UNIQUE INDEX IF NOT EXISTS uniq_technicians_email_lower ON technicians (lower(email)) WHERE email IS NOT NULL AND btrim(email) <> '';
CREATE UNIQUE INDEX IF NOT EXISTS uniq_categories_slug_lower ON categories (lower(slug));
CREATE UNIQUE INDEX IF NOT EXISTS uniq_brands_slug_lower ON brands (lower(slug));
CREATE UNIQUE INDEX IF NOT EXISTS uniq_products_sku ON products (sku);
CREATE UNIQUE INDEX IF NOT EXISTS uniq_products_barcode_lower ON products (lower(trim(barcode))) WHERE barcode IS NOT NULL AND btrim(barcode) <> '';
CREATE UNIQUE INDEX IF NOT EXISTS uniq_tags_slug_lower ON tags (lower(slug));
CREATE UNIQUE INDEX IF NOT EXISTS uniq_price_books_name_effective ON price_books (lower(name), effective_from) WHERE scope_type = 'global';
CREATE UNIQUE INDEX IF NOT EXISTS uniq_price_books_site_name_effective ON price_books (site_id, lower(name), effective_from) WHERE scope_type = 'site';
CREATE UNIQUE INDEX IF NOT EXISTS uniq_price_books_machine_name_effective ON price_books (machine_id, lower(name), effective_from) WHERE scope_type = 'machine';
CREATE UNIQUE INDEX IF NOT EXISTS uniq_price_book_items_book_product ON price_book_items (price_book_id, product_id);
CREATE UNIQUE INDEX IF NOT EXISTS uniq_planograms_name_revision ON planograms (name, revision);
CREATE UNIQUE INDEX IF NOT EXISTS uniq_orders_idempotency ON orders (idempotency_key) WHERE idempotency_key IS NOT NULL AND btrim(idempotency_key) <> '';
CREATE UNIQUE INDEX IF NOT EXISTS uniq_feature_flags_key ON feature_flags (flag_key);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DO $$
BEGIN
    RAISE EXCEPTION '00073_single_company_scope_consolidation is intentionally irreversible; restore from backup for rollback';
END $$;

-- +goose StatementEnd
