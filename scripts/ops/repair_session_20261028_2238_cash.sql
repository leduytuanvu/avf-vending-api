-- Repair AVF000195 session 2026-10-28 22:38 (VN). Clears poisoned cash_movement
-- idempotency keys / failed_terminal offline rows for D20204/D40204/D60204 and
-- payout 818f8baf when cash_acceptance_events / cash_payout_events are missing.
-- Do not INSERT cash rows. Do not touch unrelated machines.
--
-- Usage:
--   psql "$DATABASE_URL" -v dry_run=1 -f scripts/ops/repair_session_20261028_2238_cash.sql
--   psql "$DATABASE_URL" -v dry_run=0 -f scripts/ops/repair_session_20261028_2238_cash.sql

\set ON_ERROR_STOP on

\if :{?dry_run}
\else
\set dry_run 1
\endif
\if :{?machine_id}
\else
\set machine_id '01a0a7e5-3c68-7895-b526-bcb6504bccfb'
\endif

\echo '=== Session 22:38 cash repair machine=' :machine_id ' dry_run=' :dry_run ' ==='

\echo '=== Preview: cash_movement idempotency keys for session tokens ==='
SELECT mik.idempotency_key, mik.status, mik.last_seen_at
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

\if :dry_run
\echo '=== DRY RUN: would reset processed cash_movement offline rows without acceptance ==='
SELECT moe.offline_sequence, moe.idempotency_key, moe.processing_status
FROM machine_offline_events moe
WHERE moe.machine_id = :'machine_id'::uuid
  AND moe.idempotency_key LIKE 'cash_movement:%'
  AND moe.idempotency_key NOT LIKE '%:payout:%'
  AND (
      moe.idempotency_key LIKE '%D10004%'
      OR moe.idempotency_key LIKE '%D20204%'
      OR moe.idempotency_key LIKE '%D30004%'
      OR moe.idempotency_key LIKE '%D40204%'
      OR moe.idempotency_key LIKE '%D50004%'
      OR moe.idempotency_key LIKE '%D60204%'
  )
  AND lower(moe.processing_status) IN ('processed', 'succeeded', 'failed_terminal', 'rejected')
  AND NOT EXISTS (
      SELECT 1 FROM cash_acceptance_events cae
      WHERE cae.machine_id = moe.machine_id
        AND moe.idempotency_key LIKE '%' || cae.device_event_id
  );

\echo '=== DRY RUN: would reset payout offline rows without cash_payout_events ==='
SELECT moe.offline_sequence, moe.idempotency_key, moe.processing_status
FROM machine_offline_events moe
WHERE moe.machine_id = :'machine_id'::uuid
  AND moe.idempotency_key LIKE '%payout:818f8baf%'
  AND lower(moe.processing_status) IN ('processed', 'succeeded', 'failed_terminal', 'rejected')
  AND NOT EXISTS (
      SELECT 1 FROM cash_payout_events cpe
      WHERE cpe.machine_id = moe.machine_id
        AND cpe.withdrawal_id = '818f8baf-08d4-45f2-8270-a2b1b98deb46'
  );
\else
\echo '=== APPLY: reset cash_movement offline rows missing acceptance ==='
UPDATE machine_offline_events moe
SET processing_status = 'failed_retryable',
    processing_error = 'ops_repair: session_20261028_2238 missing cash_acceptance_events'
WHERE moe.machine_id = :'machine_id'::uuid
  AND moe.idempotency_key LIKE 'cash_movement:%'
  AND moe.idempotency_key NOT LIKE '%:payout:%'
  AND (
      moe.idempotency_key LIKE '%D10004%'
      OR moe.idempotency_key LIKE '%D20204%'
      OR moe.idempotency_key LIKE '%D30004%'
      OR moe.idempotency_key LIKE '%D40204%'
      OR moe.idempotency_key LIKE '%D50004%'
      OR moe.idempotency_key LIKE '%D60204%'
  )
  AND lower(moe.processing_status) IN ('processed', 'succeeded', 'failed_terminal', 'rejected')
  AND NOT EXISTS (
      SELECT 1 FROM cash_acceptance_events cae
      WHERE cae.machine_id = moe.machine_id
        AND moe.idempotency_key LIKE '%' || cae.device_event_id
  );

\echo '=== APPLY: delete poisoned cash_movement idempotency keys without acceptance ==='
DELETE FROM machine_idempotency_keys mik
WHERE mik.machine_id = :'machine_id'::uuid
  AND mik.idempotency_key LIKE 'cash_movement:%'
  AND mik.idempotency_key NOT LIKE '%:payout:%'
  AND (
      mik.idempotency_key LIKE '%D10004%'
      OR mik.idempotency_key LIKE '%D20204%'
      OR mik.idempotency_key LIKE '%D30004%'
      OR mik.idempotency_key LIKE '%D40204%'
      OR mik.idempotency_key LIKE '%D50004%'
      OR mik.idempotency_key LIKE '%D60204%'
  )
  AND NOT EXISTS (
      SELECT 1 FROM cash_acceptance_events cae
      WHERE cae.machine_id = mik.machine_id
        AND mik.idempotency_key LIKE '%' || cae.device_event_id
  );

\echo '=== APPLY: reset payout offline rows missing cash_payout_events ==='
UPDATE machine_offline_events moe
SET processing_status = 'failed_retryable',
    processing_error = 'ops_repair: session_20261028_2238 missing cash_payout_events'
WHERE moe.machine_id = :'machine_id'::uuid
  AND moe.idempotency_key LIKE '%payout:818f8baf%'
  AND lower(moe.processing_status) IN ('processed', 'succeeded', 'failed_terminal', 'rejected')
  AND NOT EXISTS (
      SELECT 1 FROM cash_payout_events cpe
      WHERE cpe.machine_id = moe.machine_id
        AND cpe.withdrawal_id = '818f8baf-08d4-45f2-8270-a2b1b98deb46'
  );

\echo '=== APPLY: delete poisoned payout idempotency keys ==='
DELETE FROM machine_idempotency_keys mik
WHERE mik.machine_id = :'machine_id'::uuid
  AND mik.idempotency_key LIKE 'cash_movement:%'
  AND mik.idempotency_key LIKE '%818f8baf%'
  AND NOT EXISTS (
      SELECT 1 FROM cash_payout_events cpe
      WHERE cpe.machine_id = mik.machine_id
        AND cpe.withdrawal_id = '818f8baf-08d4-45f2-8270-a2b1b98deb46'
  );
\endif
