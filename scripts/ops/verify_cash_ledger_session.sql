-- Read-only verify: bill credits + payout for a machine session window.
\set machine_id '01a0a7e5-3c68-7895-b526-bcb6504bccfb'
\set window_from '2026-10-08T05:30:00+00'
\set window_to '2026-10-08T06:15:00+00'
\set withdrawal_id 'a1803384-dfb8-4d2d-89ec-7cef515eeb85'

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
