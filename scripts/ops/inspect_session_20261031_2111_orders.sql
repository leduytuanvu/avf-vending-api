-- Read-only crosscheck: cash session 2026-10-31 21:11–21:13 VN on AVF000195.
-- Usage:
--   psql "$DATABASE_URL" -f scripts/ops/inspect_session_20261031_2111_orders.sql

\set ON_ERROR_STOP on

\echo '=== Named orders (21:11 session + leftover 20:45 seq913) ==='
SELECT o.id,
       o.status,
       o.total_minor,
       m.code AS machine_code,
       o.created_at AT TIME ZONE 'Asia/Ho_Chi_Minh' AS created_at_vn,
       p.state AS payment_state,
       p.provider AS payment_provider,
       p.amount_minor AS payment_amount_minor,
       array_agg(vs.line_sequence ORDER BY vs.line_sequence) FILTER (WHERE vs.id IS NOT NULL) AS line_sequences,
       array_agg(vs.slot_index ORDER BY vs.line_sequence) FILTER (WHERE vs.id IS NOT NULL) AS slot_indexes,
       array_agg(vs.state ORDER BY vs.line_sequence) FILTER (WHERE vs.id IS NOT NULL) AS line_states,
       array_agg(vs.failure_reason ORDER BY vs.line_sequence) FILTER (WHERE vs.id IS NOT NULL) AS line_failures,
       CASE
           WHEN o.id = '57c70b3b-8da9-4cd1-8dc8-c9824cc635e2'::uuid THEN 'order1_completed_b10'
           WHEN o.id = '01a1924d-d4b7-7280-b8f6-673df1763f31'::uuid THEN 'order2_failed_b7'
           WHEN o.id = '01a1924e-8077-7524-8363-7788be781571'::uuid THEN 'order3_partial_c6_c2'
           WHEN o.id = '01a19235-7794-7793-addd-e82ebb8450c9'::uuid THEN 'leftover_seq913_2045'
           ELSE 'other'
       END AS session_label,
       CASE
           WHEN o.status IN ('paid', 'vending', 'created', 'quoted') THEN 'REVIEW_NON_TERMINAL'
           WHEN o.status IN ('completed', 'partially_completed', 'failed', 'cancelled') THEN 'OK_TERMINAL'
           ELSE 'REVIEW'
       END AS admin_expectation
FROM orders o
JOIN machines m ON m.id = o.machine_id
LEFT JOIN payments p ON p.order_id = o.id
LEFT JOIN vend_sessions vs ON vs.order_id = o.id
WHERE o.id IN (
    '57c70b3b-8da9-4cd1-8dc8-c9824cc635e2'::uuid,
    '01a1924d-d4b7-7280-b8f6-673df1763f31'::uuid,
    '01a1924e-8077-7524-8363-7788be781571'::uuid,
    '01a19235-7794-7793-addd-e82ebb8450c9'::uuid
)
GROUP BY o.id, o.status, o.total_minor, m.code, o.created_at, p.state, p.provider, p.amount_minor
ORDER BY o.created_at;

\echo '=== Payments for named orders ==='
SELECT p.order_id,
       p.id AS payment_id,
       p.provider,
       p.state AS payment_state,
       p.outcome,
       p.amount_minor,
       p.created_at AT TIME ZONE 'Asia/Ho_Chi_Minh' AS created_at_vn
FROM payments p
WHERE p.order_id IN (
    '57c70b3b-8da9-4cd1-8dc8-c9824cc635e2'::uuid,
    '01a1924d-d4b7-7280-b8f6-673df1763f31'::uuid,
    '01a1924e-8077-7524-8363-7788be781571'::uuid,
    '01a19235-7794-7793-addd-e82ebb8450c9'::uuid
)
ORDER BY p.created_at;

\echo '=== Vend sessions for named orders ==='
SELECT vs.order_id,
       vs.line_sequence,
       vs.slot_index,
       vs.state,
       vs.failure_reason,
       vs.verification_status,
       vs.started_at AT TIME ZONE 'Asia/Ho_Chi_Minh' AS started_at_vn,
       vs.completed_at AT TIME ZONE 'Asia/Ho_Chi_Minh' AS completed_at_vn
FROM vend_sessions vs
WHERE vs.order_id IN (
    '57c70b3b-8da9-4cd1-8dc8-c9824cc635e2'::uuid,
    '01a1924d-d4b7-7280-b8f6-673df1763f31'::uuid,
    '01a1924e-8077-7524-8363-7788be781571'::uuid,
    '01a19235-7794-7793-addd-e82ebb8450c9'::uuid
)
ORDER BY vs.order_id, vs.line_sequence;

\echo '=== Machine window 21:10–21:20 VN (may include extra ids) ==='
SELECT o.id,
       o.status,
       o.total_minor,
       m.code AS machine_code,
       o.created_at AT TIME ZONE 'Asia/Ho_Chi_Minh' AS created_at_vn,
       p.state AS payment_state,
       p.provider AS payment_provider,
       p.amount_minor AS payment_amount_minor,
       array_agg(vs.line_sequence ORDER BY vs.line_sequence) FILTER (WHERE vs.id IS NOT NULL) AS line_sequences,
       array_agg(vs.slot_index ORDER BY vs.line_sequence) FILTER (WHERE vs.id IS NOT NULL) AS slot_indexes,
       array_agg(vs.state ORDER BY vs.line_sequence) FILTER (WHERE vs.id IS NOT NULL) AS line_states
FROM orders o
JOIN machines m ON m.id = o.machine_id
LEFT JOIN payments p ON p.order_id = o.id
LEFT JOIN vend_sessions vs ON vs.order_id = o.id
WHERE o.machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND o.created_at >= '2026-10-31 14:10:00+00'::timestamptz
  AND o.created_at < '2026-10-31 14:20:00+00'::timestamptz
GROUP BY o.id, o.status, o.total_minor, m.code, o.created_at, p.state, p.provider, p.amount_minor
ORDER BY o.created_at;

\echo '=== Non-terminal leftover subset ==='
SELECT o.id, o.status, m.code AS machine_code, p.state AS payment_state, p.provider
FROM orders o
JOIN machines m ON m.id = o.machine_id
LEFT JOIN payments p ON p.order_id = o.id
WHERE o.id IN (
    '57c70b3b-8da9-4cd1-8dc8-c9824cc635e2'::uuid,
    '01a1924d-d4b7-7280-b8f6-673df1763f31'::uuid,
    '01a1924e-8077-7524-8363-7788be781571'::uuid,
    '01a19235-7794-7793-addd-e82ebb8450c9'::uuid
)
  AND o.status IN ('created', 'quoted', 'paid', 'vending')
ORDER BY o.created_at;
