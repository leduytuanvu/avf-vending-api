-- Template: repair orders stuck in paid/vending when machine already dispensed (or failed) locally
-- but ConfirmVendSuccess/ReportVendFailure never reached terminal order status.
--
-- BEFORE running with dry_run=0:
--   1. Confirm hardware / logcat evidence for each order_id.
--   2. Inspect vend_sessions line states (in_progress vs success/failed).
--
-- Usage:
--   psql "$DATABASE_URL" -v dry_run=1 -v order_id='<uuid>' -f scripts/ops/repair_stuck_paid_vending_template.sql
--   psql "$DATABASE_URL" -v dry_run=0 -v order_id='<uuid>' -f scripts/ops/repair_stuck_paid_vending_template.sql

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
JOIN vend_sessions vs ON vs.order_id = o.id
WHERE o.id = :'order_id'::uuid
GROUP BY o.id, o.status, o.total_minor, m.code;

\if :dry_run
\echo '=== DRY RUN: no writes ==='
\else
BEGIN;

UPDATE vend_sessions
SET
    state = 'success',
    failure_reason = NULL,
    completed_at = COALESCE(completed_at, now())
WHERE order_id = :'order_id'::uuid
  AND state = 'in_progress';

UPDATE orders
SET
    status = 'completed',
    updated_at = now()
WHERE id = :'order_id'::uuid
  AND status IN ('paid', 'vending');

COMMIT;
\endif
