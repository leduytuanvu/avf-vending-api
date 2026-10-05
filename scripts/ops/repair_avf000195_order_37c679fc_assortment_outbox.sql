-- AVF000195: verify assortment + offline replay for order 37c679fc-49b6-4671-99ea-c6e9dbfdf7cc
-- Read-only verification; run repair steps from repair_avf000195_order_37c679fc_assortment_outbox.md
\set ON_ERROR_STOP on

\echo '=== Machine ==='
SELECT id, site_code, serial_number, status
FROM machines
WHERE id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid;

\echo '=== Slot configs A4 / A5 (expect products 01a089d0 / 01a089cf) ==='
SELECT slot_code, product_id, price_minor, is_active
FROM machine_slot_configs
WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND slot_code IN ('A4', 'A5')
ORDER BY slot_code;

\echo '=== Published assortment products for machine (primary binding) ==='
SELECT p.id AS product_id, ai.sort_order, a.id AS assortment_id, a.name AS assortment_name
FROM machines m
INNER JOIN machine_assortment_bindings b ON b.machine_id = m.id AND b.is_primary AND b.valid_to IS NULL
INNER JOIN assortments a ON a.id = b.assortment_id
INNER JOIN assortment_items ai ON ai.assortment_id = a.id
INNER JOIN products p ON p.id = ai.product_id
WHERE m.id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND p.id IN (
    '01a089d0-48cc-7b0b-8f65-18d5c115625a'::uuid,
    '01a089cf-516c-7c02-b57e-152ce8e114ce'::uuid
  );

\echo '=== Order on server (empty until replay succeeds) ==='
SELECT id, status, total_minor, created_at
FROM orders
WHERE id = '37c679fc-49b6-4671-99ea-c6e9dbfdf7cc'::uuid;

\echo '=== Offline ledger (seq around 102 / offline-sale key) ==='
SELECT offline_sequence, event_type, idempotency_key, processing_status,
       left(processing_error, 200) AS processing_error
FROM machine_offline_events
WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND (
    offline_sequence BETWEEN 100 AND 110
    OR idempotency_key LIKE '%37c679fc%'
    OR idempotency_key LIKE '%230004%'
  )
ORDER BY offline_sequence;

\echo '=== Sync cursor ==='
SELECT stream_name, last_sequence, updated_at
FROM machine_sync_cursors
WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND stream_name = 'offline';
