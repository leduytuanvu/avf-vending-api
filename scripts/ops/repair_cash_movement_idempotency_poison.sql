-- Cash movement offline push: idempotency ledger poisoned without cash_acceptance_events.
-- Machine from 2026-10-08 incident (adjust UUID as needed).
-- Run VERIFY sections first with a read-only role; apply DELETE only after review.

\set machine_id '01a0a7e5-3c68-7895-b526-bcb6504bccfb'

-- 1) Idempotency rows for cash_movement keys (PushOfflineEvents ledger)
SELECT mik.operation,
       mik.idempotency_key,
       mik.status,
       mik.last_seen_at,
       encode(mik.request_hash, 'hex') AS request_hash_hex
FROM machine_idempotency_keys mik
WHERE mik.machine_id = :'machine_id'::uuid
  AND mik.idempotency_key LIKE 'cash_movement:%'
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
ORDER BY offline_sequence DESC
LIMIT 200;

-- 3) Forensic acceptance (bill insert) — empty when web shows no deposits
SELECT device_event_id,
       denomination_minor,
       credit_source,
       accepted_at
FROM cash_acceptance_events
WHERE machine_id = :'machine_id'::uuid
ORDER BY accepted_at DESC
LIMIT 100;

-- 4) Keys with ledger conflict but no acceptance row (poison candidates)
SELECT mik.idempotency_key,
       mik.status,
       mik.last_seen_at
FROM machine_idempotency_keys mik
WHERE mik.machine_id = :'machine_id'::uuid
  AND mik.idempotency_key LIKE 'cash_movement:%'
  AND NOT EXISTS (
      SELECT 1
      FROM cash_acceptance_events cae
      WHERE cae.machine_id = mik.machine_id
        AND cae.device_event_id = regexp_replace(
            mik.idempotency_key,
            '^cash_movement:(v2:)?' || :'machine_id' || '(:[^:]*)?:',
            ''
        )
  )
  AND mik.idempotency_key NOT LIKE '%:payout:%'
ORDER BY mik.last_seen_at DESC;

\echo '=== Poison candidates (section 4) ==='

\if :{?dry_run}
\else
\set dry_run 1
\endif

\if :dry_run
\echo '=== DRY RUN: would delete poisoned idempotency keys (no writes) ==='
SELECT mik.idempotency_key, mik.status, mik.last_seen_at
FROM machine_idempotency_keys mik
WHERE mik.machine_id = :'machine_id'::uuid
  AND mik.idempotency_key LIKE 'cash_movement:%'
  AND mik.idempotency_key NOT LIKE '%:payout:%'
  AND NOT EXISTS (
      SELECT 1 FROM cash_acceptance_events cae
      WHERE cae.machine_id = mik.machine_id
        AND mik.idempotency_key LIKE '%' || cae.device_event_id
  );
\else
\echo '=== APPLY: deleting poisoned idempotency keys ==='
DELETE FROM machine_idempotency_keys mik
WHERE mik.machine_id = :'machine_id'::uuid
  AND mik.idempotency_key LIKE 'cash_movement:%'
  AND mik.idempotency_key NOT LIKE '%:payout:%'
  AND NOT EXISTS (
      SELECT 1 FROM cash_acceptance_events cae
      WHERE cae.machine_id = mik.machine_id
        AND mik.idempotency_key LIKE '%' || cae.device_event_id
  );
\endif
