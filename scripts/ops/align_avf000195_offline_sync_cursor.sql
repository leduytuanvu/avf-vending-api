-- Align machine_sync_cursors for AVF000195 after OUTBOX_SEQUENCE_HOLE (2026-10-04).
-- Device min pending seq=4, server last_sequence=0 → set last_sequence=3 (stream offline).
--
--   psql "$DATABASE_URL" -v dry_run=1 -f scripts/ops/align_avf000195_offline_sync_cursor.sql
--   psql "$DATABASE_URL" -v dry_run=0 -f scripts/ops/align_avf000195_offline_sync_cursor.sql

\set ON_ERROR_STOP on

\echo '=== machine_sync_cursors before (AVF000195) ==='
SELECT machine_id, stream_name, last_sequence, last_synced_at, updated_at
FROM machine_sync_cursors
WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
ORDER BY stream_name;

\if :dry_run
\echo '=== DRY RUN: would set last_sequence=3 for offline stream when last_sequence < 3 ==='
SELECT machine_id,
       stream_name,
       last_sequence AS current_last_sequence,
       3::bigint AS target_last_sequence
FROM machine_sync_cursors
WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND stream_name = 'offline'
  AND last_sequence < 3;
\else
UPDATE machine_sync_cursors
SET last_sequence = 3,
    updated_at = NOW()
WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
  AND stream_name = 'offline'
  AND last_sequence < 3;

\echo '=== machine_sync_cursors after ==='
SELECT machine_id, stream_name, last_sequence, last_synced_at, updated_at
FROM machine_sync_cursors
WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
ORDER BY stream_name;
\endif
