-- Repair AVF000195 session 2026-10-29 15:24 (VN). Deletes forensic phantom
-- bill_credit device_event_id=3E0106 (50.000₫ replay after settled escrow).
-- Do not touch the 10 real notes 230103…390106. Do not rewind offline events
-- (re-ingest would recreate the phantom while the old APK is still live).
--
-- Usage:
--   psql "$DATABASE_URL" -v dry_run=1 -f scripts/ops/repair_session_20261029_1524_phantom_bill_credit.sql
--   psql "$DATABASE_URL" -v dry_run=0 -f scripts/ops/repair_session_20261029_1524_phantom_bill_credit.sql

\set ON_ERROR_STOP on

\if :{?dry_run}
\else
\set dry_run 1
\endif
\if :{?machine_id}
\else
\set machine_id '01a0a7e5-3c68-7895-b526-bcb6504bccfb'
\endif
\if :{?window_from}
\else
\set window_from '2026-10-29T08:24:00Z'
\endif
\if :{?window_to}
\else
\set window_to '2026-10-29T08:27:00Z'
\endif
\if :{?phantom_device_event_id}
\else
\set phantom_device_event_id '3E0106'
\endif

\echo '=== Session 15:24 phantom bill_credit repair machine=' :machine_id ' dry_run=' :dry_run ' ==='
\echo 'window=' :window_from ' .. ' :window_to ' phantom=' :phantom_device_event_id

\echo '=== Preview: cash_acceptance_events in window ==='
SELECT cae.device_event_id,
       cae.denomination_minor,
       cae.credit_source,
       cae.accepted_at,
       cae.raw_metadata->>'raw_record_hex' AS raw_record_hex
FROM cash_acceptance_events cae
WHERE cae.machine_id = :'machine_id'::uuid
  AND cae.accepted_at >= :'window_from'::timestamptz
  AND cae.accepted_at < :'window_to'::timestamptz
ORDER BY cae.accepted_at;

\echo '=== Preview: phantom cash_acceptance_events ==='
SELECT cae.id,
       cae.device_event_id,
       cae.denomination_minor,
       cae.credit_source,
       cae.accepted_at,
       cae.raw_metadata
FROM cash_acceptance_events cae
WHERE cae.machine_id = :'machine_id'::uuid
  AND cae.device_event_id = :'phantom_device_event_id'
  AND cae.accepted_at >= :'window_from'::timestamptz
  AND cae.accepted_at < :'window_to'::timestamptz
  AND cae.denomination_minor = 50000;

\echo '=== Preview: phantom cash_accepted ledger rows ==='
SELECT fle.id,
       fle.entry_type,
       fle.signed_amount_minor,
       fle.occurred_at,
       fle.metadata
FROM financial_ledger_entries fle
JOIN cash_acceptance_events cae
  ON cae.machine_id = fle.machine_id
 AND cae.device_event_id = :'phantom_device_event_id'
 AND cae.accepted_at >= :'window_from'::timestamptz
 AND cae.accepted_at < :'window_to'::timestamptz
 AND cae.denomination_minor = 50000
 AND fle.entry_type = 'cash_accepted'
 AND fle.signed_amount_minor = cae.denomination_minor
 AND fle.occurred_at = cae.accepted_at
WHERE fle.machine_id = :'machine_id'::uuid
  AND (
      fle.metadata::text LIKE '%' || :'phantom_device_event_id' || '%'
      OR fle.metadata = '{}'::jsonb
      OR COALESCE(fle.metadata->>'raw_record_hex', '') = :'phantom_device_event_id'
      OR COALESCE(fle.metadata->>'device_event_id', '') = :'phantom_device_event_id'
  );

\echo '=== Preview: machine_idempotency_keys containing phantom ==='
SELECT mik.operation,
       mik.idempotency_key,
       mik.status,
       mik.last_seen_at
FROM machine_idempotency_keys mik
WHERE mik.machine_id = :'machine_id'::uuid
  AND mik.idempotency_key LIKE '%' || :'phantom_device_event_id' || '%'
ORDER BY mik.last_seen_at;

\echo '=== Preview: machine_offline_events containing phantom (leave processed) ==='
SELECT moe.offline_sequence,
       moe.event_type,
       moe.idempotency_key,
       moe.processing_status,
       moe.occurred_at
FROM machine_offline_events moe
WHERE moe.machine_id = :'machine_id'::uuid
  AND (
      moe.idempotency_key LIKE '%' || :'phantom_device_event_id' || '%'
      OR moe.payload::text LIKE '%' || :'phantom_device_event_id' || '%'
  )
ORDER BY moe.offline_sequence;

\if :dry_run
\echo '=== DRY RUN: no deletes ==='
\else
\echo '=== APPLY: delete phantom cash_accepted ledger rows ==='
DELETE FROM financial_ledger_entries fle
USING cash_acceptance_events cae
WHERE fle.machine_id = :'machine_id'::uuid
  AND cae.machine_id = fle.machine_id
  AND cae.device_event_id = :'phantom_device_event_id'
  AND cae.accepted_at >= :'window_from'::timestamptz
  AND cae.accepted_at < :'window_to'::timestamptz
  AND cae.denomination_minor = 50000
  AND fle.entry_type = 'cash_accepted'
  AND fle.signed_amount_minor = cae.denomination_minor
  AND fle.occurred_at = cae.accepted_at
  AND (
      fle.metadata::text LIKE '%' || :'phantom_device_event_id' || '%'
      OR fle.metadata = '{}'::jsonb
      OR COALESCE(fle.metadata->>'raw_record_hex', '') = :'phantom_device_event_id'
      OR COALESCE(fle.metadata->>'device_event_id', '') = :'phantom_device_event_id'
  );

\echo '=== APPLY: delete phantom cash_acceptance_events ==='
DELETE FROM cash_acceptance_events cae
WHERE cae.machine_id = :'machine_id'::uuid
  AND cae.device_event_id = :'phantom_device_event_id'
  AND cae.accepted_at >= :'window_from'::timestamptz
  AND cae.accepted_at < :'window_to'::timestamptz
  AND cae.denomination_minor = 50000;

\echo '=== APPLY: delete phantom machine_idempotency_keys ==='
DELETE FROM machine_idempotency_keys mik
WHERE mik.machine_id = :'machine_id'::uuid
  AND mik.idempotency_key LIKE '%' || :'phantom_device_event_id' || '%';
\endif

\echo '=== After: window acceptances (expect 10, no 3E0106) ==='
SELECT cae.device_event_id,
       cae.denomination_minor,
       cae.credit_source,
       cae.accepted_at
FROM cash_acceptance_events cae
WHERE cae.machine_id = :'machine_id'::uuid
  AND cae.accepted_at >= :'window_from'::timestamptz
  AND cae.accepted_at < :'window_to'::timestamptz
ORDER BY cae.accepted_at;

SELECT count(*) AS window_acceptance_count,
       count(*) FILTER (WHERE device_event_id = :'phantom_device_event_id') AS phantom_acceptance_count
FROM cash_acceptance_events
WHERE machine_id = :'machine_id'::uuid
  AND accepted_at >= :'window_from'::timestamptz
  AND accepted_at < :'window_to'::timestamptz;

SELECT count(*) AS phantom_idempotency_count
FROM machine_idempotency_keys
WHERE machine_id = :'machine_id'::uuid
  AND idempotency_key LIKE '%' || :'phantom_device_event_id' || '%';
