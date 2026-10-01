-- Repair orders stuck in `vending` / `in_progress` when machine logcat proves terminal outcomes
-- but ConfirmVendSuccess / ReportVendFailure never reached the server (pre-fix app build).
--
-- Evidence: AVF000195 logcat 2026-10-01 (commissioning + 25k cash order).
-- After line repair, applies the same terminal-order resolution as repair_stale_vending_orders.sql.
--
-- Usage:
--   psql "$DATABASE_URL" -v dry_run=1 -f scripts/ops/repair_stuck_vend_close_gap.sql
--   psql "$DATABASE_URL" -v dry_run=0 -f scripts/ops/repair_stuck_vend_close_gap.sql

\set ON_ERROR_STOP on

\echo '=== Preview: target orders before repair ==='
SELECT o.id,
       o.status,
       o.total_minor,
       m.code AS machine_code,
       o.created_at,
       array_agg(vs.state ORDER BY vs.line_sequence) AS line_states,
       array_agg(vs.slot_index ORDER BY vs.line_sequence) AS slot_indexes
FROM orders o
JOIN machines m ON m.id = o.machine_id
JOIN vend_sessions vs ON vs.order_id = o.id
WHERE o.id IN (
    '01a0f634-9563-7e11-b3a7-a5a3225c017a'::uuid,
    '01a0f5fa-5f54-7d77-8a04-0a0352a51a43'::uuid,
    '01a0f6c7-6651-7388-a1ed-5562088994cd'::uuid
)
GROUP BY o.id, o.status, o.total_minor, m.code, o.created_at
ORDER BY o.created_at;

\if :dry_run
\echo '=== DRY RUN: would apply line-level repairs (no writes) ==='
SELECT vs.order_id,
       vs.line_sequence,
       vs.slot_index,
       vs.state AS current_state,
       CASE
           WHEN vs.order_id = '01a0f5fa-5f54-7d77-8a04-0a0352a51a43'::uuid
                AND vs.line_sequence IN (1, 2) THEN 'success'
           WHEN vs.order_id = '01a0f634-9563-7e11-b3a7-a5a3225c017a'::uuid
                AND vs.line_sequence IN (1, 2) THEN 'success'
           WHEN vs.order_id = '01a0f6c7-6651-7388-a1ed-5562088994cd'::uuid
                AND vs.line_sequence IN (1, 2) THEN 'success'
           WHEN vs.order_id = '01a0f634-9563-7e11-b3a7-a5a3225c017a'::uuid
                AND vs.line_sequence = 3 THEN 'failed'
           WHEN vs.order_id = '01a0f6c7-6651-7388-a1ed-5562088994cd'::uuid
                AND vs.line_sequence = 3 THEN 'failed'
           ELSE vs.state
       END AS next_state,
       CASE
           WHEN vs.order_id = '01a0f634-9563-7e11-b3a7-a5a3225c017a'::uuid
                AND vs.line_sequence = 3 THEN 'LANE_VEND_AMBIGUOUS'
           WHEN vs.order_id = '01a0f6c7-6651-7388-a1ed-5562088994cd'::uuid
                AND vs.line_sequence = 3 THEN 'LANE_VEND_AMBIGUOUS'
           ELSE vs.failure_reason
       END AS next_failure_reason
FROM vend_sessions vs
WHERE vs.order_id IN (
    '01a0f634-9563-7e11-b3a7-a5a3225c017a'::uuid,
    '01a0f5fa-5f54-7d77-8a04-0a0352a51a43'::uuid,
    '01a0f6c7-6651-7388-a1ed-5562088994cd'::uuid
)
ORDER BY vs.order_id, vs.line_sequence;
\else
BEGIN;

\echo '=== Applying line-level repairs from logcat evidence ==='

UPDATE vend_sessions
SET
    state = 'success',
    failure_reason = NULL,
    completed_at = COALESCE(completed_at, now())
WHERE order_id = '01a0f5fa-5f54-7d77-8a04-0a0352a51a43'::uuid
  AND state = 'in_progress'
  AND line_sequence IN (1, 2);

UPDATE vend_sessions
SET
    state = 'success',
    failure_reason = NULL,
    completed_at = COALESCE(completed_at, now())
WHERE order_id = '01a0f634-9563-7e11-b3a7-a5a3225c017a'::uuid
  AND state = 'in_progress'
  AND line_sequence IN (1, 2);

UPDATE vend_sessions
SET
    state = 'failed',
    failure_reason = 'LANE_VEND_AMBIGUOUS',
    completed_at = COALESCE(completed_at, now())
WHERE order_id = '01a0f634-9563-7e11-b3a7-a5a3225c017a'::uuid
  AND state = 'in_progress'
  AND line_sequence = 3;

UPDATE vend_sessions
SET
    state = 'success',
    failure_reason = NULL,
    completed_at = COALESCE(completed_at, now())
WHERE order_id = '01a0f6c7-6651-7388-a1ed-5562088994cd'::uuid
  AND state = 'in_progress'
  AND line_sequence IN (1, 2);

UPDATE vend_sessions
SET
    state = 'failed',
    failure_reason = 'LANE_VEND_AMBIGUOUS',
    completed_at = COALESCE(completed_at, now())
WHERE order_id = '01a0f6c7-6651-7388-a1ed-5562088994cd'::uuid
  AND state = 'in_progress'
  AND line_sequence = 3;

\echo '=== Resolving terminal order status for all vending orders with terminal lines ==='

WITH terminal_sessions AS (
    SELECT
        o.id AS order_id,
        bool_or(vs.state = 'success') AS any_success,
        bool_or(vs.state = 'failed') AS any_failed,
        bool_and(vs.state IN ('success', 'failed', 'cancelled', 'skipped')) AS all_terminal
    FROM orders o
    JOIN vend_sessions vs ON vs.order_id = o.id
    WHERE o.status = 'vending'
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
       array_agg(vs.state ORDER BY vs.line_sequence) AS line_states,
       array_agg(COALESCE(vs.failure_reason, '') ORDER BY vs.line_sequence) AS failure_reasons
FROM orders o
JOIN machines m ON m.id = o.machine_id
JOIN vend_sessions vs ON vs.order_id = o.id
WHERE o.id IN (
    '01a0f634-9563-7e11-b3a7-a5a3225c017a'::uuid,
    '01a0f5fa-5f54-7d77-8a04-0a0352a51a43'::uuid,
    '01a0f6c7-6651-7388-a1ed-5562088994cd'::uuid
)
GROUP BY o.id, o.status, o.total_minor, m.code
ORDER BY o.created_at;
