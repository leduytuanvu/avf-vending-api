-- Cash movement offline push: idempotency ledger poisoned without cash_acceptance_events.
-- Incident 2026-10-27 18:36Z: ICT hex 920204 (historic) and 940105 (live) minted unbounded v3 keys.
-- Machine from 2026-10-08 / 2026-10-27 incident (adjust UUID as needed).
-- Run VERIFY sections first with a read-only role; apply DELETE only after the patched APK is installed
-- (or the old app is stopped). Deleting while the old APK still runs will remint v3 immediately.
--
-- Usage:
--   psql "$DATABASE_URL" -v dry_run=1 -v machine_id='01a0a7e5-3c68-7895-b526-bcb6504bccfb' \
--     -v key_token_a='920204' -v key_token_b='940105' \
--     -f scripts/ops/repair_cash_movement_idempotency_poison.sql

\set ON_ERROR_STOP on

\if :{?machine_id}
\else
\set machine_id '01a0a7e5-3c68-7895-b526-bcb6504bccfb'
\endif

\if :{?key_token_a}
\else
\set key_token_a '920204'
\endif

\if :{?key_token_b}
\else
\set key_token_b '940105'
\endif

\if :{?dry_run}
\else
\set dry_run 1
\endif

-- 1) Idempotency rows for the incident cash_movement keys (PushOfflineEvents ledger)
SELECT mik.operation,
       mik.idempotency_key,
       mik.status,
       mik.last_seen_at,
       encode(mik.request_hash, 'hex') AS request_hash_hex
FROM machine_idempotency_keys mik
WHERE mik.machine_id = :'machine_id'::uuid
  AND mik.idempotency_key LIKE 'cash_movement:%'
  AND mik.idempotency_key NOT LIKE '%:payout:%'
  AND (
      mik.idempotency_key LIKE '%' || :'key_token_a' || '%'
      OR mik.idempotency_key LIKE '%' || :'key_token_b' || '%'
  )
ORDER BY mik.last_seen_at DESC
LIMIT 200;

-- 2) Offline ledger rows for the same keys
SELECT offline_sequence,
       idempotency_key,
       event_type,
       processing_status,
       processing_error,
       occurred_at
FROM machine_offline_events
WHERE machine_id = :'machine_id'::uuid
  AND idempotency_key LIKE 'cash_movement:%'
  AND idempotency_key NOT LIKE '%:payout:%'
  AND (
      idempotency_key LIKE '%' || :'key_token_a' || '%'
      OR idempotency_key LIKE '%' || :'key_token_b' || '%'
  )
ORDER BY offline_sequence DESC
LIMIT 200;

-- 3) Forensic acceptance (bill insert) — empty when web shows no deposits
SELECT device_event_id,
       denomination_minor,
       credit_source,
       accepted_at
FROM cash_acceptance_events
WHERE machine_id = :'machine_id'::uuid
  AND (
      device_event_id LIKE '%' || :'key_token_a' || '%'
      OR device_event_id LIKE '%' || :'key_token_b' || '%'
  )
ORDER BY accepted_at DESC
LIMIT 100;

-- 4) Keys with ledger conflict but no acceptance row (poison candidates)
SELECT mik.idempotency_key,
       mik.status,
       mik.last_seen_at
FROM machine_idempotency_keys mik
WHERE mik.machine_id = :'machine_id'::uuid
  AND mik.idempotency_key LIKE 'cash_movement:%'
  AND mik.idempotency_key NOT LIKE '%:payout:%'
  AND (
      mik.idempotency_key LIKE '%' || :'key_token_a' || '%'
      OR mik.idempotency_key LIKE '%' || :'key_token_b' || '%'
  )
  AND NOT EXISTS (
      SELECT 1
      FROM cash_acceptance_events cae
      WHERE cae.machine_id = mik.machine_id
        AND mik.idempotency_key LIKE '%' || cae.device_event_id
  )
ORDER BY mik.last_seen_at DESC;

\echo '=== Poison candidates (section 4, hex-narrowed) ==='

\if :dry_run
\echo '=== DRY RUN: would delete poisoned idempotency keys (no writes) ==='
SELECT mik.idempotency_key, mik.status, mik.last_seen_at
FROM machine_idempotency_keys mik
WHERE mik.machine_id = :'machine_id'::uuid
  AND mik.idempotency_key LIKE 'cash_movement:%'
  AND mik.idempotency_key NOT LIKE '%:payout:%'
  AND (
      mik.idempotency_key LIKE '%' || :'key_token_a' || '%'
      OR mik.idempotency_key LIKE '%' || :'key_token_b' || '%'
  )
  AND NOT EXISTS (
      SELECT 1 FROM cash_acceptance_events cae
      WHERE cae.machine_id = mik.machine_id
        AND mik.idempotency_key LIKE '%' || cae.device_event_id
  );
\else
\echo '=== APPLY: reset offline ledger rows (processed bill cash without acceptance) ==='
UPDATE machine_offline_events moe
SET processing_status = 'failed_retryable',
    processing_error = 'ops_repair: missing cash_acceptance_events'
WHERE moe.machine_id = :'machine_id'::uuid
  AND moe.idempotency_key LIKE 'cash_movement:%'
  AND moe.idempotency_key NOT LIKE '%:payout:%'
  AND (
      moe.idempotency_key LIKE '%' || :'key_token_a' || '%'
      OR moe.idempotency_key LIKE '%' || :'key_token_b' || '%'
  )
  AND lower(moe.processing_status) IN ('processed', 'succeeded')
  AND NOT EXISTS (
      SELECT 1 FROM cash_acceptance_events cae
      WHERE cae.machine_id = moe.machine_id
        AND moe.idempotency_key LIKE '%' || cae.device_event_id
  );

\echo '=== APPLY: deleting poisoned idempotency keys ==='
DELETE FROM machine_idempotency_keys mik
WHERE mik.machine_id = :'machine_id'::uuid
  AND mik.idempotency_key LIKE 'cash_movement:%'
  AND mik.idempotency_key NOT LIKE '%:payout:%'
  AND (
      mik.idempotency_key LIKE '%' || :'key_token_a' || '%'
      OR mik.idempotency_key LIKE '%' || :'key_token_b' || '%'
  )
  AND NOT EXISTS (
      SELECT 1 FROM cash_acceptance_events cae
      WHERE cae.machine_id = mik.machine_id
        AND mik.idempotency_key LIKE '%' || cae.device_event_id
  );
\endif
