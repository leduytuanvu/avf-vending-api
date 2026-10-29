-- Cash movement offline push: idempotency ledger poisoned without cash_acceptance_events.
-- Incident 2026-10-27 18:36Z: ICT hex 920204 (historic) and 940105 (live) minted unbounded v3 keys.
-- Incident 2026-10-28 14:00: same-hex escrow_held vs bill_credit (token B20204) stalled the
-- offline sequence so nạp/rút never reached cash_acceptance_events / cash_payout_events.
-- Incident 2026-10-28 17:04: duplicate bill_credit rows shared BC0103/C00103/C20103/BE0103 and
-- v3 rekey reused client_event_id, stalling floor 11189 so nạp/rút still missing on web.
-- Payout NOTE_CONFIRMED sat behind that floor; apply also clears payout keys with no
-- cash_payout_events row.
-- Machine from 2026-10-08 / 2026-10-27 incident (adjust UUID as needed).
-- Run VERIFY sections first with a read-only role; apply DELETE only after the patched APK is installed
-- (kind-scoped cash_movement keys + enqueueIfAbsent + v3-only rekey, or the old app is stopped).
-- Deleting while the old APK still mints colliding …:BC0103:bill_credit keys will re-poison immediately.
--
-- Usage:
--   psql "$DATABASE_URL" -v dry_run=1 -v machine_id='01a0a7e5-3c68-7895-b526-bcb6504bccfb' \
--     -v key_token_a='920204' -v key_token_b='940105' \
--     -v key_token_c='BC0103' -v key_token_d='C00103' -v key_token_e='C20103' \
--     -v key_token_f='BE0103' \
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

\if :{?key_token_c}
\else
\set key_token_c 'BC0103'
\endif

\if :{?key_token_d}
\else
\set key_token_d 'C00103'
\endif

\if :{?key_token_e}
\else
\set key_token_e 'C20103'
\endif

\if :{?key_token_f}
\else
\set key_token_f 'BE0103'
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
      OR mik.idempotency_key LIKE '%' || :'key_token_c' || '%'
      OR mik.idempotency_key LIKE '%' || :'key_token_d' || '%'
      OR mik.idempotency_key LIKE '%' || :'key_token_e' || '%'
      OR mik.idempotency_key LIKE '%' || :'key_token_f' || '%'
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
      OR idempotency_key LIKE '%' || :'key_token_c' || '%'
      OR idempotency_key LIKE '%' || :'key_token_d' || '%'
      OR idempotency_key LIKE '%' || :'key_token_e' || '%'
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
      OR device_event_id LIKE '%' || :'key_token_c' || '%'
      OR device_event_id LIKE '%' || :'key_token_d' || '%'
      OR device_event_id LIKE '%' || :'key_token_e' || '%'
      OR device_event_id LIKE '%' || :'key_token_f' || '%'
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
      OR mik.idempotency_key LIKE '%' || :'key_token_c' || '%'
      OR mik.idempotency_key LIKE '%' || :'key_token_d' || '%'
      OR mik.idempotency_key LIKE '%' || :'key_token_e' || '%'
      OR mik.idempotency_key LIKE '%' || :'key_token_f' || '%'
  )
  AND NOT EXISTS (
      SELECT 1
      FROM cash_acceptance_events cae
      WHERE cae.machine_id = mik.machine_id
        AND mik.idempotency_key LIKE '%' || cae.device_event_id
  )
ORDER BY mik.last_seen_at DESC;

\echo '=== Poison candidates (section 4, hex-narrowed) ==='

-- 5) Inventory: every cash_movement key without a cash_acceptance_events row
-- (17:04 stall can poison hexes beyond the token list). VERIFY only here;
-- APPLY below stays token-narrowed unless operators expand tokens.
\echo '=== All cash_movement keys missing cash_acceptance_events (verify) ==='
SELECT mik.idempotency_key,
       mik.status,
       mik.last_seen_at
FROM machine_idempotency_keys mik
WHERE mik.machine_id = :'machine_id'::uuid
  AND mik.idempotency_key LIKE 'cash_movement:%'
  AND mik.idempotency_key NOT LIKE '%:payout:%'
  AND NOT EXISTS (
      SELECT 1 FROM cash_acceptance_events cae
      WHERE cae.machine_id = mik.machine_id
        AND mik.idempotency_key LIKE '%' || cae.device_event_id
  )
ORDER BY mik.last_seen_at DESC
LIMIT 200;

-- 6) Payout keys with no cash_payout_events row (17:04 NOTE_CONFIRMED sat behind the stall).
\echo '=== Payout keys missing cash_payout_events (verify) ==='
SELECT mik.idempotency_key,
       mik.status,
       mik.last_seen_at
FROM machine_idempotency_keys mik
WHERE mik.machine_id = :'machine_id'::uuid
  AND mik.idempotency_key LIKE 'cash_movement:%'
  AND mik.idempotency_key LIKE '%:payout:%'
  AND NOT EXISTS (
      SELECT 1 FROM cash_payout_events cpe
      WHERE cpe.machine_id = mik.machine_id
        AND mik.idempotency_key LIKE '%' || cpe.device_event_id
  )
ORDER BY mik.last_seen_at DESC
LIMIT 200;

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
      OR mik.idempotency_key LIKE '%' || :'key_token_c' || '%'
      OR mik.idempotency_key LIKE '%' || :'key_token_d' || '%'
      OR mik.idempotency_key LIKE '%' || :'key_token_e' || '%'
      OR mik.idempotency_key LIKE '%' || :'key_token_f' || '%'
  )
  AND NOT EXISTS (
      SELECT 1 FROM cash_acceptance_events cae
      WHERE cae.machine_id = mik.machine_id
        AND mik.idempotency_key LIKE '%' || cae.device_event_id
  );

\echo '=== DRY RUN: would delete poisoned payout idempotency keys (no writes) ==='
SELECT mik.idempotency_key, mik.status, mik.last_seen_at
FROM machine_idempotency_keys mik
WHERE mik.machine_id = :'machine_id'::uuid
  AND mik.idempotency_key LIKE 'cash_movement:%'
  AND mik.idempotency_key LIKE '%:payout:%'
  AND NOT EXISTS (
      SELECT 1 FROM cash_payout_events cpe
      WHERE cpe.machine_id = mik.machine_id
        AND mik.idempotency_key LIKE '%' || cpe.device_event_id
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
      OR moe.idempotency_key LIKE '%' || :'key_token_c' || '%'
      OR moe.idempotency_key LIKE '%' || :'key_token_d' || '%'
      OR moe.idempotency_key LIKE '%' || :'key_token_e' || '%'
      OR moe.idempotency_key LIKE '%' || :'key_token_f' || '%'
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
      OR mik.idempotency_key LIKE '%' || :'key_token_c' || '%'
      OR mik.idempotency_key LIKE '%' || :'key_token_d' || '%'
      OR mik.idempotency_key LIKE '%' || :'key_token_e' || '%'
      OR mik.idempotency_key LIKE '%' || :'key_token_f' || '%'
  )
  AND NOT EXISTS (
      SELECT 1 FROM cash_acceptance_events cae
      WHERE cae.machine_id = mik.machine_id
        AND mik.idempotency_key LIKE '%' || cae.device_event_id
  );

\echo '=== APPLY: reset offline ledger rows (processed payout without cash_payout_events) ==='
UPDATE machine_offline_events moe
SET processing_status = 'failed_retryable',
    processing_error = 'ops_repair: missing cash_payout_events'
WHERE moe.machine_id = :'machine_id'::uuid
  AND moe.idempotency_key LIKE 'cash_movement:%'
  AND moe.idempotency_key LIKE '%:payout:%'
  AND lower(moe.processing_status) IN ('processed', 'succeeded')
  AND NOT EXISTS (
      SELECT 1 FROM cash_payout_events cpe
      WHERE cpe.machine_id = moe.machine_id
        AND moe.idempotency_key LIKE '%' || cpe.device_event_id
  );

\echo '=== APPLY: deleting poisoned payout idempotency keys ==='
DELETE FROM machine_idempotency_keys mik
WHERE mik.machine_id = :'machine_id'::uuid
  AND mik.idempotency_key LIKE 'cash_movement:%'
  AND mik.idempotency_key LIKE '%:payout:%'
  AND NOT EXISTS (
      SELECT 1 FROM cash_payout_events cpe
      WHERE cpe.machine_id = mik.machine_id
        AND mik.idempotency_key LIKE '%' || cpe.device_event_id
  );
\endif
