-- Repair order 01a0fafb-ecb1-7d5f-a06c-8405377516da (client 9bfd1c9c-d458-4b9b-8c55-5f83b62e8214):
-- 2× B5 same slot; line 1 dispensed, line 2 no-drop; server stuck created / pending lines.
--
-- Usage:
--   psql "$DATABASE_URL" -v dry_run=1 -f scripts/ops/repair_order_01a0fafb_partial_same_slot.sql
--   psql "$DATABASE_URL" -v dry_run=0 -f scripts/ops/repair_order_01a0fafb_partial_same_slot.sql

\set ON_ERROR_STOP on

\echo '=== Preview: order 01a0fafb before repair ==='
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
WHERE o.id = '01a0fafb-ecb1-7d5f-a06c-8405377516da'::uuid
GROUP BY o.id, o.status, o.total_minor, m.code;

\if :dry_run
\echo '=== DRY RUN: would mark line 1 success, line 2 failed, order partial/completed ==='
SELECT vs.line_sequence,
       vs.state AS current_state,
       CASE
           WHEN vs.line_sequence = 1 THEN 'success'
           WHEN vs.line_sequence = 2 THEN 'failed'
           ELSE vs.state
       END AS next_state
FROM vend_sessions vs
WHERE vs.order_id = '01a0fafb-ecb1-7d5f-a06c-8405377516da'::uuid
ORDER BY vs.line_sequence;
\else
BEGIN;

UPDATE vend_sessions
SET
    state = 'success',
    completed_at = COALESCE(completed_at, now())
WHERE order_id = '01a0fafb-ecb1-7d5f-a06c-8405377516da'::uuid
  AND line_sequence = 1
  AND state IN ('pending', 'in_progress');

UPDATE vend_sessions
SET
    state = 'failed',
    failure_reason = COALESCE(failure_reason, 'TCN no-drop'),
    completed_at = COALESCE(completed_at, now())
WHERE order_id = '01a0fafb-ecb1-7d5f-a06c-8405377516da'::uuid
  AND line_sequence = 2
  AND state IN ('pending', 'in_progress');

UPDATE orders
SET status = 'partial'
WHERE id = '01a0fafb-ecb1-7d5f-a06c-8405377516da'::uuid
  AND status IN ('created', 'vending', 'paid')
  AND EXISTS (
      SELECT 1 FROM vend_sessions vs
      WHERE vs.order_id = '01a0fafb-ecb1-7d5f-a06c-8405377516da'::uuid AND vs.state = 'success'
  )
  AND EXISTS (
      SELECT 1 FROM vend_sessions vs
      WHERE vs.order_id = '01a0fafb-ecb1-7d5f-a06c-8405377516da'::uuid AND vs.state = 'failed'
  );

COMMIT;

\echo '=== After repair ==='
SELECT o.id, o.status,
       array_agg(vs.line_sequence ORDER BY vs.line_sequence) AS line_sequences,
       array_agg(vs.state ORDER BY vs.line_sequence) AS line_states
FROM orders o
JOIN vend_sessions vs ON vs.order_id = o.id
WHERE o.id = '01a0fafb-ecb1-7d5f-a06c-8405377516da'::uuid
GROUP BY o.id, o.status;
\endif
