-- AVF000195: remediation for offline insert failed @ seq 37 (order 54a327c2-…)
-- RUN ONLY AFTER verify_avf000195_offline_replay.sql on prod read-only.
-- NEVER bump machine_sync_cursors.last_sequence to 37 while insert still REJECTED on device.
\set ON_ERROR_STOP on

\echo '=== Step 0: current state (re-run verify queries) ==='
SELECT stream_name, last_sequence, last_synced_at, updated_at
FROM machine_sync_cursors
WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND stream_name = 'offline';

SELECT offline_sequence, event_type, client_event_id, idempotency_key,
       processing_status, left(processing_error, 300) AS processing_error
FROM machine_offline_events
WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND offline_sequence = 37;

\echo '=== Decision tree (manual; uncomment ONE block after review) ==='

-- Case A: No row at seq 37, cursor=36 — deploy API logging fix; device retries; no cursor change.

-- Case B: Stale row at seq 37 (wrong event / stuck processing) AND commerce has NO order 54a327c2:
-- Backup row, then delete ONLY if payload is not the target offline sale and ops sign-off:
-- DELETE FROM machine_offline_events
-- WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
--   AND offline_sequence = 37
--   AND idempotency_key <> 'offline-sale:54a327c2-3ecd-4014-b90c-121fed479cd6';

-- Case C: Row seq 37 correct payload but processing_status stuck (non-terminal) — allow device replay;
-- optionally reset status so dispatch re-runs (does NOT advance cursor by itself):
-- UPDATE machine_offline_events
-- SET processing_status = 'pending', processing_error = ''
-- WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
--   AND offline_sequence = 37
--   AND idempotency_key = 'offline-sale:54a327c2-3ecd-4014-b90c-121fed479cd6'
--   AND processing_status NOT IN ('processed', 'succeeded', 'replayed', 'duplicate', 'rejected', 'failed_terminal');

-- Case D: Order 54a327c2 already exists in orders — reconcile device via commerce idempotency API;
-- mark ledger terminal without deleting commerce row:
-- UPDATE machine_offline_events
-- SET processing_status = 'processed', processing_error = ''
-- WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
--   AND offline_sequence = 37
--   AND idempotency_key = 'offline-sale:54a327c2-3ecd-4014-b90c-121fed479cd6';

\echo '=== Post-repair: confirm order + cursor (expect last_sequence >= 37 after successful push) ==='
SELECT id, status, total_minor, created_at
FROM orders
WHERE id = '54a327c2-3ecd-4014-b90c-121fed479cd6'::uuid;
