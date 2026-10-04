-- One-time ops: after API fix for offline client_created_at backfill, let AVF000195 replay seq 38+.
-- Run read-only verify first: scripts/ops/verify_avf000195_offline_replay.sql
\set ON_ERROR_STOP on

\echo '=== before ==='
SELECT machine_id, stream_name, last_sequence FROM machine_sync_cursors
WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid AND stream_name = 'offline';

SELECT offline_sequence, processing_status, left(processing_error, 120) AS processing_error
FROM machine_offline_events
WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND offline_sequence BETWEEN 38 AND 42
ORDER BY offline_sequence;

BEGIN;

UPDATE machine_sync_cursors
SET last_sequence = 37, updated_at = now()
WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND stream_name = 'offline'
  AND last_sequence > 37;

UPDATE machine_offline_events
SET processing_status = 'processing', processing_error = ''
WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND offline_sequence >= 38
  AND processing_status = 'rejected';

COMMIT;

\echo '=== after ==='
SELECT machine_id, stream_name, last_sequence FROM machine_sync_cursors
WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid AND stream_name = 'offline';
