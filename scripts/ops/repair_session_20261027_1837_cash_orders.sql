-- Repair 2026-10-27 18:36Z–18:41Z cash session on AVF000195.
-- Hardware completed B1 (slot 11) and A10 (slot 10); leftover QR shell was abandoned.
-- Do not invent A1 (b8eb380c) and do not INSERT cash_acceptance_events.
--
-- Orders:
--   01a17ea7-63f8-7eb3-9eb8-7c35a27650b5  B1 paid + cash captured, vend slot 11 pending → completed
--   01a17ea7-a2c6-7509-a53a-cd8cd519840b  A10 created, no payment, vend slot 10 pending → insert 10k cash + completed
--   01a17ea7-8439-7c04-948b-fdda8f0160e6  leftover QR created → cancelled
--
-- Usage:
--   psql "$DATABASE_URL" -v dry_run=1 -f scripts/ops/repair_session_20261027_1837_cash_orders.sql
--   psql "$DATABASE_URL" -v dry_run=0 -f scripts/ops/repair_session_20261027_1837_cash_orders.sql

\set ON_ERROR_STOP on

\if :{?dry_run}
\else
\set dry_run 1
\endif

\echo '=== Preview: session 01:37 orders before repair ==='
SELECT o.id,
       o.status,
       o.total_minor,
       o.winning_payment_id,
       m.code AS machine_code,
       o.created_at,
       array_agg(vs.state ORDER BY vs.line_sequence) AS line_states,
       array_agg(vs.slot_index ORDER BY vs.line_sequence) AS slot_indexes
FROM orders o
JOIN machines m ON m.id = o.machine_id
LEFT JOIN vend_sessions vs ON vs.order_id = o.id
WHERE o.id IN (
    '01a17ea7-63f8-7eb3-9eb8-7c35a27650b5'::uuid,
    '01a17ea7-a2c6-7509-a53a-cd8cd519840b'::uuid,
    '01a17ea7-8439-7c04-948b-fdda8f0160e6'::uuid
)
GROUP BY o.id, o.status, o.total_minor, o.winning_payment_id, m.code, o.created_at
ORDER BY o.created_at;

\echo '=== Preview: payments before repair ==='
SELECT p.order_id,
       p.id,
       p.provider,
       p.state,
       p.outcome,
       p.amount_minor,
       p.idempotency_key
FROM payments p
WHERE p.order_id IN (
    '01a17ea7-63f8-7eb3-9eb8-7c35a27650b5'::uuid,
    '01a17ea7-a2c6-7509-a53a-cd8cd519840b'::uuid,
    '01a17ea7-8439-7c04-948b-fdda8f0160e6'::uuid
)
ORDER BY p.created_at;

\if :dry_run
\echo '=== DRY RUN: B1 vend pending → success, paid → completed ==='
SELECT vs.order_id,
       vs.line_sequence,
       vs.slot_index,
       vs.state AS current_state,
       'success' AS next_state
FROM vend_sessions vs
WHERE vs.order_id = '01a17ea7-63f8-7eb3-9eb8-7c35a27650b5'::uuid
  AND vs.slot_index = 11
  AND vs.state IN ('pending', 'in_progress');

SELECT o.id,
       o.status AS current_status,
       'completed' AS next_status
FROM orders o
WHERE o.id = '01a17ea7-63f8-7eb3-9eb8-7c35a27650b5'::uuid
  AND o.status IN ('paid', 'vending');

\echo '=== DRY RUN: A10 insert 10k cash captured + vend success + complete ==='
SELECT '01a17ea7-a2c6-7509-a53a-cd8cd519840b'::uuid AS order_id,
       'completed' AS next_status,
       10000::bigint AS payment_amount_minor,
       'cash' AS payment_provider,
       'captured' AS payment_state,
       'winner' AS payment_outcome;

SELECT vs.order_id,
       vs.line_sequence,
       vs.slot_index,
       vs.state AS current_state,
       'success' AS next_state
FROM vend_sessions vs
WHERE vs.order_id = '01a17ea7-a2c6-7509-a53a-cd8cd519840b'::uuid
  AND vs.slot_index = 10
  AND vs.state IN ('pending', 'in_progress');

\echo '=== DRY RUN: leftover QR created → cancelled ==='
SELECT o.id,
       o.status AS current_status,
       'cancelled' AS next_status
FROM orders o
WHERE o.id = '01a17ea7-8439-7c04-948b-fdda8f0160e6'::uuid
  AND o.status IN ('created', 'quoted');
\else
BEGIN;

\echo '=== B1: mark slot 11 vend success ==='
UPDATE vend_sessions
SET
    state = 'success',
    failure_reason = NULL,
    started_at = COALESCE(started_at, now()),
    completed_at = COALESCE(completed_at, now())
WHERE order_id = '01a17ea7-63f8-7eb3-9eb8-7c35a27650b5'::uuid
  AND slot_index = 11
  AND state IN ('pending', 'in_progress');

\echo '=== B1: paid → completed ==='
UPDATE orders
SET
    status = 'completed',
    updated_at = now()
WHERE id = '01a17ea7-63f8-7eb3-9eb8-7c35a27650b5'::uuid
  AND status IN ('paid', 'vending');

\echo '=== A10: insert repair cash payment (idempotent) ==='
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
    '01a17ea7-a2c6-7509-a53a-cd8cd519840b'::uuid,
    'cash',
    'captured',
    10000,
    o.currency,
    'ops-repair:01a17ea7-a2c6:cash-captured',
    'not_required',
    'settled',
    'winner'
FROM orders o
WHERE o.id = '01a17ea7-a2c6-7509-a53a-cd8cd519840b'::uuid
  AND NOT EXISTS (
      SELECT 1
      FROM payments p
      WHERE p.order_id = '01a17ea7-a2c6-7509-a53a-cd8cd519840b'::uuid
        AND p.idempotency_key = 'ops-repair:01a17ea7-a2c6:cash-captured'
  );

\echo '=== A10: mark slot 10 vend success ==='
UPDATE vend_sessions
SET
    state = 'success',
    failure_reason = NULL,
    started_at = COALESCE(started_at, now()),
    completed_at = COALESCE(completed_at, now())
WHERE order_id = '01a17ea7-a2c6-7509-a53a-cd8cd519840b'::uuid
  AND slot_index = 10
  AND state IN ('pending', 'in_progress');

\echo '=== A10: created → completed + link winning payment ==='
UPDATE orders o
SET
    status = 'completed',
    winning_payment_id = p.id,
    winning_claimed_at = COALESCE(o.winning_claimed_at, now()),
    updated_at = now()
FROM payments p
WHERE o.id = '01a17ea7-a2c6-7509-a53a-cd8cd519840b'::uuid
  AND p.order_id = o.id
  AND p.idempotency_key = 'ops-repair:01a17ea7-a2c6:cash-captured'
  AND o.status IN ('created', 'quoted', 'paid', 'vending');

\echo '=== leftover QR: created → cancelled ==='
UPDATE orders
SET
    status = 'cancelled',
    updated_at = now()
WHERE id = '01a17ea7-8439-7c04-948b-fdda8f0160e6'::uuid
  AND status IN ('created', 'quoted');

COMMIT;
\endif

\echo '=== Preview: session 01:37 orders after repair ==='
SELECT o.id,
       o.status,
       o.total_minor,
       o.winning_payment_id,
       p.provider,
       p.state AS payment_state,
       p.outcome,
       array_agg(vs.state ORDER BY vs.line_sequence) AS line_states,
       array_agg(vs.slot_index ORDER BY vs.line_sequence) AS slot_indexes
FROM orders o
LEFT JOIN payments p ON p.id = o.winning_payment_id
LEFT JOIN vend_sessions vs ON vs.order_id = o.id
WHERE o.id IN (
    '01a17ea7-63f8-7eb3-9eb8-7c35a27650b5'::uuid,
    '01a17ea7-a2c6-7509-a53a-cd8cd519840b'::uuid,
    '01a17ea7-8439-7c04-948b-fdda8f0160e6'::uuid
)
GROUP BY o.id, o.status, o.total_minor, o.winning_payment_id, p.provider, p.state, p.outcome
ORDER BY o.created_at;
