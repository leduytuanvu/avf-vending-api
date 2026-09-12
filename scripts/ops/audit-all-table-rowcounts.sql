-- Full public schema row-count audit (read-only).
-- Single-pass count per table with session timeouts.
--
-- Usage:
--   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f scripts/ops/audit-all-table-rowcounts.sql
--
-- For post-wipe strict acceptance use verify-table-data-empty.sql instead.

\set ON_ERROR_STOP on
SET statement_timeout = '120s';
SET lock_timeout = '5s';

\echo '=== audit-all-table-rowcounts: scanning public tables (read-only) ==='

CREATE TEMP TABLE _audit_rowcounts ON COMMIT DROP AS
SELECT
  tablename,
  (xpath('/row/c/text()', query_to_xml(
    format('SELECT count(*) AS c FROM %I.%I', schemaname, tablename),
    false, true, '')))[1]::text::int AS row_count
FROM pg_tables
WHERE schemaname = 'public';

\echo '=== tables with rows > 0 ==='
SELECT tablename, row_count
FROM _audit_rowcounts
WHERE row_count > 0
ORDER BY row_count DESC, tablename;

\echo ''
\echo '=== summary ==='
SELECT
  count(*) FILTER (WHERE row_count > 0 AND tablename <> 'goose_db_version') AS nonempty_business_tables,
  count(*) FILTER (WHERE tablename = 'goose_db_version') AS goose_table_present,
  max(row_count) FILTER (WHERE tablename = 'goose_db_version') AS goose_rows,
  count(*) AS total_public_tables
FROM _audit_rowcounts;
