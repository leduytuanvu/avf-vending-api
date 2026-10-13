-- Alert-only: orders stuck in paid/vending (no auto-complete). Pair with field evidence before repair template.
--
-- Usage:
--   psql "$DATABASE_URL" -v stale_minutes=15 -v machine_code='AVF000109' \
--     -f scripts/ops/monitor_stale_paid_vending_orders.sql

\set ON_ERROR_STOP on

\if :{?stale_minutes}
\else
\set stale_minutes 15
\endif

\if :{?machine_code}
\else
\set machine_code ''
\endif

SELECT o.id,
       o.status,
       o.total_minor,
       m.code AS machine_code,
       o.updated_at,
       extract(epoch FROM (now() - o.updated_at)) / 60.0 AS minutes_since_update,
       array_agg(DISTINCT vs.state ORDER BY vs.state) AS vend_line_states
FROM orders o
JOIN machines m ON m.id = o.machine_id
LEFT JOIN vend_sessions vs ON vs.order_id = o.id
WHERE o.status IN ('paid', 'vending')
  AND o.updated_at < now() - (:'stale_minutes' || ' minutes')::interval
  AND (:'machine_code' = '' OR m.code = :'machine_code')
GROUP BY o.id, o.status, o.total_minor, m.code, o.updated_at
ORDER BY o.updated_at;
