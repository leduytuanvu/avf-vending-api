-- Repair order 01a0f8db-3ad8-7d32-b060-0c15665b48a1 (AVF000195):
-- 3-line cart B3 + B4 + B4; line 3 physically failed but ConfirmVendSuccess on line 2
-- (duplicate slot_index=14) marked all B4 lines success and closed order completed.
--
-- Evidence: logcat 2026-10-02 02:05-02:06, VEND_SEQUENCE_END partial success=2 failed=1.
--
-- Usage:
--   psql "$DATABASE_URL" -v dry_run=1 -f scripts/ops/repair_order_01a0f8db_duplicate_slot.sql
--   psql "$DATABASE_URL" -v dry_run=0 -f scripts/ops/repair_order_01a0f8db_duplicate_slot.sql

\set ON_ERROR_STOP on

\echo '=== Preview: order 01a0f8db before repair ==='
SELECT o.id,
       o.status,
       o.total_minor,
       m.code AS machine_code,
       array_agg(vs.line_sequence ORDER BY vs.line_sequence) AS line_sequences,
       array_agg(vs.slot_index ORDER BY vs.line_sequence) AS slot_indexes,
       array_agg(vs.state ORDER BY vs.line_sequence) AS line_states,
       array_agg(vs.failure_reason ORDER BY vs.line_sequence) AS failure_reasons
FROM orders o
JOIN machines m ON m.id = o.machine_id
JOIN vend_sessions vs ON vs.order_id = o.id
WHERE o.id = '01a0f8db-3ad8-7d32-b060-0c15665b48a1'::uuid
GROUP BY o.id, o.status, o.total_minor, m.code;

\if :dry_run
\echo '=== DRY RUN: would apply repairs (no writes) ==='
SELECT vs.line_sequence,
       vs.slot_index,
       vs.state AS current_state,
       CASE WHEN vs.line_sequence = 3 THEN 'failed' ELSE vs.state END AS next_state,
       CASE WHEN vs.line_sequence = 3 THEN 'TCN no-drop / LANE_VEND_AMBIGUOUS' ELSE vs.failure_reason END AS next_failure_reason
FROM vend_sessions vs
WHERE vs.order_id = '01a0f8db-3ad8-7d32-b060-0c15665b48a1'::uuid
ORDER BY vs.line_sequence;
\else
BEGIN;

\echo '=== Repair line 3 vend session ==='
UPDATE vend_sessions
SET
    state = 'failed',
    failure_reason = 'TCN no-drop / LANE_VEND_AMBIGUOUS',
    completed_at = COALESCE(completed_at, now())
WHERE order_id = '01a0f8db-3ad8-7d32-b060-0c15665b48a1'::uuid
  AND line_sequence = 3;

\echo '=== Set order partially_completed ==='
UPDATE orders
SET status = 'partially_completed'
WHERE id = '01a0f8db-3ad8-7d32-b060-0c15665b48a1'::uuid
  AND status IN ('completed', 'vending');

COMMIT;

\echo '=== After repair ==='
SELECT o.id, o.status,
       array_agg(vs.line_sequence ORDER BY vs.line_sequence) AS line_sequences,
       array_agg(vs.state ORDER BY vs.line_sequence) AS line_states
FROM orders o
JOIN vend_sessions vs ON vs.order_id = o.id
WHERE o.id = '01a0f8db-3ad8-7d32-b060-0c15665b48a1'::uuid
GROUP BY o.id, o.status;
\endif
