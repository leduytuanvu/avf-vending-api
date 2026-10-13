-- Inspect AVF000109 stuck-order cluster (2026-10-13 morning, partial UUID prefix 01a132).
-- Usage: psql "$DATABASE_URL" -f scripts/ops/inspect_avf000109_01a132_orders.sql
\set ON_ERROR_STOP on

\echo '=== Orders matching 01a132% on AVF000109 ==='
SELECT o.id,
       o.status,
       o.total_minor,
       o.created_at,
       o.updated_at,
       m.code AS machine_code
FROM orders o
JOIN machines m ON m.id = o.machine_id
WHERE m.code = 'AVF000109'
  AND o.id::text LIKE '01a132%'
ORDER BY o.created_at;

\echo '=== Payments ==='
SELECT p.order_id,
       p.provider,
       p.state AS payment_state,
       p.amount_minor,
       p.created_at
FROM payments p
JOIN orders o ON o.id = p.order_id
JOIN machines m ON m.id = o.machine_id
WHERE m.code = 'AVF000109'
  AND o.id::text LIKE '01a132%'
ORDER BY p.created_at;

\echo '=== Vend sessions ==='
SELECT vs.order_id,
       vs.line_sequence,
       vs.slot_index,
       vs.state,
       vs.failure_reason,
       vs.started_at,
       vs.completed_at
FROM vend_sessions vs
JOIN orders o ON o.id = vs.order_id
JOIN machines m ON m.id = o.machine_id
WHERE m.code = 'AVF000109'
  AND o.id::text LIKE '01a132%'
ORDER BY vs.order_id, vs.line_sequence;
