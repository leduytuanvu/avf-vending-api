-- Read-only crosscheck: offline cash session 2026-10-31 23:32–23:35 VN on AVF000195.
-- Usage:
--   psql "$DATABASE_URL" -f scripts/ops/inspect_session_20261031_2332_orders.sql

\set ON_ERROR_STOP on

\echo '=== Named orders (23:32 session) ==='
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
           WHEN o.id = '70c37cb1-a1d6-40c5-8f93-bbbf9f92b178'::uuid THEN 'order1_completed_a10'
           WHEN o.id = 'f7418887-62e1-46ef-a4bb-af5b4c892f9c'::uuid THEN 'order2_failed_a9'
           WHEN o.id = 'c8229fb6-ec32-4f68-b78b-42ff968adfe6'::uuid THEN 'order3_partial_a10_b2'
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
    '70c37cb1-a1d6-40c5-8f93-bbbf9f92b178'::uuid,
    'f7418887-62e1-46ef-a4bb-af5b4c892f9c'::uuid,
    'c8229fb6-ec32-4f68-b78b-42ff968adfe6'::uuid
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
    '70c37cb1-a1d6-40c5-8f93-bbbf9f92b178'::uuid,
    'f7418887-62e1-46ef-a4bb-af5b4c892f9c'::uuid,
    'c8229fb6-ec32-4f68-b78b-42ff968adfe6'::uuid
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
    '70c37cb1-a1d6-40c5-8f93-bbbf9f92b178'::uuid,
    'f7418887-62e1-46ef-a4bb-af5b4c892f9c'::uuid,
    'c8229fb6-ec32-4f68-b78b-42ff968adfe6'::uuid
)
ORDER BY vs.order_id, vs.line_sequence;

\echo '=== Machine window 23:31–23:36 VN ==='
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
  AND o.created_at >= '2026-10-31 16:31:00+00'::timestamptz
  AND o.created_at < '2026-10-31 16:36:00+00'::timestamptz
ORDER BY o.created_at;

\echo '=== Cash acceptance 540204 / 560204 / 580204 ==='
SELECT cae.device_event_id,
       cae.denomination_minor,
       cae.credit_source,
       cae.accepted_at AT TIME ZONE 'Asia/Ho_Chi_Minh' AS accepted_at_vn,
       cae.order_id
FROM cash_acceptance_events cae
WHERE cae.machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND (
      cae.device_event_id LIKE '%540204%'
      OR cae.device_event_id LIKE '%560204%'
      OR cae.device_event_id LIKE '%580204%'
  )
ORDER BY cae.accepted_at;

\echo '=== Bill lifecycle escrow_held 530004 / 550004 / 570004 ==='
SELECT cbl.device_event_id,
       cbl.lifecycle_type,
       cbl.denomination_minor,
       cbl.occurred_at_device AT TIME ZONE 'Asia/Ho_Chi_Minh' AS occurred_at_vn
FROM cash_bill_lifecycle_events cbl
WHERE cbl.machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND (
      cbl.device_event_id LIKE '%530004%'
      OR cbl.device_event_id LIKE '%550004%'
      OR cbl.device_event_id LIKE '%570004%'
  )
ORDER BY cbl.occurred_at_device;

\echo '=== Payout 30240e21 ==='
SELECT cpe.withdrawal_id,
       cpe.event_type,
       cpe.device_event_id,
       cpe.amount_minor,
       cpe.note_sequence,
       cpe.occurred_at_device AT TIME ZONE 'Asia/Ho_Chi_Minh' AS occurred_at_vn
FROM cash_payout_events cpe
WHERE cpe.machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND cpe.withdrawal_id = '30240e21-8d9b-43e1-8fc8-e5130c929ffb'
ORDER BY cpe.occurred_at_device, cpe.note_sequence, cpe.event_type;

\echo '=== Non-terminal leftover subset ==='
SELECT o.id, o.status, m.code AS machine_code, p.state AS payment_state, p.provider
FROM orders o
JOIN machines m ON m.id = o.machine_id
LEFT JOIN payments p ON p.order_id = o.id
WHERE o.id IN (
    '70c37cb1-a1d6-40c5-8f93-bbbf9f92b178'::uuid,
    'f7418887-62e1-46ef-a4bb-af5b4c892f9c'::uuid,
    'c8229fb6-ec32-4f68-b78b-42ff968adfe6'::uuid
)
  AND o.status IN ('created', 'quoted', 'paid', 'vending')
ORDER BY o.created_at;
