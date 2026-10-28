-- Read-only forensics: AVF000195 local session 2026-10-29 02:28:59+ +07
-- (2026-10-28 19:28:59+ UTC). Hardware had 10k nap + order 1 B1 cash vend OK + QR 20k cart.
-- Do not INSERT cash_acceptance_events. Do not APPLY repairs from this file.
--
-- Local order ids from kiosk log:
--   be6f4c3d-45d8-43e7-b8d5-52b6971a37e0  B1 10k cash wallet vend OK
--   916cbfaf-6ab8-47b4-81fc-7780ab84f03e  QR MoMo A10+B1 20k (session 01a183fd-...)
-- Cash token: 040204 (10k recycler billType=0x04)
-- Sale outbox: 6b347e0e-5421-4137-9f13-f2d6a7cc308e
-- Vend success: 80590486-0907-4e9a-b191-d7205a28bd24
--
-- Usage:
--   psql "$DATABASE_URL" -f scripts/ops/verify_session_20261029_0228_cash_orders.sql

\set ON_ERROR_STOP on

\if :{?machine_id}
\else
\set machine_id '01a0a7e5-3c68-7895-b526-bcb6504bccfb'
\endif
\if :{?window_from}
\else
\set window_from '2026-10-28T19:25:00+00'
\endif
\if :{?window_to}
\else
\set window_to '2026-10-28T19:40:00+00'
\endif

\echo '=== Session 02:28 verify ==='
\echo 'machine_id=' :machine_id
\echo 'from=' :window_from ' to=' :window_to

\echo ''
\echo '=== Orders (local client UUIDs from kiosk) ==='
SELECT o.id,
       o.status,
       o.total_minor,
       o.winning_payment_id,
       m.code AS machine_code,
       o.created_at,
       array_agg(vs.line_sequence ORDER BY vs.line_sequence) AS line_sequences,
       array_agg(vs.slot_index ORDER BY vs.line_sequence) AS slot_indexes,
       array_agg(vs.state ORDER BY vs.line_sequence) AS line_states
FROM orders o
JOIN machines m ON m.id = o.machine_id
LEFT JOIN vend_sessions vs ON vs.order_id = o.id
WHERE o.id IN (
    'be6f4c3d-45d8-43e7-b8d5-52b6971a37e0'::uuid,
    '916cbfaf-6ab8-47b4-81fc-7780ab84f03e'::uuid
)
GROUP BY o.id, o.status, o.total_minor, o.winning_payment_id, m.code, o.created_at
ORDER BY o.created_at;

\echo ''
\echo '=== Machine orders in window (may use server UUIDs) ==='
SELECT o.id,
       o.status,
       o.total_minor,
       o.created_at,
       array_agg(vs.state ORDER BY vs.line_sequence) AS line_states,
       array_agg(vs.slot_index ORDER BY vs.line_sequence) AS slot_indexes
FROM orders o
LEFT JOIN vend_sessions vs ON vs.order_id = o.id
WHERE o.machine_id = :'machine_id'::uuid
  AND o.created_at >= :'window_from'::timestamptz
  AND o.created_at < :'window_to'::timestamptz
GROUP BY o.id, o.status, o.total_minor, o.created_at
ORDER BY o.created_at;

\echo ''
\echo '=== Payments for local order ids + MoMo session ==='
SELECT p.order_id, p.id, p.provider, p.state, p.outcome, p.amount_minor, p.idempotency_key
FROM payments p
WHERE p.order_id IN (
    'be6f4c3d-45d8-43e7-b8d5-52b6971a37e0'::uuid,
    '916cbfaf-6ab8-47b4-81fc-7780ab84f03e'::uuid
)
   OR p.idempotency_key LIKE '%01a183fd-3175-7881-8187-78bdae6f786a%'
   OR p.idempotency_key LIKE '%be6f4c3d%'
   OR p.idempotency_key LIKE '%916cbfaf%'
ORDER BY p.created_at;

\echo ''
\echo '=== Offline events for sale/vend keys ==='
SELECT offline_sequence,
       event_type,
       idempotency_key,
       client_event_id,
       processing_status,
       left(processing_error, 180) AS processing_error,
       occurred_at
FROM machine_offline_events
WHERE machine_id = :'machine_id'::uuid
  AND (
      idempotency_key LIKE 'offline-sale:be6f4c3d%'
      OR idempotency_key LIKE 'offline-sale:916cbfaf%'
      OR idempotency_key LIKE 'offline-vend-%be6f4c3d%'
      OR idempotency_key LIKE 'offline-vend-%916cbfaf%'
      OR client_event_id IN (
          'be6f4c3d-45d8-43e7-b8d5-52b6971a37e0',
          '916cbfaf-6ab8-47b4-81fc-7780ab84f03e',
          '6b347e0e-5421-4137-9f13-f2d6a7cc308e',
          '80590486-0907-4e9a-b191-d7205a28bd24'
      )
  )
ORDER BY offline_sequence;

\echo ''
\echo '=== Offline cash_movement events (session token 040204) ==='
SELECT offline_sequence,
       event_type,
       idempotency_key,
       client_event_id,
       processing_status,
       left(processing_error, 180) AS processing_error,
       occurred_at
FROM machine_offline_events
WHERE machine_id = :'machine_id'::uuid
  AND idempotency_key LIKE 'cash_movement:%'
  AND (
      idempotency_key LIKE '%040204%'
      OR client_event_id LIKE '%040204%'
  )
ORDER BY offline_sequence;

\echo ''
\echo '=== machine_sync_cursors (legacy + cash + commerce) ==='
SELECT stream_name,
       last_sequence,
       last_synced_at,
       updated_at
FROM machine_sync_cursors
WHERE machine_id = :'machine_id'::uuid
ORDER BY stream_name;

\echo ''
\echo '=== machine_idempotency_keys cash_movement poison vs ingest ==='
SELECT mik.operation,
       mik.idempotency_key,
       mik.status,
       mik.last_seen_at,
       encode(mik.request_hash, 'hex') AS request_hash_hex
FROM machine_idempotency_keys mik
WHERE mik.machine_id = :'machine_id'::uuid
  AND mik.idempotency_key LIKE 'cash_movement:%'
  AND mik.idempotency_key LIKE '%040204%'
ORDER BY mik.last_seen_at;

\echo ''
\echo '=== Bill credits (cash_acceptance_events) in window ==='
SELECT device_event_id,
       denomination_minor,
       credit_source,
       raw_metadata->>'raw_record_hex' AS raw_record_hex,
       COALESCE(occurred_at_device, accepted_at) AS occurred_at_device,
       accepted_at
FROM cash_acceptance_events
WHERE machine_id = :'machine_id'::uuid
  AND accepted_at >= :'window_from'::timestamptz
  AND accepted_at < :'window_to'::timestamptz
ORDER BY accepted_at;

\echo ''
\echo '=== Session token 040204 missing cash_acceptance_events ==='
SELECT tok AS token
FROM (VALUES ('040204')) t(tok)
WHERE NOT EXISTS (
    SELECT 1
    FROM cash_acceptance_events cae
    WHERE cae.machine_id = :'machine_id'::uuid
      AND (
          cae.device_event_id ILIKE '%' || tok || '%'
          OR COALESCE(cae.raw_metadata->>'raw_record_hex', '') ILIKE '%' || tok || '%'
      )
);

\echo ''
\echo '=== Payout events in window ==='
SELECT device_event_id,
       event_type,
       denomination_minor,
       amount_minor,
       note_sequence,
       withdrawal_id,
       occurred_at_device
FROM cash_payout_events
WHERE machine_id = :'machine_id'::uuid
  AND occurred_at_device >= :'window_from'::timestamptz
  AND occurred_at_device < :'window_to'::timestamptz
ORDER BY note_sequence, occurred_at_device;

\echo ''
\echo '=== Offline events in window (all sequenced) ==='
SELECT offline_sequence,
       event_type,
       left(idempotency_key, 80) AS idempotency_key,
       client_event_id,
       processing_status,
       occurred_at
FROM machine_offline_events
WHERE machine_id = :'machine_id'::uuid
  AND occurred_at >= :'window_from'::timestamptz
  AND occurred_at < :'window_to'::timestamptz
ORDER BY offline_sequence;
