-- Read-only verify AVF000195 offline replay (orders + cursor).
\set ON_ERROR_STOP on

\echo '=== machine_sync_cursors (AVF000195 offline) ==='
SELECT machine_id, stream_name, last_sequence, last_synced_at, updated_at
FROM machine_sync_cursors
WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND stream_name = 'offline';

\echo '=== orders (test offline cash 2026-10-04 ~15:05 and ~15:45 and ~16:14) ==='
SELECT id, machine_id, status, total_minor, created_at
FROM orders
WHERE id IN (
    'ffc489f5-0bcd-45f3-971a-ce709f7c2056'::uuid,
    '0a6984ee-7139-4d18-b943-9a843da0e6d3'::uuid,
    'caea5197-7295-47f6-b9d8-7d1af2fc1207'::uuid,
    '54a327c2-3ecd-4014-b90c-121fed479cd6'::uuid
)
ORDER BY created_at;

\echo '=== machine_offline_events seq 37 + order 54a327c2 (2026-10-04 ~16:14 incident) ==='
SELECT offline_sequence, event_type, client_event_id, idempotency_key,
       processing_status, left(processing_error, 200) AS processing_error,
       received_at, payload->>'order_id' AS order_id_payload
FROM machine_offline_events
WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND (
    offline_sequence = 37
    OR client_event_id = '54a327c2-3ecd-4014-b90c-121fed479cd6'
    OR idempotency_key = 'offline-sale:54a327c2-3ecd-4014-b90c-121fed479cd6'
    OR payload::text LIKE '%54a327c2-3ecd-4014-b90c-121fed479cd6%'
  )
ORDER BY offline_sequence;

\echo '=== client_event_id collision (same UUID, different offline_sequence) ==='
SELECT offline_sequence, client_event_id, idempotency_key, processing_status
FROM machine_offline_events
WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND client_event_id = '54a327c2-3ecd-4014-b90c-121fed479cd6';

\echo '=== machine_offline_events (payload mentions other incident order ids) ==='
SELECT offline_sequence, event_type, processing_status, received_at,
       payload->>'order_id' AS order_id_payload,
       left(idempotency_key, 80) AS idempotency_key
FROM machine_offline_events
WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND (
    payload::text LIKE '%ffc489f5-0bcd-45f3-971a-ce709f7c2056%'
    OR payload::text LIKE '%0a6984ee-7139-4d18-b943-9a843da0e6d3%'
    OR payload::text LIKE '%caea5197-7295-47f6-b9d8-7d1af2fc1207%'
  )
ORDER BY offline_sequence;

\echo '=== AVF000195 machine row (connectivity hint) ==='
SELECT id, code, status, last_seen_at, updated_at
FROM machines
WHERE id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid;

\echo '=== recent orders for AVF000195 (last 10) ==='
SELECT id, status, total_minor, created_at
FROM orders
WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
ORDER BY created_at DESC
LIMIT 10;

\echo '=== API log hint: scripts/ops/grep_avf000195_offline_api_logs.sh on app-node ==='
