-- Read-only crosscheck: cash session logcat 2026-10-12 00:53–00:57 + stuck admin order 2be0.
--
-- Usage:
--   psql "$DATABASE_URL" -f scripts/ops/inspect_session_20261012_0053_stuck_paid_orders.sql

\set ON_ERROR_STOP on

\echo '=== Stuck admin order (orphan paid shell) ==='
SELECT o.id,
       o.status,
       o.total_minor,
       m.code AS machine_code,
       o.created_at AT TIME ZONE 'Asia/Ho_Chi_Minh' AS created_at_vn,
       p.state AS payment_state,
       p.provider AS payment_provider,
       array_agg(vs.line_sequence ORDER BY vs.line_sequence) AS line_sequences,
       array_agg(vs.state ORDER BY vs.line_sequence) AS line_states,
       CASE
           WHEN o.status IN ('paid', 'vending') THEN 'STUCK_ADMIN'
           WHEN o.status IN ('completed', 'partially_completed', 'failed', 'cancelled') THEN 'OK_TERMINAL'
           ELSE 'REVIEW'
       END AS admin_expectation
FROM orders o
JOIN machines m ON m.id = o.machine_id
LEFT JOIN payments p ON p.order_id = o.id
LEFT JOIN vend_sessions vs ON vs.order_id = o.id
WHERE o.id = '01a12c1c-2be0-7c9c-aaf1-9c97cb374245'::uuid
GROUP BY o.id, o.status, o.total_minor, m.code, o.created_at, p.state, p.provider;

\echo '=== Log session vend orders (expected terminal from device gRPC) ==='
SELECT o.id,
       o.status,
       o.total_minor,
       m.code AS machine_code,
       o.created_at AT TIME ZONE 'Asia/Ho_Chi_Minh' AS created_at_vn,
       array_agg(vs.line_sequence ORDER BY vs.line_sequence) AS line_sequences,
       array_agg(vs.state ORDER BY vs.line_sequence) AS line_states,
       CASE
           WHEN o.status IN ('paid', 'vending') THEN 'STUCK_ADMIN'
           WHEN o.status IN ('completed', 'partially_completed', 'failed', 'cancelled') THEN 'OK_TERMINAL'
           ELSE 'REVIEW'
       END AS admin_expectation
FROM orders o
JOIN machines m ON m.id = o.machine_id
JOIN vend_sessions vs ON vs.order_id = o.id
WHERE o.id IN (
    '01a12c19-bb9b-79b9-9c38-630c19f2a7b7'::uuid,
    '01a12c19-f44d-7d5e-a8df-9c434ba065c1'::uuid,
    '01a12c1a-3ae5-742a-9b00-ceca6cd227f0'::uuid,
    '01a12c1a-a4f4-740d-8d6c-d5c8c1d0d176'::uuid,
    '01a12c1b-2d6c-77df-9950-195b2da60f17'::uuid,
    '01a12c1b-bfea-7980-a89c-f44bd6576e62'::uuid,
    '01a12c1c-22a8-7fd1-b89d-49b868845734'::uuid,
    '01a12c1c-75cb-7dfd-ac17-772813f5569e'::uuid
)
GROUP BY o.id, o.status, o.total_minor, m.code, o.created_at
ORDER BY o.created_at;

\echo '=== Stuck subset (paid / vending) ==='
SELECT o.id, o.status, m.code AS machine_code,
       array_agg(vs.state ORDER BY vs.line_sequence) AS line_states
FROM orders o
JOIN machines m ON m.id = o.machine_id
LEFT JOIN vend_sessions vs ON vs.order_id = o.id
WHERE o.id IN (
    '01a12c1c-2be0-7c9c-aaf1-9c97cb374245'::uuid,
    '01a12c19-bb9b-79b9-9c38-630c19f2a7b7'::uuid,
    '01a12c19-f44d-7d5e-a8df-9c434ba065c1'::uuid,
    '01a12c1a-3ae5-742a-9b00-ceca6cd227f0'::uuid,
    '01a12c1a-a4f4-740d-8d6c-d5c8c1d0d176'::uuid,
    '01a12c1b-2d6c-77df-9950-195b2da60f17'::uuid,
    '01a12c1b-bfea-7980-a89c-f44bd6576e62'::uuid,
    '01a12c1c-22a8-7fd1-b89d-49b868845734'::uuid,
    '01a12c1c-75cb-7dfd-ac17-772813f5569e'::uuid
)
  AND o.status IN ('paid', 'vending')
GROUP BY o.id, o.status, m.code
ORDER BY o.created_at;
