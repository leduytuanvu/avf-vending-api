-- Repair order 01a0facc-e10b-7720-99ee-b3ac7b66a705 (AVF000195):
-- 2-line cart A9 x2; both lines failed locally but app skipped deferred failure flush
-- (local_first_offline), leaving server order created / vend lines pending.
--
-- Evidence: logcat 2026-10-02 11:08-11:09, client order 75ff4504-b98e-4c00-91ea-8ae626e2f409.
--
-- Usage:
--   psql "$DATABASE_URL" -v dry_run=1 -f scripts/ops/repair_order_01a0facc_all_failed.sql
--   psql "$DATABASE_URL" -v dry_run=0 -f scripts/ops/repair_order_01a0facc_all_failed.sql

\set ON_ERROR_STOP on

\echo '=== Preview: order 01a0facc before repair ==='
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
WHERE o.id = '01a0facc-e10b-7720-99ee-b3ac7b66a705'::uuid
GROUP BY o.id, o.status, o.total_minor, m.code;

\if :dry_run
\echo '=== DRY RUN: would mark both lines failed and order failed ==='
SELECT vs.line_sequence,
       vs.slot_index,
       vs.state AS current_state,
       'failed' AS next_state,
       'TCN no-drop / LANE_VEND_AMBIGUOUS' AS next_failure_reason
FROM vend_sessions vs
WHERE vs.order_id = '01a0facc-e10b-7720-99ee-b3ac7b66a705'::uuid
ORDER BY vs.line_sequence;
\else
BEGIN;

\echo '=== Repair vend sessions (lines 1 and 2) ==='
UPDATE vend_sessions
SET
    state = 'failed',
    failure_reason = 'TCN no-drop / LANE_VEND_AMBIGUOUS',
    completed_at = COALESCE(completed_at, now())
WHERE order_id = '01a0facc-e10b-7720-99ee-b3ac7b66a705'::uuid
  AND line_sequence IN (1, 2);

\echo '=== Set order failed ==='
UPDATE orders
SET status = 'failed'
WHERE id = '01a0facc-e10b-7720-99ee-b3ac7b66a705'::uuid
  AND status IN ('created', 'paid', 'vending');

COMMIT;

\echo '=== After repair ==='
SELECT o.id, o.status,
       array_agg(vs.line_sequence ORDER BY vs.line_sequence) AS line_sequences,
       array_agg(vs.state ORDER BY vs.line_sequence) AS line_states
FROM orders o
JOIN vend_sessions vs ON vs.order_id = o.id
WHERE o.id = '01a0facc-e10b-7720-99ee-b3ac7b66a705'::uuid
GROUP BY o.id, o.status;
\endif
