-- +goose Up
-- +goose StatementBegin

-- Convert the final database shape to single-company. Historical migrations stay
-- immutable; this migration removes company ownership from the live schema.

DO $$
DECLARE
    r record;
BEGIN
    -- Remove feature/promotion/rollout target rows that specifically point at the
    -- old company table before dropping target columns/checks.
    FOR r IN
        SELECT c.table_schema, c.table_name
        FROM information_schema.columns c
        JOIN information_schema.tables t
          ON t.table_schema = c.table_schema AND t.table_name = c.table_name
        WHERE c.column_name = 'target_type'
          AND c.table_schema = 'public'
          AND t.table_type = 'BASE TABLE'
    LOOP
        EXECUTE format(
            'UPDATE %I.%I SET target_type = %L WHERE target_type = %L',
            r.table_schema,
            r.table_name,
            'global',
            'company'
        );
    END LOOP;

    -- Drop checks that mention the removed target kind.
    FOR r IN
        SELECT n.nspname AS table_schema, c.relname AS table_name, con.conname
        FROM pg_constraint con
        JOIN pg_class c ON c.oid = con.conrelid
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'public'
          AND con.contype = 'c'
          AND pg_get_constraintdef(con.oid) ILIKE '%company%'
    LOOP
        EXECUTE format('ALTER TABLE %I.%I DROP CONSTRAINT IF EXISTS %I', r.table_schema, r.table_name, r.conname);
    END LOOP;

    -- Drop all FK/unique/check constraints that mention the old scope column.
    FOR r IN
        SELECT n.nspname AS table_schema, c.relname AS table_name, con.conname
        FROM pg_constraint con
        JOIN pg_class c ON c.oid = con.conrelid
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'public'
          AND pg_get_constraintdef(con.oid) ILIKE '%scope_id%'
    LOOP
        EXECUTE format('ALTER TABLE %I.%I DROP CONSTRAINT IF EXISTS %I', r.table_schema, r.table_name, r.conname);
    END LOOP;

    -- Drop indexes that only existed for company scoping.
    FOR r IN
        SELECT schemaname AS table_schema, indexname
        FROM pg_indexes
        WHERE schemaname = 'public'
          AND (indexdef ILIKE '%scope_id%' OR indexname ILIKE 'ux\_%\_org\_%' ESCAPE '\' OR indexname ILIKE 'ix\_%\_company%')
    LOOP
        EXECUTE format('DROP INDEX IF EXISTS %I.%I', r.table_schema, r.indexname);
    END LOOP;

    -- Remove all company scope columns from tables.
    FOR r IN
        SELECT c.table_schema, c.table_name
        FROM information_schema.columns c
        JOIN information_schema.tables t
          ON t.table_schema = c.table_schema AND t.table_name = c.table_name
        WHERE c.column_name = 'scope_id'
          AND c.table_schema = 'public'
          AND t.table_type = 'BASE TABLE'
    LOOP
        EXECUTE format('ALTER TABLE %I.%I DROP COLUMN IF EXISTS scope_id CASCADE', r.table_schema, r.table_name);
    END LOOP;

    -- Remove target columns that pointed to the old company entity.
    FOR r IN
        SELECT c.table_schema, c.table_name
        FROM information_schema.columns c
        JOIN information_schema.tables t
          ON t.table_schema = c.table_schema AND t.table_name = c.table_name
        WHERE c.column_name = 'company_target_id'
          AND c.table_schema = 'public'
          AND t.table_type = 'BASE TABLE'
    LOOP
        EXECUTE format('ALTER TABLE %I.%I DROP COLUMN IF EXISTS company_target_id CASCADE', r.table_schema, r.table_name);
    END LOOP;

    DROP TABLE IF EXISTS companies CASCADE;
END $$;

-- Single-company uniqueness replacements.
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
    RAISE EXCEPTION '00073_remove_companies_single_company is intentionally irreversible; restore from backup for rollback';
END $$;

-- +goose StatementEnd
