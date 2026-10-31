-- Read-only crosscheck: offline cash session 2026-11-01 00:49–00:53 VN on AVF000195.
-- Usage:
--   psql "$DATABASE_URL" -f scripts/ops/inspect_session_20261101_0049_orders.sql

\set ON_ERROR_STOP on

\echo '=== Named orders (00:49 session) ==='
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
           WHEN o.id = '28c7b612-89b1-4271-9188-688f7514d1b0'::uuid THEN 'order1_failed_a7'
           WHEN o.id = '40f524c9-f016-4634-af87-acbfb6404a10'::uuid THEN 'order2_success_a8'
           WHEN o.id = 'd1ad297d-47e7-459d-a068-ede2727fd633'::uuid THEN 'order3_partial_b10_b9'
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
    '28c7b612-89b1-4271-9188-688f7514d1b0'::uuid,
    '40f524c9-f016-4634-af87-acbfb6404a10'::uuid,
    'd1ad297d-47e7-459d-a068-ede2727fd633'::uuid
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
    '28c7b612-89b1-4271-9188-688f7514d1b0'::uuid,
    '40f524c9-f016-4634-af87-acbfb6404a10'::uuid,
    'd1ad297d-47e7-459d-a068-ede2727fd633'::uuid
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
    '28c7b612-89b1-4271-9188-688f7514d1b0'::uuid,
    '40f524c9-f016-4634-af87-acbfb6404a10'::uuid,
    'd1ad297d-47e7-459d-a068-ede2727fd633'::uuid
)
ORDER BY vs.order_id, vs.line_sequence;

\echo '=== Machine window 00:49–00:53 VN ==='
SELECT o.id,
       o.status,
       o.total_minor,
       m.code AS machine_code,
       o.created_at AT TIME ZONE 'Asia/Ho_Chi_Minh' AS created_at_vn,
       p.state AS payment_state,
       p.provider AS payment_provider,
       p.amount_minor AS payment_amount_minor
FROM orders o
JOIN machines m ON m.id = o.machine_id
LEFT JOIN payments p ON p.order_id = o.id
WHERE o.machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND o.created_at >= '2026-10-31 17:49:00+00'::timestamptz
  AND o.created_at < '2026-10-31 17:53:00+00'::timestamptz
ORDER BY o.created_at;

\echo '=== Cash acceptance 620204 / 640103 / 660103 / 680105 ==='
SELECT cae.device_event_id,
       cae.denomination_minor,
       cae.credit_source,
       cae.accepted_at AT TIME ZONE 'Asia/Ho_Chi_Minh' AS accepted_at_vn,
       cae.order_id
FROM cash_acceptance_events cae
WHERE cae.machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND (
      cae.device_event_id LIKE '%620204%'
      OR cae.device_event_id LIKE '%640103%'
      OR cae.device_event_id LIKE '%660103%'
      OR cae.device_event_id LIKE '%680105%'
  )
ORDER BY cae.accepted_at;

\echo '=== Bill lifecycle escrow_held 610004 / 630003 / 650003 / 670005 ==='
SELECT cbl.device_event_id,
       cbl.lifecycle_type,
       cbl.denomination_minor,
       cbl.occurred_at_device AT TIME ZONE 'Asia/Ho_Chi_Minh' AS occurred_at_vn
FROM cash_bill_lifecycle_events cbl
WHERE cbl.machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND (
      cbl.device_event_id LIKE '%610004%'
      OR cbl.device_event_id LIKE '%630003%'
      OR cbl.device_event_id LIKE '%650003%'
      OR cbl.device_event_id LIKE '%670005%'
  )
ORDER BY cbl.occurred_at_device;

\echo '=== Offline sequence occupants 1123-1141 ==='
SELECT moe.offline_sequence,
       moe.event_type,
       moe.idempotency_key,
       moe.client_event_id,
       moe.processing_status,
       left(moe.processing_error, 180) AS processing_error,
       moe.occurred_at AT TIME ZONE 'Asia/Ho_Chi_Minh' AS occurred_at_vn
FROM machine_offline_events moe
WHERE moe.machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND moe.offline_sequence BETWEEN 1123 AND 1141
ORDER BY moe.offline_sequence;

\echo '=== Slot occupants 1125 / 1132 / 1133 / 1137 ==='
SELECT moe.offline_sequence,
       moe.event_type,
       moe.idempotency_key,
       moe.client_event_id,
       moe.processing_status
FROM machine_offline_events moe
WHERE moe.machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND moe.offline_sequence IN (1125, 1132, 1133, 1134, 1137, 1138, 1139, 1140, 1141)
ORDER BY moe.offline_sequence;

\echo '=== Non-terminal leftover subset ==='
SELECT o.id, o.status, m.code AS machine_code, p.state AS payment_state, p.provider
FROM orders o
JOIN machines m ON m.id = o.machine_id
LEFT JOIN payments p ON p.order_id = o.id
WHERE o.id IN (
    '28c7b612-89b1-4271-9188-688f7514d1b0'::uuid,
    '40f524c9-f016-4634-af87-acbfb6404a10'::uuid,
    'd1ad297d-47e7-459d-a068-ede2727fd633'::uuid
)
  AND o.status IN ('created', 'quoted', 'paid', 'vending')
ORDER BY o.created_at;
