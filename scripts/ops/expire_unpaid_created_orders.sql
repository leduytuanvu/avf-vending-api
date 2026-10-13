-- Expire unpaid orders stuck in created/quoted past checkout window (ops; does NOT touch paid/vending).
--
-- Usage:
--   psql "$DATABASE_URL" -v dry_run=1 -v stale_minutes=30 -f scripts/ops/expire_unpaid_created_orders.sql
--   psql "$DATABASE_URL" -v dry_run=0 -v stale_minutes=30 -f scripts/ops/expire_unpaid_created_orders.sql
--
-- Optional machine filter:
--   psql ... -v machine_code='AVF000109' -f scripts/ops/expire_unpaid_created_orders.sql

\set ON_ERROR_STOP on

\if :{?stale_minutes}
\else
\set stale_minutes 30
\endif

\if :{?machine_code}
\else
\set machine_code ''
\endif

\echo '=== Preview unpaid created/quoted older than :stale_minutes minutes ==='
SELECT o.id,
       o.status,
       o.total_minor,
       m.code AS machine_code,
       o.created_at,
       o.updated_at,
       p.state AS payment_state
FROM orders o
JOIN machines m ON m.id = o.machine_id
LEFT JOIN LATERAL (
    SELECT state
    FROM payments
    WHERE order_id = o.id
    ORDER BY created_at DESC
    LIMIT 1
) p ON true
WHERE o.status IN ('created', 'quoted')
  AND o.created_at < now() - (:'stale_minutes' || ' minutes')::interval
  AND (p.state IS NULL OR p.state NOT IN ('captured', 'partially_refunded', 'authorized'))
  AND (:'machine_code' = '' OR m.code = :'machine_code')
ORDER BY o.created_at;

\if :dry_run
\echo '=== DRY RUN: no writes ==='
\else
BEGIN;

UPDATE orders o
SET status = 'cancelled',
    updated_at = now()
FROM machines m
WHERE o.machine_id = m.id
  AND o.status IN ('created', 'quoted')
  AND o.created_at < now() - (:'stale_minutes' || ' minutes')::interval
  AND (:'machine_code' = '' OR m.code = :'machine_code')
  AND NOT EXISTS (
      SELECT 1
      FROM payments p
      WHERE p.order_id = o.id
        AND p.state IN ('captured', 'partially_refunded', 'authorized')
  );

COMMIT;
\endif
