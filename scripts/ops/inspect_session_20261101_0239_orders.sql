-- Read-only crosscheck: cash session 2026-11-01 02:39–02:45 VN on AVF000195.
-- Usage:
--   psql "$DATABASE_URL" -f scripts/ops/inspect_session_20261101_0239_orders.sql

\set ON_ERROR_STOP on

\echo '=== Named orders (02:39 session) ==='
SELECT o.id,
       o.status,
       o.total_minor,
       o.pricing_source,
       m.code AS machine_code,
       o.created_at AT TIME ZONE 'Asia/Ho_Chi_Minh' AS created_at_vn,
       o.updated_at AT TIME ZONE 'Asia/Ho_Chi_Minh' AS updated_at_vn,
       p.state AS payment_state,
       p.provider AS payment_provider,
       p.amount_minor AS payment_amount_minor,
       array_agg(vs.line_sequence ORDER BY vs.line_sequence) FILTER (WHERE vs.id IS NOT NULL) AS line_sequences,
       array_agg(vs.slot_index ORDER BY vs.line_sequence) FILTER (WHERE vs.id IS NOT NULL) AS slot_indexes,
       array_agg(vs.state ORDER BY vs.line_sequence) FILTER (WHERE vs.id IS NOT NULL) AS line_states,
       array_agg(vs.failure_reason ORDER BY vs.line_sequence) FILTER (WHERE vs.id IS NOT NULL) AS line_failures,
       CASE
           WHEN o.id = '2b277cf2-77c6-4b41-bda6-9d7a7e6e4936'::uuid THEN 'order1_failed_a2_5k'
           WHEN o.id = '48670d49-a474-497f-872c-2ba01c02495a'::uuid THEN 'order2_success_a5_5k'
           WHEN o.id = '5aa55ba0-7ac9-4c0b-a366-e8b2abf1f42a'::uuid THEN 'order3_client_qr_cash_15k'
           WHEN o.id = '01a1937a-a390-7c83-93b8-b3b409c93463'::uuid THEN 'order3_server_alias_15k'
           WHEN o.id = '01a1937a-f498-75d5-8a1d-fb30f862f50c'::uuid THEN 'order3_fulfillment_partial_b7_b5'
           WHEN o.id = '71d497e3-c0e9-4d9e-a2b5-1555e9dd106e'::uuid THEN 'order4_success_b6_15k'
           WHEN o.id = 'ff97b6ab-8d85-4f38-afc3-c46513f6292a'::uuid THEN 'order5_success_b9_10k'
           WHEN o.id = 'de723ee4-effd-4102-918d-ccb416b2aac6'::uuid THEN 'order6_failed_c7_10k'
           WHEN o.id = 'b8239908-2466-40be-bca0-0fea72765d7d'::uuid THEN 'order7_partial_c5_c6_20k'
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
    '2b277cf2-77c6-4b41-bda6-9d7a7e6e4936'::uuid,
    '48670d49-a474-497f-872c-2ba01c02495a'::uuid,
    '5aa55ba0-7ac9-4c0b-a366-e8b2abf1f42a'::uuid,
    '01a1937a-a390-7c83-93b8-b3b409c93463'::uuid,
    '01a1937a-f498-75d5-8a1d-fb30f862f50c'::uuid,
    '71d497e3-c0e9-4d9e-a2b5-1555e9dd106e'::uuid,
    'ff97b6ab-8d85-4f38-afc3-c46513f6292a'::uuid,
    'de723ee4-effd-4102-918d-ccb416b2aac6'::uuid,
    'b8239908-2466-40be-bca0-0fea72765d7d'::uuid
)
GROUP BY o.id, o.status, o.total_minor, o.pricing_source, m.code, o.created_at, o.updated_at, p.state, p.provider, p.amount_minor
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
    '2b277cf2-77c6-4b41-bda6-9d7a7e6e4936'::uuid,
    '48670d49-a474-497f-872c-2ba01c02495a'::uuid,
    '5aa55ba0-7ac9-4c0b-a366-e8b2abf1f42a'::uuid,
    '01a1937a-a390-7c83-93b8-b3b409c93463'::uuid,
    '01a1937a-f498-75d5-8a1d-fb30f862f50c'::uuid,
    '71d497e3-c0e9-4d9e-a2b5-1555e9dd106e'::uuid,
    'ff97b6ab-8d85-4f38-afc3-c46513f6292a'::uuid,
    'de723ee4-effd-4102-918d-ccb416b2aac6'::uuid,
    'b8239908-2466-40be-bca0-0fea72765d7d'::uuid
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
    '2b277cf2-77c6-4b41-bda6-9d7a7e6e4936'::uuid,
    '48670d49-a474-497f-872c-2ba01c02495a'::uuid,
    '5aa55ba0-7ac9-4c0b-a366-e8b2abf1f42a'::uuid,
    '01a1937a-a390-7c83-93b8-b3b409c93463'::uuid,
    '01a1937a-f498-75d5-8a1d-fb30f862f50c'::uuid,
    '71d497e3-c0e9-4d9e-a2b5-1555e9dd106e'::uuid,
    'ff97b6ab-8d85-4f38-afc3-c46513f6292a'::uuid,
    'de723ee4-effd-4102-918d-ccb416b2aac6'::uuid,
    'b8239908-2466-40be-bca0-0fea72765d7d'::uuid
)
ORDER BY vs.order_id, vs.line_sequence;

\echo '=== Machine window 02:39–02:50 VN ==='
SELECT o.id,
       o.status,
       o.total_minor,
       o.pricing_source,
       m.code AS machine_code,
       o.created_at AT TIME ZONE 'Asia/Ho_Chi_Minh' AS created_at_vn,
       p.state AS payment_state,
       p.provider AS payment_provider,
       p.amount_minor AS payment_amount_minor
FROM orders o
JOIN machines m ON m.id = o.machine_id
LEFT JOIN payments p ON p.order_id = o.id
WHERE o.machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND o.created_at >= '2026-10-31 19:39:00+00'::timestamptz
  AND o.created_at < '2026-10-31 19:50:00+00'::timestamptz
ORDER BY o.created_at;

\echo '=== Cash acceptance 760103 / 7A0104 / 7C0104 / 800104 / 820104 / 840105 ==='
SELECT cae.device_event_id,
       cae.denomination_minor,
       cae.credit_source,
       cae.accepted_at AT TIME ZONE 'Asia/Ho_Chi_Minh' AS accepted_at_vn,
       cae.order_id
FROM cash_acceptance_events cae
WHERE cae.machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND (
      cae.device_event_id LIKE '%760103%'
      OR cae.device_event_id LIKE '%7A0104%'
      OR cae.device_event_id LIKE '%7C0104%'
      OR cae.device_event_id LIKE '%800104%'
      OR cae.device_event_id LIKE '%820104%'
      OR cae.device_event_id LIKE '%840105%'
  )
ORDER BY cae.accepted_at;

\echo '=== Bill lifecycle escrow_held 750003 / 790004 / 7B0004 / 7F0004 / 810004 / 830005 ==='
SELECT cbl.device_event_id,
       cbl.lifecycle_type,
       cbl.denomination_minor,
       cbl.occurred_at_device AT TIME ZONE 'Asia/Ho_Chi_Minh' AS occurred_at_vn
FROM cash_bill_lifecycle_events cbl
WHERE cbl.machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND (
      cbl.device_event_id LIKE '%750003%'
      OR cbl.device_event_id LIKE '%790004%'
      OR cbl.device_event_id LIKE '%7B0004%'
      OR cbl.device_event_id LIKE '%7F0004%'
      OR cbl.device_event_id LIKE '%810004%'
      OR cbl.device_event_id LIKE '%830005%'
  )
ORDER BY cbl.occurred_at_device;

\echo '=== Cash acceptance in window 02:39–02:50 VN ==='
SELECT cae.device_event_id,
       cae.denomination_minor,
       cae.credit_source,
       cae.accepted_at AT TIME ZONE 'Asia/Ho_Chi_Minh' AS accepted_at_vn,
       cae.order_id
FROM cash_acceptance_events cae
WHERE cae.machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND cae.accepted_at >= '2026-10-31 19:39:00+00'::timestamptz
  AND cae.accepted_at < '2026-10-31 19:50:00+00'::timestamptz
ORDER BY cae.accepted_at;

\echo '=== Cash payouts in window (withdrawals) ==='
SELECT cpe.withdrawal_id,
       cpe.device_event_id,
       cpe.event_type,
       cpe.denomination_minor,
       cpe.amount_minor,
       cpe.occurred_at_device AT TIME ZONE 'Asia/Ho_Chi_Minh' AS occurred_at_vn
FROM cash_payout_events cpe
WHERE cpe.machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND cpe.occurred_at_device >= '2026-10-31 19:39:00+00'::timestamptz
  AND cpe.occurred_at_device < '2026-10-31 19:50:00+00'::timestamptz
ORDER BY cpe.occurred_at_device;

\echo '=== Offline sequence occupants 1162-1210 ==='
SELECT moe.offline_sequence,
       moe.event_type,
       moe.idempotency_key,
       moe.client_event_id,
       moe.processing_status,
       left(moe.processing_error, 180) AS processing_error,
       moe.occurred_at AT TIME ZONE 'Asia/Ho_Chi_Minh' AS occurred_at_vn,
       moe.received_at AT TIME ZONE 'Asia/Ho_Chi_Minh' AS received_at_vn
FROM machine_offline_events moe
WHERE moe.machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND moe.offline_sequence BETWEEN 1162 AND 1210
ORDER BY moe.offline_sequence;

\echo '=== Offline events for named orders / bill hex ==='
SELECT moe.offline_sequence,
       moe.event_type,
       moe.processing_status,
       left(moe.processing_error, 160) AS processing_error,
       left(moe.idempotency_key, 140) AS idempotency_key
FROM machine_offline_events moe
WHERE moe.machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND (
       moe.idempotency_key LIKE '%2b277cf2-77c6-4b41-bda6-9d7a7e6e4936%'
    OR moe.idempotency_key LIKE '%48670d49-a474-497f-872c-2ba01c02495a%'
    OR moe.idempotency_key LIKE '%5aa55ba0-7ac9-4c0b-a366-e8b2abf1f42a%'
    OR moe.idempotency_key LIKE '%01a1937a-a390-7c83-93b8-b3b409c93463%'
    OR moe.idempotency_key LIKE '%01a1937a-f498-75d5-8a1d-fb30f862f50c%'
    OR moe.idempotency_key LIKE '%71d497e3-c0e9-4d9e-a2b5-1555e9dd106e%'
    OR moe.idempotency_key LIKE '%ff97b6ab-8d85-4f38-afc3-c46513f6292a%'
    OR moe.idempotency_key LIKE '%de723ee4-effd-4102-918d-ccb416b2aac6%'
    OR moe.idempotency_key LIKE '%b8239908-2466-40be-bca0-0fea72765d7d%'
    OR moe.idempotency_key LIKE '%750003%'
    OR moe.idempotency_key LIKE '%760103%'
    OR moe.idempotency_key LIKE '%790004%'
    OR moe.idempotency_key LIKE '%7A0104%'
    OR moe.idempotency_key LIKE '%7B0004%'
    OR moe.idempotency_key LIKE '%7C0104%'
    OR moe.idempotency_key LIKE '%7F0004%'
    OR moe.idempotency_key LIKE '%800104%'
    OR moe.idempotency_key LIKE '%810004%'
    OR moe.idempotency_key LIKE '%820104%'
    OR moe.idempotency_key LIKE '%830005%'
    OR moe.idempotency_key LIKE '%840105%'
  )
ORDER BY moe.offline_sequence;

\echo '=== Non-terminal leftover subset ==='
SELECT o.id, o.status, m.code AS machine_code, p.state AS payment_state, p.provider
FROM orders o
JOIN machines m ON m.id = o.machine_id
LEFT JOIN payments p ON p.order_id = o.id
WHERE o.id IN (
    '2b277cf2-77c6-4b41-bda6-9d7a7e6e4936'::uuid,
    '48670d49-a474-497f-872c-2ba01c02495a'::uuid,
    '5aa55ba0-7ac9-4c0b-a366-e8b2abf1f42a'::uuid,
    '01a1937a-a390-7c83-93b8-b3b409c93463'::uuid,
    '01a1937a-f498-75d5-8a1d-fb30f862f50c'::uuid,
    '71d497e3-c0e9-4d9e-a2b5-1555e9dd106e'::uuid,
    'ff97b6ab-8d85-4f38-afc3-c46513f6292a'::uuid,
    'de723ee4-effd-4102-918d-ccb416b2aac6'::uuid,
    'b8239908-2466-40be-bca0-0fea72765d7d'::uuid
)
  AND o.status IN ('created', 'quoted', 'paid', 'vending')
ORDER BY o.created_at;
