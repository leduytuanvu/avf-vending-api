-- Repair orphan paid shell 01a12c1c-2be0-7c9c-aaf1-9c97cb374245 when logcat proves no vend
-- (payment captured, client abandonLocalOrder without CancelOrder).
--
-- BEFORE dry_run=0: confirm vend_sessions are pending/in_progress and no hardware vend for this id.
--
-- Usage:
--   psql "$DATABASE_URL" -v dry_run=1 -f scripts/ops/repair_session_20261012_2be0_stuck_paid_order.sql
--   psql "$DATABASE_URL" -v dry_run=0 -f scripts/ops/repair_session_20261012_2be0_stuck_paid_order.sql

\set ON_ERROR_STOP on

\echo '=== Preview target order ==='
SELECT o.id,
       o.status,
       o.total_minor,
       m.code AS machine_code,
       array_agg(vs.line_sequence ORDER BY vs.line_sequence) AS line_sequences,
       array_agg(vs.state ORDER BY vs.line_sequence) AS line_states
FROM orders o
JOIN machines m ON m.id = o.machine_id
LEFT JOIN vend_sessions vs ON vs.order_id = o.id
WHERE o.id = '01a12c1c-2be0-7c9c-aaf1-9c97cb374245'::uuid
GROUP BY o.id, o.status, o.total_minor, m.code;

\if :dry_run
\echo '=== DRY RUN: would cancel paid shell (no vend evidence) ==='
SELECT '01a12c1c-2be0-7c9c-aaf1-9c97cb374245'::uuid AS order_id,
       'cancelled' AS next_status;
\else
BEGIN;

UPDATE vend_sessions
SET
    state = 'failed',
    failure_reason = COALESCE(failure_reason, 'checkout_abandoned_no_vend (logcat 2026-10-12)'),
    completed_at = COALESCE(completed_at, now())
WHERE order_id = '01a12c1c-2be0-7c9c-aaf1-9c97cb374245'::uuid
  AND state NOT IN ('success', 'failed', 'cancelled', 'skipped');

UPDATE orders
SET
    status = 'cancelled',
    updated_at = now()
WHERE id = '01a12c1c-2be0-7c9c-aaf1-9c97cb374245'::uuid
  AND status IN ('paid', 'vending', 'created', 'quoted');

COMMIT;
\endif
