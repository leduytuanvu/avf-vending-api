-- Post-wipe acceptance: goose_db_version only may have rows.
\set ON_ERROR_STOP on
SET statement_timeout = '120s';
SET lock_timeout = '5s';

\ir audit-all-table-rowcounts.sql

\echo ''
\echo '=== strict acceptance ==='
DO $$
DECLARE
  goose_rows bigint := 0;
  violations text := '';
  r record;
BEGIN
  FOR r IN
    SELECT tablename,
      (xpath('/row/c/text()', query_to_xml(
        format('SELECT count(*) AS c FROM %I.%I', schemaname, tablename),
        false, true, '')))[1]::text::int AS row_count
    FROM pg_tables
    WHERE schemaname = 'public'
    ORDER BY tablename
  LOOP
    IF r.tablename = 'goose_db_version' THEN
      goose_rows := r.row_count;
      CONTINUE;
    END IF;
    IF r.row_count > 0 THEN
      violations := violations || format(E'\n  - %s: %s rows', r.tablename, r.row_count);
    END IF;
  END LOOP;

  IF goose_rows < 1 THEN
    RAISE EXCEPTION 'verify-table-data-empty: goose_db_version is empty (migration metadata lost)';
  END IF;

  IF violations <> '' THEN
    RAISE EXCEPTION 'verify-table-data-empty: non-empty tables after wipe:%', violations;
  END IF;

  RAISE NOTICE 'verify-table-data-empty: PASS';
END $$;
