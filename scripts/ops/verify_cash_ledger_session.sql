-- Read-only verify: bill credits + payout for a machine session window.
\set machine_id '01a0a7e5-3c68-7895-b526-bcb6504bccfb'
\if :{?machine_id_input}
\set machine_id :'machine_id_input'
\endif
\set window_from '2026-10-08 05:30:00+00'
\if :{?window_from_input}
\set window_from :'window_from_input'
\endif
\set window_to '2026-10-08 06:15:00+00'
\if :{?window_to_input}
\set window_to :'window_to_input'
\endif
\set withdrawal_id 'a1803384-dfb8-4d2d-89ec-7cef515eeb85'
\if :{?withdrawal_id_input}
\set withdrawal_id :'withdrawal_id_input'
\endif

\echo '=== Session verify ==='
\echo 'machine_id=' :machine_id
\echo 'from=' :window_from ' to=' :window_to
\echo 'withdrawal_id=' :withdrawal_id

\echo ''
\echo '=== Bill credits (cash_acceptance_events) ==='
SELECT device_event_id,
       denomination_minor,
       raw_record_hex,
       occurred_at_device,
       created_at
FROM cash_acceptance_events
WHERE machine_id = :'machine_id'::uuid
  AND occurred_at_device >= :'window_from'::timestamptz
  AND occurred_at_device < :'window_to'::timestamptz
ORDER BY occurred_at_device;

\echo ''
\echo '=== Bill credit count (10k minor) ==='
SELECT count(*) AS bill_credit_10k_count
FROM cash_acceptance_events
WHERE machine_id = :'machine_id'::uuid
  AND occurred_at_device >= :'window_from'::timestamptz
  AND occurred_at_device < :'window_to'::timestamptz
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
  AND withdrawal_id = :'withdrawal_id'::uuid
ORDER BY note_sequence, occurred_at_device;

\echo ''
\echo '=== Payout note count ==='
SELECT count(*) AS payout_event_count
FROM cash_payout_events
WHERE machine_id = :'machine_id'::uuid
  AND withdrawal_id = :'withdrawal_id'::uuid;

\echo ''
\echo '=== Stuck idempotency (seq101 driver credit key) ==='
SELECT idempotency_key, status, last_seen_at
FROM machine_idempotency_keys
WHERE machine_id = :'machine_id'::uuid
  AND idempotency_key LIKE '%291a2dfd-bb5a-4df4-920a-93812ba45f20%';
