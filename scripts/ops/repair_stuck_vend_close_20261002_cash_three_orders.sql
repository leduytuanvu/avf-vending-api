-- Repair cash orders stuck in vending after hardware completed (log 2026-10-02 19:56–19:59).
-- ConfirmVendSuccess never applied due to duplicate StartVend idempotency_payload_mismatch.
--
-- Orders (all lines dispensed successfully on machine):
--   01a0fcb0-2192-7870-828c-e93375e18904 (1 line)
--   01a0fcb0-ac77-740f-8161-933a344b53a9 (2 lines)
--   01a0fcb1-8e49-7951-b389-721123a34c31 (3 lines)
--
-- Usage:
--   psql "$DATABASE_URL" -v dry_run=1 -f scripts/ops/repair_stuck_vend_close_20261002_cash_three_orders.sql
--   psql "$DATABASE_URL" -v dry_run=0 -f scripts/ops/repair_stuck_vend_close_20261002_cash_three_orders.sql

\set ON_ERROR_STOP on

\echo '=== Preview: target orders before repair ==='
SELECT o.id,
       o.status,
       o.total_minor,
       m.code AS machine_code,
       o.created_at,
       array_agg(vs.state ORDER BY vs.line_sequence) AS line_states,
       array_agg(vs.line_sequence ORDER BY vs.line_sequence) AS line_sequences
FROM orders o
JOIN machines m ON m.id = o.machine_id
JOIN vend_sessions vs ON vs.order_id = o.id
WHERE o.id IN (
    '01a0fcb0-2192-7870-828c-e93375e18904'::uuid,
    '01a0fcb0-ac77-740f-8161-933a344b53a9'::uuid,
    '01a0fcb1-8e49-7951-b389-721123a34c31'::uuid
)
GROUP BY o.id, o.status, o.total_minor, m.code, o.created_at
ORDER BY o.created_at;

\if :dry_run
\echo '=== DRY RUN: would mark in_progress lines success (no writes) ==='
SELECT vs.order_id,
       vs.line_sequence,
       vs.slot_index,
       vs.state AS current_state,
       'success' AS next_state
FROM vend_sessions vs
WHERE vs.order_id IN (
    '01a0fcb0-2192-7870-828c-e93375e18904'::uuid,
    '01a0fcb0-ac77-740f-8161-933a344b53a9'::uuid,
    '01a0fcb1-8e49-7951-b389-721123a34c31'::uuid
)
  AND vs.state = 'in_progress'
ORDER BY vs.order_id, vs.line_sequence;
\else
BEGIN;

\echo '=== Applying line-level success for hardware-completed cash vends ==='

UPDATE vend_sessions
SET
    state = 'success',
    failure_reason = NULL,
    completed_at = COALESCE(completed_at, now())
WHERE order_id IN (
    '01a0fcb0-2192-7870-828c-e93375e18904'::uuid,
    '01a0fcb0-ac77-740f-8161-933a344b53a9'::uuid,
    '01a0fcb1-8e49-7951-b389-721123a34c31'::uuid
)
  AND state = 'in_progress';

\echo '=== Resolving terminal order status ==='

WITH terminal_sessions AS (
    SELECT
        o.id AS order_id,
        bool_or(vs.state = 'success') AS any_success,
        bool_or(vs.state = 'failed') AS any_failed,
        bool_and(vs.state IN ('success', 'failed', 'cancelled', 'skipped')) AS all_terminal
    FROM orders o
    JOIN vend_sessions vs ON vs.order_id = o.id
    WHERE o.id IN (
        '01a0fcb0-2192-7870-828c-e93375e18904'::uuid,
        '01a0fcb0-ac77-740f-8161-933a344b53a9'::uuid,
        '01a0fcb1-8e49-7951-b389-721123a34c31'::uuid
    )
    GROUP BY o.id
),
resolved AS (
    SELECT
        order_id,
        CASE
            WHEN any_success AND any_failed THEN 'partially_completed'
            WHEN any_success THEN 'completed'
            WHEN any_failed THEN 'failed'
            ELSE 'vending'
        END AS next_status
    FROM terminal_sessions
    WHERE all_terminal
)
UPDATE orders o
SET
    status = r.next_status,
    updated_at = now()
FROM resolved r
WHERE o.id = r.order_id
  AND r.next_status <> 'vending'
  AND o.status = 'vending';

COMMIT;
\endif

\echo '=== Preview: target orders after repair ==='
SELECT o.id,
       o.status,
       o.total_minor,
       m.code AS machine_code,
       array_agg(vs.state ORDER BY vs.line_sequence) AS line_states
FROM orders o
JOIN machines m ON m.id = o.machine_id
JOIN vend_sessions vs ON vs.order_id = o.id
WHERE o.id IN (
    '01a0fcb0-2192-7870-828c-e93375e18904'::uuid,
    '01a0fcb0-ac77-740f-8161-933a344b53a9'::uuid,
    '01a0fcb1-8e49-7951-b389-721123a34c31'::uuid
)
GROUP BY o.id, o.status, o.total_minor, m.code
ORDER BY o.created_at;
