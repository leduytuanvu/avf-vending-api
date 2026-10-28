-- Read-only forensics: AVF000195 local session 2026-10-29 00:52:15–00:53:50 +07
-- (2026-10-28 17:52:15–17:53:50 UTC). Hardware had 3 orders + 20k+10k nap + 10k rut.
-- Do not INSERT cash_acceptance_events. Do not APPLY repairs from this file.
--
-- Local order ids from kiosk log:
--   456c0923-a516-4002-9842-dc69d1fbc7d9  B1 10k success
--   501e15b3-6768-4a84-ac8f-1cfb67df22d1  A9 10k TCN_NO_DROP fail
--   9856a394-570c-4700-85b6-9267772e76b7  A7 ok + B1 no-drop partial
-- Cash tokens: 060005/070105 (20k), 090204 (10k nap)
-- Payout: f446f604-4a75-45b5-96ac-1266108b423a
--
-- Usage:
--   psql "$DATABASE_URL" -f scripts/ops/verify_session_20261029_0052_cash_orders.sql

\set ON_ERROR_STOP on

\if :{?machine_id}
\else
\set machine_id '01a0a7e5-3c68-7895-b526-bcb6504bccfb'
\endif
\if :{?window_from}
\else
\set window_from '2026-10-28T17:50:00+00'
\endif
\if :{?window_to}
\else
\set window_to '2026-10-28T17:55:00+00'
\endif
\if :{?withdrawal_id}
\else
\set withdrawal_id 'f446f604-4a75-45b5-96ac-1266108b423a'
\endif

\echo '=== Session 00:52 verify ==='
\echo 'machine_id=' :machine_id
\echo 'from=' :window_from ' to=' :window_to
\echo 'withdrawal_id=' :withdrawal_id

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
    '456c0923-a516-4002-9842-dc69d1fbc7d9'::uuid,
    '501e15b3-6768-4a84-ac8f-1cfb67df22d1'::uuid,
    '9856a394-570c-4700-85b6-9267772e76b7'::uuid
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
\echo '=== Payments for local order ids ==='
SELECT p.order_id, p.id, p.provider, p.state, p.outcome, p.amount_minor, p.idempotency_key
FROM payments p
WHERE p.order_id IN (
    '456c0923-a516-4002-9842-dc69d1fbc7d9'::uuid,
    '501e15b3-6768-4a84-ac8f-1cfb67df22d1'::uuid,
    '9856a394-570c-4700-85b6-9267772e76b7'::uuid
)
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
      idempotency_key LIKE 'offline-sale:456c0923%'
      OR idempotency_key LIKE 'offline-sale:501e15b3%'
      OR idempotency_key LIKE 'offline-sale:9856a394%'
      OR idempotency_key LIKE 'offline-vend-%456c0923%'
      OR idempotency_key LIKE 'offline-vend-%501e15b3%'
      OR idempotency_key LIKE 'offline-vend-%9856a394%'
  )
ORDER BY offline_sequence;

\echo ''
\echo '=== Offline cash_movement events (session tokens) ==='
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
      idempotency_key LIKE '%060005%'
      OR idempotency_key LIKE '%070105%'
      OR idempotency_key LIKE '%090204%'
      OR idempotency_key LIKE '%f446f604%'
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
  AND (
      mik.idempotency_key LIKE '%060005%'
      OR mik.idempotency_key LIKE '%070105%'
      OR mik.idempotency_key LIKE '%090204%'
      OR mik.idempotency_key LIKE '%f446f604%'
  )
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
\echo '=== Session tokens missing cash_acceptance_events ==='
SELECT tok AS token
FROM (VALUES ('060005'), ('070105'), ('090204')) t(tok)
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
\echo '=== Payout events ==='
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
\echo '=== Payout keys without cash_payout_events ==='
SELECT moe.idempotency_key,
       moe.processing_status,
       moe.offline_sequence
FROM machine_offline_events moe
WHERE moe.machine_id = :'machine_id'::uuid
  AND moe.idempotency_key LIKE '%payout:f446f604%'
  AND NOT EXISTS (
      SELECT 1
      FROM cash_payout_events cpe
      WHERE cpe.machine_id = moe.machine_id
        AND cpe.withdrawal_id = :'withdrawal_id'
  )
ORDER BY moe.offline_sequence;
