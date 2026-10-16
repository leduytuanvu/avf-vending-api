-- Mark stale non-terminal orders as unknown (TTL reconciliation). Review dry_run first.
-- Usage:
--   psql "$DATABASE_URL" -v dry_run=1 -v stale_minutes=45 -v machine_code='AVF000109' \
--     -f scripts/ops/mark_stale_non_terminal_orders_unknown.sql
\set ON_ERROR_STOP on

\if :{?dry_run}
\else
\set dry_run 1
\endif

\if :{?stale_minutes}
\else
\set stale_minutes 45
\endif

\if :{?machine_code}
\else
\set machine_code ''
\endif

\echo '=== Candidates (paid/vending/created with capture, older than stale_minutes) ==='
SELECT o.id,
       o.status,
       m.code AS machine_code,
       o.updated_at,
       now() - o.updated_at AS age
FROM orders o
JOIN machines m ON m.id = o.machine_id
LEFT JOIN LATERAL (
    SELECT p.state
    FROM payments p
    WHERE p.order_id = o.id
    ORDER BY p.created_at DESC
    LIMIT 1
) pay ON TRUE
WHERE o.status IN ('paid', 'vending', 'created', 'quoted')
  AND o.updated_at < now() - (:stale_minutes || ' minutes')::interval
  AND (
    :'machine_code' = ''
    OR m.code = :'machine_code'
  )
  AND (
    o.status IN ('paid', 'vending')
    OR pay.state IN ('captured', 'partially_refunded', 'authorized')
  )
ORDER BY o.updated_at;

\if :dry_run
\echo 'dry_run=1 — no updates applied'
\else
UPDATE orders o
SET status = 'unknown',
    updated_at = now()
FROM machines m
WHERE m.id = o.machine_id
  AND o.status IN ('paid', 'vending', 'created', 'quoted')
  AND o.updated_at < now() - (:stale_minutes || ' minutes')::interval
  AND (
    :'machine_code' = ''
    OR m.code = :'machine_code'
  );
\endif
