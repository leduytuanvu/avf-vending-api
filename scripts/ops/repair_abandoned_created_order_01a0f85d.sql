-- Repair order stuck in `created` with pending vend lines when machine logcat proves
-- successful offline cash vend but server shell was abandoned (AVF000195, 2026-10-01 23:47).
--
-- Evidence: logcat PID 19374 — OFFLINE_VEND_SUCCESS A1+A2, VEND_SEQUENCE_END success=2,
-- server shell 01a0f85d-3f9a-758a-bdf6-0ede30e267f8 never received payment/vend close.
--
-- Usage:
--   psql "$DATABASE_URL" -v dry_run=1 -f scripts/ops/repair_abandoned_created_order_01a0f85d.sql
--   psql "$DATABASE_URL" -v dry_run=0 -f scripts/ops/repair_abandoned_created_order_01a0f85d.sql

\set ON_ERROR_STOP on

\echo '=== Preview: order 01a0f85d before repair ==='
SELECT o.id,
       o.status,
       o.total_minor,
       o.winning_payment_id,
       m.code AS machine_code,
       array_agg(vs.state ORDER BY vs.line_sequence) AS line_states,
       array_agg(vs.slot_index ORDER BY vs.line_sequence) AS slot_indexes
FROM orders o
JOIN machines m ON m.id = o.machine_id
LEFT JOIN vend_sessions vs ON vs.order_id = o.id
WHERE o.id = '01a0f85d-3f9a-758a-bdf6-0ede30e267f8'::uuid
GROUP BY o.id, o.status, o.total_minor, o.winning_payment_id, m.code;

\if :dry_run
\echo '=== DRY RUN: would insert captured cash payment + mark lines success + complete order ==='
SELECT '01a0f85d-3f9a-758a-bdf6-0ede30e267f8'::uuid AS order_id,
       'completed' AS next_status,
       15000::bigint AS payment_amount_minor,
       'cash' AS payment_provider,
       'captured' AS payment_state;
\else
BEGIN;

\echo '=== Insert repair payment (idempotent) ==='
INSERT INTO payments (
    id,
    order_id,
    provider,
    state,
    amount_minor,
    currency,
    idempotency_key,
    reconciliation_status,
    settlement_status,
    outcome
)
SELECT
    public.uuid_generate_v7(),
    '01a0f85d-3f9a-758a-bdf6-0ede30e267f8'::uuid,
    'cash',
    'captured',
    15000,
    o.currency,
    'ops-repair:01a0f85d:cash-captured',
    'not_required',
    'settled',
    'winner'
FROM orders o
WHERE o.id = '01a0f85d-3f9a-758a-bdf6-0ede30e267f8'::uuid
  AND NOT EXISTS (
      SELECT 1
      FROM payments p
      WHERE p.order_id = '01a0f85d-3f9a-758a-bdf6-0ede30e267f8'::uuid
        AND p.idempotency_key = 'ops-repair:01a0f85d:cash-captured'
  );

\echo '=== Mark vend lines success ==='
UPDATE vend_sessions
SET
    state = 'success',
    failure_reason = NULL,
    started_at = COALESCE(started_at, now()),
    completed_at = COALESCE(completed_at, now())
WHERE order_id = '01a0f85d-3f9a-758a-bdf6-0ede30e267f8'::uuid
  AND state IN ('pending', 'in_progress');

\echo '=== Complete order + link winning payment ==='
UPDATE orders o
SET
    status = 'completed',
    winning_payment_id = p.id,
    winning_claimed_at = COALESCE(o.winning_claimed_at, now()),
    updated_at = now()
FROM payments p
WHERE o.id = '01a0f85d-3f9a-758a-bdf6-0ede30e267f8'::uuid
  AND p.order_id = o.id
  AND p.idempotency_key = 'ops-repair:01a0f85d:cash-captured'
  AND o.status IN ('created', 'quoted', 'paid', 'vending');

COMMIT;
\endif

\echo '=== Preview: order 01a0f85d after repair ==='
SELECT o.id,
       o.status,
       o.total_minor,
       o.winning_payment_id,
       p.provider,
       p.state AS payment_state,
       array_agg(vs.state ORDER BY vs.line_sequence) AS line_states
FROM orders o
LEFT JOIN payments p ON p.id = o.winning_payment_id
LEFT JOIN vend_sessions vs ON vs.order_id = o.id
WHERE o.id = '01a0f85d-3f9a-758a-bdf6-0ede30e267f8'::uuid
GROUP BY o.id, o.status, o.total_minor, o.winning_payment_id, p.provider, p.state;
