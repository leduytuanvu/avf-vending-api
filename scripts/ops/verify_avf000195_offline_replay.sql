-- Read-only verify AVF000195 offline replay (orders + cursor).
\set ON_ERROR_STOP on

\echo '=== machine_sync_cursors (AVF000195 offline) ==='
SELECT machine_id, stream_name, last_sequence, last_synced_at, updated_at
FROM machine_sync_cursors
WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND stream_name = 'offline';

\echo '=== orders (test offline cash 2026-10-04 ~15:05) ==='
SELECT id, machine_id, status, total_minor, created_at
FROM orders
WHERE id IN (
    'ffc489f5-0bcd-45f3-971a-ce709f7c2056'::uuid,
    '0a6984ee-7139-4d18-b943-9a843da0e6d3'::uuid
)
ORDER BY created_at;

\echo '=== machine_offline_events (payload mentions order id) ==='
SELECT offline_sequence, event_type, processing_status, received_at,
       payload->>'order_id' AS order_id_payload,
       left(idempotency_key, 80) AS idempotency_key
FROM machine_offline_events
WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND (
    payload::text LIKE '%ffc489f5-0bcd-45f3-971a-ce709f7c2056%'
    OR payload::text LIKE '%0a6984ee-7139-4d18-b943-9a843da0e6d3%'
  )
ORDER BY offline_sequence;
