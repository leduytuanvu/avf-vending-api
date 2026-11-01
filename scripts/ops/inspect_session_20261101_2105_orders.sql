-- Read-only crosscheck: offline cash session 2026-11-01 21:05–21:09 VN on AVF000195.
-- Usage:
--   psql "$DATABASE_URL" -f scripts/ops/inspect_session_20261101_2105_orders.sql

\set ON_ERROR_STOP on

\echo '=== Named orders (21:05 session) ==='
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
           WHEN o.id = 'c2e63c79-bda7-4228-8042-36a2f9ff77ae'::uuid THEN 'order1_success_b2_5k'
           WHEN o.id = '2b35fcdc-0b70-423d-b08a-bd03500457bd'::uuid THEN 'order2_failed_a5_5k'
           WHEN o.id = '04f347fb-69b3-4b26-ad1d-3068a7a4b561'::uuid THEN 'order3_partial_a9_a8_20k'
           WHEN o.id = 'a53e75c6-0888-4255-85cd-13494bec94bd'::uuid THEN 'order4_success_a2_5k'
           WHEN o.id = 'ea25bfe3-102f-4452-8b48-92d0595033e1'::uuid THEN 'order5_failed_b2_5k'
           WHEN o.id = '047b5c31-c0a8-4527-a034-50ecc53cebc2'::uuid THEN 'order6_partial_a6_a2_20k'
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
    'c2e63c79-bda7-4228-8042-36a2f9ff77ae'::uuid,
    '2b35fcdc-0b70-423d-b08a-bd03500457bd'::uuid,
    '04f347fb-69b3-4b26-ad1d-3068a7a4b561'::uuid,
    'a53e75c6-0888-4255-85cd-13494bec94bd'::uuid,
    'ea25bfe3-102f-4452-8b48-92d0595033e1'::uuid,
    '047b5c31-c0a8-4527-a034-50ecc53cebc2'::uuid
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
    'c2e63c79-bda7-4228-8042-36a2f9ff77ae'::uuid,
    '2b35fcdc-0b70-423d-b08a-bd03500457bd'::uuid,
    '04f347fb-69b3-4b26-ad1d-3068a7a4b561'::uuid,
    'a53e75c6-0888-4255-85cd-13494bec94bd'::uuid,
    'ea25bfe3-102f-4452-8b48-92d0595033e1'::uuid,
    '047b5c31-c0a8-4527-a034-50ecc53cebc2'::uuid
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
    'c2e63c79-bda7-4228-8042-36a2f9ff77ae'::uuid,
    '2b35fcdc-0b70-423d-b08a-bd03500457bd'::uuid,
    '04f347fb-69b3-4b26-ad1d-3068a7a4b561'::uuid,
    'a53e75c6-0888-4255-85cd-13494bec94bd'::uuid,
    'ea25bfe3-102f-4452-8b48-92d0595033e1'::uuid,
    '047b5c31-c0a8-4527-a034-50ecc53cebc2'::uuid
)
ORDER BY vs.order_id, vs.line_sequence;

\echo '=== Machine window 21:05–21:12 VN ==='
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
  AND o.created_at >= '2026-11-01 14:05:00+00'::timestamptz
  AND o.created_at < '2026-11-01 14:12:00+00'::timestamptz
ORDER BY o.created_at;

\echo '=== Cash acceptance FE0204 / 010105 / 050204 / 070106 ==='
SELECT cae.device_event_id,
       cae.denomination_minor,
       cae.credit_source,
       cae.accepted_at AT TIME ZONE 'Asia/Ho_Chi_Minh' AS accepted_at_vn,
       cae.order_id
FROM cash_acceptance_events cae
WHERE cae.machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND (
      cae.device_event_id LIKE '%FE0204%'
      OR cae.device_event_id LIKE '%010105%'
      OR cae.device_event_id LIKE '%050204%'
      OR cae.device_event_id LIKE '%070106%'
  )
ORDER BY cae.accepted_at;

\echo '=== Bill lifecycle escrow_held FD0004 / FF0005 / 040004 / 060006 ==='
SELECT cbl.device_event_id,
       cbl.lifecycle_type,
       cbl.denomination_minor,
       cbl.occurred_at_device AT TIME ZONE 'Asia/Ho_Chi_Minh' AS occurred_at_vn
FROM cash_bill_lifecycle_events cbl
WHERE cbl.machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND (
      cbl.device_event_id LIKE '%FD0004%'
      OR cbl.device_event_id LIKE '%FF0005%'
      OR cbl.device_event_id LIKE '%040004%'
      OR cbl.device_event_id LIKE '%060006%'
  )
ORDER BY cbl.occurred_at_device;

\echo '=== Cash acceptance in window 21:05–21:12 VN ==='
SELECT cae.device_event_id,
       cae.denomination_minor,
       cae.credit_source,
       cae.accepted_at AT TIME ZONE 'Asia/Ho_Chi_Minh' AS accepted_at_vn,
       cae.order_id
FROM cash_acceptance_events cae
WHERE cae.machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND cae.accepted_at >= '2026-11-01 14:05:00+00'::timestamptz
  AND cae.accepted_at < '2026-11-01 14:12:00+00'::timestamptz
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
  AND (
      cpe.withdrawal_id = '15744adb-7721-49bf-a950-9846d9dc51cb'
      OR (
          cpe.occurred_at_device >= '2026-11-01 14:05:00+00'::timestamptz
          AND cpe.occurred_at_device < '2026-11-01 14:12:00+00'::timestamptz
      )
  )
ORDER BY cpe.occurred_at_device;

\echo '=== Offline sequence occupants 1584-1615 ==='
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
  AND moe.offline_sequence BETWEEN 1584 AND 1615
ORDER BY moe.offline_sequence;

\echo '=== Offline events for named orders / bill hex / payout ==='
SELECT moe.offline_sequence,
       moe.event_type,
       moe.processing_status,
       left(moe.processing_error, 160) AS processing_error,
       left(moe.idempotency_key, 160) AS idempotency_key
FROM machine_offline_events moe
WHERE moe.machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND (
       moe.idempotency_key LIKE '%c2e63c79-bda7-4228-8042-36a2f9ff77ae%'
    OR moe.idempotency_key LIKE '%2b35fcdc-0b70-423d-b08a-bd03500457bd%'
    OR moe.idempotency_key LIKE '%04f347fb-69b3-4b26-ad1d-3068a7a4b561%'
    OR moe.idempotency_key LIKE '%a53e75c6-0888-4255-85cd-13494bec94bd%'
    OR moe.idempotency_key LIKE '%ea25bfe3-102f-4452-8b48-92d0595033e1%'
    OR moe.idempotency_key LIKE '%047b5c31-c0a8-4527-a034-50ecc53cebc2%'
    OR moe.idempotency_key LIKE '%15744adb-7721-49bf-a950-9846d9dc51cb%'
    OR moe.idempotency_key LIKE '%FD0004%'
    OR moe.idempotency_key LIKE '%FE0204%'
    OR moe.idempotency_key LIKE '%FF0005%'
    OR moe.idempotency_key LIKE '%010105%'
    OR moe.idempotency_key LIKE '%040004%'
    OR moe.idempotency_key LIKE '%050204%'
    OR moe.idempotency_key LIKE '%060006%'
    OR moe.idempotency_key LIKE '%070106%'
  )
ORDER BY moe.offline_sequence;

\echo '=== Non-terminal leftover subset ==='
SELECT o.id, o.status, m.code AS machine_code, p.state AS payment_state, p.provider
FROM orders o
JOIN machines m ON m.id = o.machine_id
LEFT JOIN payments p ON p.order_id = o.id
WHERE o.id IN (
    'c2e63c79-bda7-4228-8042-36a2f9ff77ae'::uuid,
    '2b35fcdc-0b70-423d-b08a-bd03500457bd'::uuid,
    '04f347fb-69b3-4b26-ad1d-3068a7a4b561'::uuid,
    'a53e75c6-0888-4255-85cd-13494bec94bd'::uuid,
    'ea25bfe3-102f-4452-8b48-92d0595033e1'::uuid,
    '047b5c31-c0a8-4527-a034-50ecc53cebc2'::uuid
)
  AND o.status IN ('created', 'quoted', 'paid', 'vending')
ORDER BY o.created_at;
