-- Verification: after goose up on the squashed single-company baseline, these queries must return 0 rows.
-- Usage: psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f scripts/db/verify_public_schema_legacy_cleanup.sql

SELECT 'tables' AS check_kind, table_schema, table_name
FROM information_schema.tables
WHERE table_schema = 'public'
  AND (
    table_name ILIKE '%organization%'
    OR table_name ILIKE '%tenant%'
    OR table_name ILIKE '%scope%'
  );

SELECT 'columns' AS check_kind, table_name, column_name
FROM information_schema.columns
WHERE table_schema = 'public'
  AND (
    column_name ILIKE '%organization%'
    OR column_name ILIKE '%tenant%'
    OR column_name ILIKE '%scope%'
  );

SELECT 'indexes' AS check_kind, indexname, indexdef
FROM pg_indexes
WHERE schemaname = 'public'
  AND (
    indexname ILIKE '%organization%'
    OR indexname ILIKE '%tenant%'
    OR indexname ILIKE '%scope%'
    OR indexdef ILIKE '%organization%'
    OR indexdef ILIKE '%tenant%'
    OR indexdef ILIKE '%scope%'
  );

SELECT 'constraints' AS check_kind, conname, pg_get_constraintdef(oid) AS def
FROM pg_constraint
WHERE connamespace = 'public'::regnamespace
  AND (
    conname ILIKE '%organization%'
    OR conname ILIKE '%tenant%'
    OR conname ILIKE '%scope%'
    OR pg_get_constraintdef(oid) ILIKE '%organization%'
    OR pg_get_constraintdef(oid) ILIKE '%tenant%'
    OR pg_get_constraintdef(oid) ILIKE '%scope%'
  );

SELECT 'views' AS check_kind, table_schema, table_name, view_definition
FROM information_schema.views
WHERE table_schema = 'public'
  AND (
    table_name ILIKE '%organization%'
    OR table_name ILIKE '%tenant%'
    OR table_name ILIKE '%scope%'
    OR view_definition ILIKE '%organization%'
    OR view_definition ILIKE '%tenant%'
    OR view_definition ILIKE '%scope%'
  );
