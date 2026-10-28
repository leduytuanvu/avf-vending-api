-- Read-only forensics: AVF000195 local session 2026-10-28 22:38:28–22:40:33 +07
-- (15:38:28–15:40:33 UTC). Hardware had 3 orders + 3x10k nap + 10k rut.
-- Do not INSERT cash_acceptance_events. Do not APPLY repairs from this file.
--
-- Local order ids from kiosk log:
--   d6697ea6-3f55-487f-bad5-6a686ab4d9c5  A10 10k success
--   7809930d-8459-41ee-8e9e-d5f58c54b1a2  A10 10k TCN_NO_DROP fail
--   77265cf2-6363-4829-a262-01909d461216  A5 success + A2 no-drop partial
-- Cash tokens: D10004/D20204, D30004/D40204, D50004/D60204
-- Payout: 818f8baf-08d4-45f2-8270-a2b1b98deb46
--
-- Usage:
--   psql "$DATABASE_URL" -f scripts/ops/verify_session_20261028_2238_cash_orders.sql

\set ON_ERROR_STOP on

\if :{?machine_id}
\else
\set machine_id '01a0a7e5-3c68-7895-b526-bcb6504bccfb'
\endif
\if :{?window_from}
\else
\set window_from '2026-10-28T15:35:00+00'
\endif
\if :{?window_to}
\else
\set window_to '2026-10-28T15:50:00+00'
\endif
\if :{?withdrawal_id}
\else
\set withdrawal_id '818f8baf-08d4-45f2-8270-a2b1b98deb46'
\endif

\echo '=== Session 22:38 verify ==='
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
    'd6697ea6-3f55-487f-bad5-6a686ab4d9c5'::uuid,
    '7809930d-8459-41ee-8e9e-d5f58c54b1a2'::uuid,
    '77265cf2-6363-4829-a262-01909d461216'::uuid
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
    'd6697ea6-3f55-487f-bad5-6a686ab4d9c5'::uuid,
    '7809930d-8459-41ee-8e9e-d5f58c54b1a2'::uuid,
    '77265cf2-6363-4829-a262-01909d461216'::uuid
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
      idempotency_key LIKE 'offline-sale:d6697ea6%'
      OR idempotency_key LIKE 'offline-sale:7809930d%'
      OR idempotency_key LIKE 'offline-sale:77265cf2%'
      OR idempotency_key LIKE 'offline-vend-%d6697ea6%'
      OR idempotency_key LIKE 'offline-vend-%7809930d%'
      OR idempotency_key LIKE 'offline-vend-%77265cf2%'
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
      idempotency_key LIKE '%D10004%'
      OR idempotency_key LIKE '%D20204%'
      OR idempotency_key LIKE '%D30004%'
      OR idempotency_key LIKE '%D40204%'
      OR idempotency_key LIKE '%D50004%'
      OR idempotency_key LIKE '%D60204%'
      OR idempotency_key LIKE '%818f8baf%'
  )
ORDER BY offline_sequence;

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
      mik.idempotency_key LIKE '%D10004%'
      OR mik.idempotency_key LIKE '%D20204%'
      OR mik.idempotency_key LIKE '%D30004%'
      OR mik.idempotency_key LIKE '%D40204%'
      OR mik.idempotency_key LIKE '%D50004%'
      OR mik.idempotency_key LIKE '%D60204%'
      OR mik.idempotency_key LIKE '%818f8baf%'
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
FROM (VALUES ('D10004'), ('D20204'), ('D30004'), ('D40204'), ('D50004'), ('D60204')) t(tok)
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
  AND moe.idempotency_key LIKE '%payout:818f8baf%'
  AND NOT EXISTS (
      SELECT 1
      FROM cash_payout_events cpe
      WHERE cpe.machine_id = moe.machine_id
        AND cpe.withdrawal_id = :'withdrawal_id'
  )
ORDER BY moe.offline_sequence;
