-- Repair multi-line same-slot orders stuck in vending when line 1 failed but line 2+
-- was skipped locally and never received StartVend on the server (vend session pending).
--
-- Set order_id and line_sequence for the pending vend line before running.
--
-- Usage:
--   psql "$DATABASE_URL" -v dry_run=1 -v order_id='YOUR-ORDER-UUID' -v pending_line_sequence=2 \
--     -f scripts/ops/repair_same_slot_skipped_line_pending.sql
--   psql "$DATABASE_URL" -v dry_run=0 -v order_id='YOUR-ORDER-UUID' -v pending_line_sequence=2 \
--     -f scripts/ops/repair_same_slot_skipped_line_pending.sql

\set ON_ERROR_STOP on

\echo '=== Preview: order before repair ==='
SELECT o.id,
       o.status,
       m.code AS machine_code,
       array_agg(vs.line_sequence ORDER BY vs.line_sequence) AS line_sequences,
       array_agg(vs.state ORDER BY vs.line_sequence) AS line_states,
       array_agg(vs.failure_reason ORDER BY vs.line_sequence) AS failure_reasons
FROM orders o
JOIN machines m ON m.id = o.machine_id
JOIN vend_sessions vs ON vs.order_id = o.id
WHERE o.id = :'order_id'::uuid
GROUP BY o.id, o.status, m.code;

\if :dry_run
\echo '=== DRY RUN: would fail pending line and close order when no open vend lines ==='
SELECT vs.line_sequence,
       vs.state AS current_state,
       CASE
           WHEN vs.line_sequence = :pending_line_sequence::int THEN 'failed'
           ELSE vs.state
       END AS next_state
FROM vend_sessions vs
WHERE vs.order_id = :'order_id'::uuid
ORDER BY vs.line_sequence;
\else
BEGIN;

UPDATE vend_sessions
SET
    state = 'failed',
    failure_reason = COALESCE(failure_reason, 'TCN no-drop / LANE_VEND_AMBIGUOUS'),
    completed_at = COALESCE(completed_at, now())
WHERE order_id = :'order_id'::uuid
  AND line_sequence = :pending_line_sequence::int
  AND state IN ('pending', 'in_progress');

UPDATE orders
SET status = 'failed'
WHERE id = :'order_id'::uuid
  AND status = 'vending'
  AND NOT EXISTS (
      SELECT 1
      FROM vend_sessions vs
      WHERE vs.order_id = :'order_id'::uuid
        AND vs.state IN ('pending', 'in_progress')
  );

COMMIT;

\echo '=== After repair ==='
SELECT o.id, o.status,
       array_agg(vs.line_sequence ORDER BY vs.line_sequence) AS line_sequences,
       array_agg(vs.state ORDER BY vs.line_sequence) AS line_states
FROM orders o
JOIN vend_sessions vs ON vs.order_id = o.id
WHERE o.id = :'order_id'::uuid
GROUP BY o.id, o.status;
\endif
