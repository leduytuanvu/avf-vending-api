-- Repair order stuck in `vending` / `in_progress` when machine dispensed successfully but
-- ConfirmVendSuccess failed with no machine_slot_config (AVF000195, QR→cash, 2026-10-02).
--
-- Evidence: order 01a0f8b7-32e0-7018-857d-8e998e651cc8 — A9+A10 physical success,
-- GRPC_CONFIRM_VEND_ERROR slot_index=9/10.
--
-- Usage:
--   psql "$DATABASE_URL" -v dry_run=1 -f scripts/ops/repair_stuck_vend_close_01a0f8b7.sql
--   psql "$DATABASE_URL" -v dry_run=0 -f scripts/ops/repair_stuck_vend_close_01a0f8b7.sql

\set ON_ERROR_STOP on

\echo '=== Preview: order 01a0f8b7 before repair ==='
SELECT o.id,
       o.status,
       o.total_minor,
       o.winning_payment_id,
       m.code AS machine_code,
       p.provider,
       p.state AS payment_state,
       vs.line_sequence,
       vs.slot_index,
       vs.state AS vend_state,
       vs.failure_reason,
       pr.name AS product_name
FROM orders o
JOIN machines m ON m.id = o.machine_id
LEFT JOIN payments p ON p.order_id = o.id
JOIN vend_sessions vs ON vs.order_id = o.id
JOIN products pr ON pr.id = vs.product_id
WHERE o.id = '01a0f8b7-32e0-7018-857d-8e998e651cc8'::uuid
ORDER BY vs.line_sequence;

\if :dry_run
\echo '=== DRY RUN: would mark both lines success + complete order ==='
SELECT vs.order_id,
       vs.line_sequence,
       vs.slot_index,
       vs.state AS current_state,
       'success' AS next_state
FROM vend_sessions vs
WHERE vs.order_id = '01a0f8b7-32e0-7018-857d-8e998e651cc8'::uuid
  AND vs.state = 'in_progress'
ORDER BY vs.line_sequence;
\else
BEGIN;

\echo '=== Mark vend lines success (machine confirmed physical dispense) ==='
UPDATE vend_sessions
SET
    state = 'success',
    failure_reason = NULL,
    completed_at = COALESCE(completed_at, now())
WHERE order_id = '01a0f8b7-32e0-7018-857d-8e998e651cc8'::uuid
  AND state = 'in_progress';

\echo '=== Complete order ==='
UPDATE orders
SET
    status = 'completed',
    updated_at = now()
WHERE id = '01a0f8b7-32e0-7018-857d-8e998e651cc8'::uuid
  AND status = 'vending';

COMMIT;
\endif

\echo '=== Preview: order 01a0f8b7 after repair ==='
SELECT o.id,
       o.status,
       o.total_minor,
       m.code AS machine_code,
       array_agg(vs.state ORDER BY vs.line_sequence) AS line_states,
       array_agg(COALESCE(vs.failure_reason, '') ORDER BY vs.line_sequence) AS failure_reasons
FROM orders o
JOIN machines m ON m.id = o.machine_id
JOIN vend_sessions vs ON vs.order_id = o.id
WHERE o.id = '01a0f8b7-32e0-7018-857d-8e998e651cc8'::uuid
GROUP BY o.id, o.status, o.total_minor, m.code;
