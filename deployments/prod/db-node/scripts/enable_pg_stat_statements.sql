-- Live install was in schema extensions (same as pgcrypto). Move to public so
-- `SELECT * FROM pg_stat_statements` works without search_path changes.
ALTER EXTENSION pg_stat_statements SET SCHEMA public;
GRANT SELECT ON pg_stat_statements TO PUBLIC;
SELECT COUNT(*) AS stmt_rows FROM pg_stat_statements;
