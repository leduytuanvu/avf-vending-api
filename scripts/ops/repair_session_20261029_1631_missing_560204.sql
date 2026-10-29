-- Repair AVF000195 session 2026-10-29 16:31 (VN). Insert the missing recycler
-- 10.000₫ bill whose ICT hex 560204 collided with an older boot's acceptance.
-- New identity is boot-scoped: {boot_id}:560204. Do not delete the old hex row.
--
-- Usage:
--   psql "$DATABASE_URL" -v dry_run=1 -f scripts/ops/repair_session_20261029_1631_missing_560204.sql
--   psql "$DATABASE_URL" -v dry_run=0 -f scripts/ops/repair_session_20261029_1631_missing_560204.sql

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
\set window_from '2026-10-29T09:31:00Z'
\endif
\if :{?window_to}
\else
\set window_to '2026-10-29T09:34:00Z'
\endif
\if :{?boot_id}
\else
\set boot_id '8a9e2cfc-b468-49d7-81fc-0c9d3f806aec'
\endif
\if :{?legacy_hex}
\else
\set legacy_hex '560204'
\endif
\if :{?offline_seq}
\else
\set offline_seq 585
\endif

SELECT :'boot_id' || ':' || :'legacy_hex' AS forensic_device_event_id \gset

\echo '=== Session 16:31 missing 560204 repair machine=' :machine_id ' dry_run=' :dry_run ' ==='
\echo 'window=' :window_from ' .. ' :window_to
\echo 'forensic_id=' :forensic_device_event_id ' offline_seq=' :offline_seq

\echo '=== Collision: cash_acceptance_events device_event_id matching 560204 ==='
SELECT cae.device_event_id,
       cae.boot_id,
       cae.denomination_minor,
       cae.credit_source,
       cae.accepted_at
FROM cash_acceptance_events cae
WHERE cae.machine_id = :'machine_id'::uuid
  AND (
      cae.device_event_id = :'legacy_hex'
      OR cae.device_event_id LIKE '%' || :'legacy_hex' || '%'
  )
ORDER BY cae.accepted_at;

\echo '=== Offline seq for 560204 (expect 585 processed, denom 10000) ==='
SELECT moe.offline_sequence,
       moe.processing_status,
       moe.occurred_at,
       moe.idempotency_key,
       moe.payload->>'kind' AS payload_kind,
       COALESCE(moe.payload->>'denominationMinor', moe.payload#>>'{events,0,denominationMinor}') AS denom,
       COALESCE(moe.payload->>'creditSource', moe.payload#>>'{events,0,creditSource}') AS credit_source
FROM machine_offline_events moe
WHERE moe.machine_id = :'machine_id'::uuid
  AND (
      moe.offline_sequence = :offline_seq
      OR moe.idempotency_key LIKE '%' || :'legacy_hex' || '%'
  )
ORDER BY moe.offline_sequence;

\echo '=== Window acceptances (expect 9 before repair, 10 after) ==='
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
       coalesce(sum(denomination_minor), 0) AS window_acceptance_sum,
       count(*) FILTER (WHERE denomination_minor = 5000) AS n_5k,
       count(*) FILTER (WHERE denomination_minor = 10000) AS n_10k,
       count(*) FILTER (WHERE denomination_minor = 20000) AS n_20k,
       count(*) FILTER (WHERE denomination_minor = 50000) AS n_50k
FROM cash_acceptance_events
WHERE machine_id = :'machine_id'::uuid
  AND accepted_at >= :'window_from'::timestamptz
  AND accepted_at < :'window_to'::timestamptz;

\echo '=== Window cash_accepted / change_dispensed ledger ==='
SELECT fle.entry_type,
       count(*) AS n,
       coalesce(sum(fle.signed_amount_minor), 0) AS sum_minor
FROM financial_ledger_entries fle
WHERE fle.machine_id = :'machine_id'::uuid
  AND fle.occurred_at >= :'window_from'::timestamptz
  AND fle.occurred_at < :'window_to'::timestamptz
  AND fle.entry_type IN ('cash_accepted', 'change_dispensed')
GROUP BY fle.entry_type
ORDER BY fle.entry_type;

\echo '=== Window payout notes (expect 4 notes / 40000 confirmed) ==='
SELECT cpe.withdrawal_id,
       cpe.note_sequence,
       cpe.event_type,
       cpe.outcome_finality,
       cpe.denomination_minor,
       cpe.amount_minor,
       cpe.occurred_at_device
FROM cash_payout_events cpe
WHERE cpe.machine_id = :'machine_id'::uuid
  AND cpe.occurred_at_device >= :'window_from'::timestamptz
  AND cpe.occurred_at_device < :'window_to'::timestamptz
ORDER BY cpe.occurred_at_device, cpe.note_sequence, cpe.event_type;

SELECT count(*) FILTER (
           WHERE outcome_finality = 'confirmed'
             AND event_type IN ('NOTE_CONFIRMED', 'NOTE_DELIVERED_AFTER_FAULT', 'COMPLETED')
       ) AS confirmed_notes,
       coalesce(sum(amount_minor) FILTER (
           WHERE outcome_finality = 'confirmed'
             AND event_type IN ('NOTE_CONFIRMED', 'NOTE_DELIVERED_AFTER_FAULT', 'COMPLETED')
       ), 0) AS confirmed_sum
FROM cash_payout_events
WHERE machine_id = :'machine_id'::uuid
  AND occurred_at_device >= :'window_from'::timestamptz
  AND occurred_at_device < :'window_to'::timestamptz;

\if :dry_run
\echo '=== DRY RUN: no inserts ==='
\else
\echo '=== APPLY: insert boot-scoped 10k acceptance if missing ==='
INSERT INTO cash_acceptance_events (
    machine_id,
    device_event_id,
    denomination_minor,
    credit_source,
    currency,
    accepted_at,
    boot_id,
    occurred_at_device,
    raw_metadata
)
SELECT :'machine_id'::uuid,
       :'forensic_device_event_id',
       10000,
       'stored_in_recycler',
       'VND',
       src.occurred_at,
       :'boot_id',
       src.occurred_at,
       jsonb_build_object(
           'rawRecordHex', :'legacy_hex',
           'bootId', :'boot_id',
           'deviceEventId', :'forensic_device_event_id',
           'creditSource', 'stored_in_recycler',
           'backfill', 'session_20261029_1631'
       )
FROM (
    SELECT COALESCE(
        (
            SELECT moe.occurred_at
            FROM machine_offline_events moe
            WHERE moe.machine_id = :'machine_id'::uuid
              AND moe.offline_sequence = :offline_seq
        ),
        :'window_from'::timestamptz
    ) AS occurred_at
) src
WHERE NOT EXISTS (
    SELECT 1
    FROM cash_acceptance_events cae
    WHERE cae.machine_id = :'machine_id'::uuid
      AND cae.device_event_id = :'forensic_device_event_id'
);

\echo '=== APPLY: insert matching cash_accepted ledger if missing ==='
INSERT INTO financial_ledger_entries (
    machine_id,
    entry_type,
    signed_amount_minor,
    currency,
    occurred_at,
    metadata
)
SELECT cae.machine_id,
       'cash_accepted',
       cae.denomination_minor,
       cae.currency,
       cae.accepted_at,
       cae.raw_metadata
FROM cash_acceptance_events cae
WHERE cae.machine_id = :'machine_id'::uuid
  AND cae.device_event_id = :'forensic_device_event_id'
  AND NOT EXISTS (
      SELECT 1
      FROM financial_ledger_entries fle
      WHERE fle.machine_id = cae.machine_id
        AND fle.entry_type = 'cash_accepted'
        AND fle.signed_amount_minor = cae.denomination_minor
        AND fle.occurred_at = cae.accepted_at
        AND (
            fle.metadata::text LIKE '%' || :'forensic_device_event_id' || '%'
            OR fle.metadata->>'deviceEventId' = :'forensic_device_event_id'
            OR fle.metadata->>'backfill' = 'session_20261029_1631'
        )
  );

\echo '=== APPLY: drop exact duplicate cash_accepted ledger rows in window (keep one per amount+time) ==='
DELETE FROM financial_ledger_entries fle
WHERE fle.id IN (
    SELECT d.id
    FROM (
        SELECT id,
               ROW_NUMBER() OVER (
                   PARTITION BY signed_amount_minor, occurred_at
                   ORDER BY id
               ) AS rn
        FROM financial_ledger_entries
        WHERE machine_id = :'machine_id'::uuid
          AND entry_type = 'cash_accepted'
          AND occurred_at >= :'window_from'::timestamptz
          AND occurred_at < :'window_to'::timestamptz
    ) d
    WHERE d.rn > 1
);
\endif

\echo '=== After: window acceptances (expect 10 = 2x5k + 4x10k + 2x20k + 2x50k) ==='
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
       coalesce(sum(denomination_minor), 0) AS window_acceptance_sum,
       count(*) FILTER (WHERE denomination_minor = 5000) AS n_5k,
       count(*) FILTER (WHERE denomination_minor = 10000) AS n_10k,
       count(*) FILTER (WHERE denomination_minor = 20000) AS n_20k,
       count(*) FILTER (WHERE denomination_minor = 50000) AS n_50k,
       count(*) FILTER (WHERE device_event_id = :'forensic_device_event_id') AS boot_scoped_560204,
       count(*) FILTER (WHERE device_event_id = :'legacy_hex') AS legacy_hex_560204_in_window
FROM cash_acceptance_events
WHERE machine_id = :'machine_id'::uuid
  AND accepted_at >= :'window_from'::timestamptz
  AND accepted_at < :'window_to'::timestamptz;

SELECT fle.entry_type,
       count(*) AS n,
       coalesce(sum(fle.signed_amount_minor), 0) AS sum_minor
FROM financial_ledger_entries fle
WHERE fle.machine_id = :'machine_id'::uuid
  AND fle.occurred_at >= :'window_from'::timestamptz
  AND fle.occurred_at < :'window_to'::timestamptz
  AND fle.entry_type IN ('cash_accepted', 'change_dispensed')
GROUP BY fle.entry_type
ORDER BY fle.entry_type;
