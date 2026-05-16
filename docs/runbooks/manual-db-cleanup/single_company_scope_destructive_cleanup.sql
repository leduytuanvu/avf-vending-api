-- MANUAL ONLY — do not add to migrations/ or goose.
-- Run after backup + approval; see README.md in this directory.

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

    -- Drop indexes that only existed for old scope scoping patterns.
    FOR r IN
        SELECT schemaname AS table_schema, indexname
        FROM pg_indexes
        WHERE schemaname = 'public'
          AND (indexdef ILIKE '%scope_id%' OR indexname ILIKE 'ux\_%\_org\_%' ESCAPE '\' OR indexname ILIKE 'ix\_%\_company%')
    LOOP
        EXECUTE format('DROP INDEX IF EXISTS %I.%I', r.table_schema, r.indexname);
    END LOOP;

    -- Remove all scope_id columns from tables.
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

    -- Remove target columns that pointed at the old company entity.
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
