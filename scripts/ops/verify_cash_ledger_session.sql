-- Read-only verify: bill credits + payout for a machine session window.
-- CLI `-v name=value` wins; these \set defaults apply only when a var is unset.
\if :{?machine_id}
\else
\set machine_id '01a0a7e5-3c68-7895-b526-bcb6504bccfb'
\endif
\if :{?window_from}
\else
\set window_from '2026-10-08T05:30:00+00'
\endif
\if :{?window_to}
\else
\set window_to '2026-10-08T06:15:00+00'
\endif
\if :{?withdrawal_id}
\else
\set withdrawal_id 'a1803384-dfb8-4d2d-89ec-7cef515eeb85'
\endif

\echo '=== Session verify ==='
\echo 'machine_id=' :machine_id
\echo 'from=' :window_from ' to=' :window_to
\echo 'withdrawal_id=' :withdrawal_id

\echo ''
\echo '=== Bill credits (cash_acceptance_events) ==='
SELECT device_event_id,
       denomination_minor,
       credit_source,
       raw_metadata->>'raw_record_hex' AS raw_record_hex,
       COALESCE(occurred_at_device, accepted_at) AS occurred_at_device,
       accepted_at,
       created_at
FROM cash_acceptance_events
WHERE machine_id = :'machine_id'::uuid
  AND accepted_at >= :'window_from'::timestamptz
  AND accepted_at < :'window_to'::timestamptz
ORDER BY accepted_at;

\echo ''
\echo '=== Bill credit count (10k minor) ==='
SELECT count(*) AS bill_credit_10k_count
FROM cash_acceptance_events
WHERE machine_id = :'machine_id'::uuid
  AND accepted_at >= :'window_from'::timestamptz
  AND accepted_at < :'window_to'::timestamptz
  AND denomination_minor = 10000;

\echo ''
\echo '=== Payout events (withdrawal) ==='
SELECT device_event_id,
       event_type,
       denomination_minor,
       amount_minor,
       note_sequence,
       occurred_at_device
FROM cash_payout_events
WHERE machine_id = :'machine_id'::uuid
  AND withdrawal_id = :'withdrawal_id'
ORDER BY note_sequence, occurred_at_device;

\echo ''
\echo '=== Payout note count ==='
SELECT count(*) AS payout_event_count
FROM cash_payout_events
WHERE machine_id = :'machine_id'::uuid
  AND withdrawal_id = :'withdrawal_id';

\echo ''
\echo '=== Stuck idempotency (seq101 driver credit key) ==='
SELECT idempotency_key, status, last_seen_at
FROM machine_idempotency_keys
WHERE machine_id = :'machine_id'::uuid
  AND idempotency_key LIKE '%291a2dfd-bb5a-4df4-920a-93812ba45f20%';

\echo ''
\echo '=== Recent bill credits (any time, last 15) ==='
SELECT device_event_id,
       denomination_minor,
       raw_metadata->>'raw_record_hex' AS raw_record_hex,
       accepted_at
FROM cash_acceptance_events
WHERE machine_id = :'machine_id'::uuid
ORDER BY accepted_at DESC
LIMIT 15;

\echo ''
\echo '=== Session terminal hex: idempotency vs acceptance row ==='
WITH hex AS (
  SELECT unnest(ARRAY['2D0004', '2E0204', '2F0004', '300204', '310004']) AS suffix
)
SELECT h.suffix,
       mik.idempotency_key,
       mik.status AS idempotency_status,
       cae.device_event_id AS acceptance_device_event_id,
       cae.accepted_at
FROM hex h
LEFT JOIN machine_idempotency_keys mik
  ON mik.machine_id = :'machine_id'::uuid
 AND mik.idempotency_key LIKE '%' || h.suffix
 AND mik.idempotency_key NOT LIKE '%:payout:%'
LEFT JOIN cash_acceptance_events cae
  ON cae.machine_id = :'machine_id'::uuid
 AND (cae.device_event_id = h.suffix
      OR cae.device_event_id LIKE '%' || h.suffix
      OR cae.raw_metadata->>'raw_record_hex' LIKE '%' || h.suffix)
ORDER BY h.suffix, mik.last_seen_at DESC NULLS LAST;

\echo ''
\echo '=== Idempotency succeeded without acceptance (bill keys, limit 30) ==='
SELECT mik.idempotency_key, mik.status, mik.last_seen_at
FROM machine_idempotency_keys mik
WHERE mik.machine_id = :'machine_id'::uuid
  AND mik.idempotency_key LIKE 'cash_movement:%'
  AND mik.idempotency_key NOT LIKE '%:payout:%'
  AND mik.status IN ('succeeded', 'processed')
  AND NOT EXISTS (
    SELECT 1
    FROM cash_acceptance_events cae
    WHERE cae.machine_id = mik.machine_id
      AND mik.idempotency_key LIKE '%' || cae.device_event_id
  )
ORDER BY mik.last_seen_at DESC
LIMIT 30;

\echo ''
\echo '=== Payout events in time window (all withdrawals) ==='
SELECT withdrawal_id,
       device_event_id,
       event_type,
       note_sequence,
       occurred_at_device
FROM cash_payout_events
WHERE machine_id = :'machine_id'::uuid
  AND occurred_at_device >= :'window_from'::timestamptz
  AND occurred_at_device < :'window_to'::timestamptz
ORDER BY occurred_at_device, note_sequence;

\echo ''
\echo '=== Session 01:37 hex: idempotency vs acceptance (920204 / 940105) ==='
WITH hex AS (
  SELECT unnest(ARRAY['920204', '940105']) AS suffix
)
SELECT h.suffix,
       mik.idempotency_key,
       mik.status AS idempotency_status,
       mik.last_seen_at,
       cae.device_event_id AS acceptance_device_event_id,
       cae.denomination_minor,
       cae.accepted_at
FROM hex h
LEFT JOIN machine_idempotency_keys mik
  ON mik.machine_id = :'machine_id'::uuid
 AND mik.idempotency_key LIKE '%' || h.suffix || '%'
 AND mik.idempotency_key LIKE 'cash_movement:%'
LEFT JOIN cash_acceptance_events cae
  ON cae.machine_id = :'machine_id'::uuid
 AND (
      cae.device_event_id = h.suffix
      OR cae.device_event_id LIKE '%' || h.suffix || '%'
      OR cae.raw_metadata->>'raw_record_hex' LIKE '%' || h.suffix || '%'
 )
ORDER BY h.suffix, mik.last_seen_at DESC NULLS LAST;

\echo ''
\echo '=== Session 01:37 orders ==='
SELECT o.id,
       o.status,
       o.total_minor,
       o.pricing_source,
       o.winning_payment_id,
       o.created_at,
       o.updated_at
FROM orders o
WHERE o.machine_id = :'machine_id'::uuid
  AND o.id IN (
      '770a97e4-1693-4305-aeb7-d91e0d3a94a3'::uuid,
      '01a17ea7-a2c6-7509-a53a-cd8cd519840b'::uuid,
      'b8eb380c-8492-410f-8428-6507f34a0964'::uuid,
      '01a17ea7-8439-7c04-948b-fdda8f0160e6'::uuid,
      '01278dec-edfb-47a5-bd13-15155fa5a886'::uuid
  )
ORDER BY o.created_at;

\echo ''
\echo '=== Session 01:37 payments ==='
SELECT p.order_id,
       p.id AS payment_id,
       p.provider,
       p.state AS payment_state,
       p.outcome,
       p.amount_minor,
       p.created_at,
       p.updated_at
FROM payments p
WHERE p.order_id IN (
      '770a97e4-1693-4305-aeb7-d91e0d3a94a3'::uuid,
      '01a17ea7-a2c6-7509-a53a-cd8cd519840b'::uuid,
      'b8eb380c-8492-410f-8428-6507f34a0964'::uuid,
      '01a17ea7-8439-7c04-948b-fdda8f0160e6'::uuid,
      '01278dec-edfb-47a5-bd13-15155fa5a886'::uuid
  )
ORDER BY p.created_at;

\echo ''
\echo '=== Session 01:37 vend_sessions ==='
SELECT vs.order_id,
       vs.line_sequence,
       vs.slot_index,
       vs.state,
       vs.failure_reason,
       vs.verification_status,
       vs.started_at,
       vs.completed_at
FROM vend_sessions vs
WHERE vs.machine_id = :'machine_id'::uuid
  AND vs.order_id IN (
      '770a97e4-1693-4305-aeb7-d91e0d3a94a3'::uuid,
      '01a17ea7-a2c6-7509-a53a-cd8cd519840b'::uuid,
      'b8eb380c-8492-410f-8428-6507f34a0964'::uuid,
      '01a17ea7-8439-7c04-948b-fdda8f0160e6'::uuid,
      '01278dec-edfb-47a5-bd13-15155fa5a886'::uuid
  )
ORDER BY vs.order_id, vs.line_sequence;

\echo ''
\echo '=== Machine orders created in window ==='
SELECT o.id,
       o.status,
       o.total_minor,
       o.pricing_source,
       o.winning_payment_id,
       o.created_at
FROM orders o
WHERE o.machine_id = :'machine_id'::uuid
  AND o.created_at >= :'window_from'::timestamptz
  AND o.created_at < :'window_to'::timestamptz
ORDER BY o.created_at;

\echo ''
\echo '=== Payments for orders created in window ==='
SELECT p.order_id,
       p.id AS payment_id,
       p.provider,
       p.state AS payment_state,
       p.outcome,
       p.amount_minor,
       p.created_at
FROM payments p
JOIN orders o ON o.id = p.order_id
WHERE o.machine_id = :'machine_id'::uuid
  AND o.created_at >= :'window_from'::timestamptz
  AND o.created_at < :'window_to'::timestamptz
ORDER BY p.created_at;

\echo ''
\echo '=== Vend sessions for orders created in window ==='
SELECT vs.order_id,
       vs.line_sequence,
       vs.slot_index,
       vs.state,
       vs.failure_reason,
       vs.verification_status,
       vs.started_at,
       vs.completed_at
FROM vend_sessions vs
JOIN orders o ON o.id = vs.order_id
WHERE o.machine_id = :'machine_id'::uuid
  AND o.created_at >= :'window_from'::timestamptz
  AND o.created_at < :'window_to'::timestamptz
ORDER BY vs.order_id, vs.line_sequence;

\echo ''
\echo '=== Offline events for session orders / bill hex ==='
SELECT moe.offline_sequence,
       moe.event_type,
       moe.processing_status,
       left(moe.processing_error, 160) AS processing_error,
       left(moe.idempotency_key, 120) AS idempotency_key,
       moe.occurred_at,
       moe.received_at
FROM machine_offline_events moe
WHERE moe.machine_id = :'machine_id'::uuid
  AND (
       moe.idempotency_key LIKE '%770a97e4-1693-4305-aeb7-d91e0d3a94a3%'
    OR moe.idempotency_key LIKE '%01a17ea7-a2c6-7509-a53a-cd8cd519840b%'
    OR moe.idempotency_key LIKE '%b8eb380c-8492-410f-8428-6507f34a0964%'
    OR moe.idempotency_key LIKE '%920204%'
    OR moe.idempotency_key LIKE '%940105%'
    OR moe.payload::text LIKE '%770a97e4-1693-4305-aeb7-d91e0d3a94a3%'
    OR moe.payload::text LIKE '%01a17ea7-a2c6-7509-a53a-cd8cd519840b%'
    OR moe.payload::text LIKE '%b8eb380c-8492-410f-8428-6507f34a0964%'
    OR moe.payload::text LIKE '%920204%'
    OR moe.payload::text LIKE '%940105%'
  )
ORDER BY moe.offline_sequence
LIMIT 80;

\echo ''
\echo '=== Offline cash_movement in window ==='
SELECT moe.offline_sequence,
       moe.processing_status,
       left(moe.processing_error, 160) AS processing_error,
       left(moe.idempotency_key, 120) AS idempotency_key,
       moe.occurred_at,
       moe.received_at
FROM machine_offline_events moe
WHERE moe.machine_id = :'machine_id'::uuid
  AND moe.idempotency_key LIKE 'cash_movement:%'
  AND moe.received_at >= :'window_from'::timestamptz
  AND moe.received_at < :'window_to'::timestamptz
ORDER BY moe.offline_sequence
LIMIT 50;
