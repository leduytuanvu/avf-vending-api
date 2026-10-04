-- Align machine_sync_cursors for AVF000195 after OUTBOX_SEQUENCE_HOLE.
-- target_last_sequence = MIN(device pending sequence_no) - 1 from log OUTBOX_SEQUENCE_HOLE minPending.
--
--   psql ... -v dry_run=1 -v target_last_sequence=2 -f scripts/ops/align_avf000195_offline_sync_cursor.sql
--   psql ... -v dry_run=0 -v target_last_sequence=2 -f scripts/ops/align_avf000195_offline_sync_cursor.sql

\set ON_ERROR_STOP on

\if :{?target_last_sequence}
\else
\echo 'error: pass -v target_last_sequence=<minPending-1> from OUTBOX_SEQUENCE_HOLE log'
\quit 1
\endif

\echo '=== machine_sync_cursors before (AVF000195) target_last_sequence=':target_last_sequence' ==='
SELECT machine_id, stream_name, last_sequence, last_synced_at, updated_at
FROM machine_sync_cursors
WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
ORDER BY stream_name;

\if :dry_run
\echo '=== DRY RUN: would upsert offline stream to last_sequence >= ':target_last_sequence' ==='
SELECT '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid AS machine_id,
       'offline'::text AS stream_name,
       COALESCE(
           (SELECT last_sequence
            FROM machine_sync_cursors
            WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
              AND stream_name = 'offline'),
           0
       ) AS current_last_sequence,
       :target_last_sequence::bigint AS target_last_sequence,
       GREATEST(
           COALESCE(
               (SELECT last_sequence
                FROM machine_sync_cursors
                WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
                  AND stream_name = 'offline'),
               0
           ),
           :target_last_sequence::bigint
       ) AS resulting_last_sequence;
\else
INSERT INTO machine_sync_cursors (machine_id, stream_name, last_sequence, last_synced_at, updated_at)
VALUES (
    '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid,
    'offline',
    :target_last_sequence::bigint,
    NOW(),
    NOW()
)
ON CONFLICT (machine_id, stream_name) DO UPDATE
SET last_sequence = GREATEST(machine_sync_cursors.last_sequence, EXCLUDED.last_sequence),
    last_synced_at = COALESCE(machine_sync_cursors.last_synced_at, EXCLUDED.last_synced_at),
    updated_at = NOW()
WHERE machine_sync_cursors.last_sequence < :target_last_sequence::bigint;

\echo '=== machine_sync_cursors after (expect offline.last_sequence >= ':target_last_sequence') ==='
SELECT machine_id, stream_name, last_sequence, last_synced_at, updated_at
FROM machine_sync_cursors
WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid
ORDER BY stream_name;
\endif
