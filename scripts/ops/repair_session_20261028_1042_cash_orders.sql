-- Repair 2026-10-28 03:42Z–03:44Z cash session on AVF000195 (VN 10:42–10:44).
-- Hardware: B1 cash vend OK (local 67af9332 → server 8d6f); QR then cash B1 TCN_NO_DROP (b2e2, wallet refunded);
-- leftover QR shells abandoned. Do not INSERT cash_acceptance_events. Do not touch 01:37 repairs.
-- 15k paid 6b03 stays paid unless a later vend-success proof exists (not applied here).
--
-- Orders:
--   01a1809a-8d6f-73ca-b3d3-76d7b96da51b  cash captured, vend slot 11 pending → completed + success
--   01a1809a-b2e2-73cc-86cf-88b9169f558a  created, no payment, vend pending, TCN_NO_DROP → failed
--   01a1809a-96f0-78c3-ad71-16cf6bf5c0c3 leftover QR created → cancelled
--   01a1809b-27cb-7130-9ab2-9206ccd20451 leftover QR created → cancelled
--   01a1809b-5584-7378-a4f6-280ba653afcf leftover QR created → cancelled
--   01a1809b-6b03-79e3-8df9-e98019535cc7 15k cash paid → leave paid
--
-- Usage:
--   psql "$DATABASE_URL" -v dry_run=1 -f scripts/ops/repair_session_20261028_1042_cash_orders.sql
--   psql "$DATABASE_URL" -v dry_run=0 -f scripts/ops/repair_session_20261028_1042_cash_orders.sql

\set ON_ERROR_STOP on

\if :{?dry_run}
\else
\set dry_run 1
\endif

\echo '=== Preview: session 10:42 orders before repair ==='
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
    '01a1809a-8d6f-73ca-b3d3-76d7b96da51b'::uuid,
    '01a1809a-b2e2-73cc-86cf-88b9169f558a'::uuid,
    '01a1809a-96f0-78c3-ad71-16cf6bf5c0c3'::uuid,
    '01a1809b-27cb-7130-9ab2-9206ccd20451'::uuid,
    '01a1809b-5584-7378-a4f6-280ba653afcf'::uuid,
    '01a1809b-6b03-79e3-8df9-e98019535cc7'::uuid
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
    '01a1809a-8d6f-73ca-b3d3-76d7b96da51b'::uuid,
    '01a1809a-b2e2-73cc-86cf-88b9169f558a'::uuid,
    '01a1809a-96f0-78c3-ad71-16cf6bf5c0c3'::uuid,
    '01a1809b-27cb-7130-9ab2-9206ccd20451'::uuid,
    '01a1809b-5584-7378-a4f6-280ba653afcf'::uuid,
    '01a1809b-6b03-79e3-8df9-e98019535cc7'::uuid
)
ORDER BY p.created_at;

\if :dry_run
\echo '=== DRY RUN: cash vend pending → success, paid → completed ==='
SELECT vs.order_id,
       vs.line_sequence,
       vs.slot_index,
       vs.state AS current_state,
       'success' AS next_state
FROM vend_sessions vs
WHERE vs.order_id = '01a1809a-8d6f-73ca-b3d3-76d7b96da51b'::uuid
  AND vs.slot_index = 11
  AND vs.state IN ('pending', 'in_progress');

SELECT o.id,
       o.status AS current_status,
       'completed' AS next_status
FROM orders o
WHERE o.id = '01a1809a-8d6f-73ca-b3d3-76d7b96da51b'::uuid
  AND o.status IN ('paid', 'vending');

\echo '=== DRY RUN: no-drop cash order → vend failed TCN_NO_DROP, order failed (no cash insert) ==='
SELECT vs.order_id,
       vs.line_sequence,
       vs.slot_index,
       vs.state AS current_state,
       'failed' AS next_state,
       'TCN_NO_DROP' AS next_failure_reason
FROM vend_sessions vs
WHERE vs.order_id = '01a1809a-b2e2-73cc-86cf-88b9169f558a'::uuid
  AND vs.state IN ('pending', 'in_progress');

SELECT o.id,
       o.status AS current_status,
       'failed' AS next_status
FROM orders o
WHERE o.id = '01a1809a-b2e2-73cc-86cf-88b9169f558a'::uuid
  AND o.status IN ('created', 'quoted', 'paid', 'vending');

\echo '=== DRY RUN: leftover QR shells created → cancelled (uncaptured only) ==='
SELECT o.id,
       o.status AS current_status,
       'cancelled' AS next_status
FROM orders o
WHERE o.id IN (
    '01a1809a-96f0-78c3-ad71-16cf6bf5c0c3'::uuid,
    '01a1809b-27cb-7130-9ab2-9206ccd20451'::uuid,
    '01a1809b-5584-7378-a4f6-280ba653afcf'::uuid
)
  AND o.status IN ('created', 'quoted')
  AND NOT EXISTS (
      SELECT 1
      FROM payments p
      WHERE p.order_id = o.id
        AND p.state IN ('captured', 'partially_refunded', 'refunded')
  );

SELECT p.order_id,
       p.id,
       p.provider,
       p.state AS current_state,
       'canceled' AS next_state
FROM payments p
WHERE p.order_id IN (
    '01a1809a-96f0-78c3-ad71-16cf6bf5c0c3'::uuid,
    '01a1809b-27cb-7130-9ab2-9206ccd20451'::uuid,
    '01a1809b-5584-7378-a4f6-280ba653afcf'::uuid
)
  AND p.state NOT IN ('captured', 'partially_refunded', 'refunded', 'canceled');

\echo '=== DRY RUN: 15k 6b03 left paid (no vend-success proof in this repair) ==='
SELECT o.id,
       o.status,
       o.total_minor
FROM orders o
WHERE o.id = '01a1809b-6b03-79e3-8df9-e98019535cc7'::uuid;
\else
BEGIN;

\echo '=== 8d6f: mark slot 11 vend success ==='
UPDATE vend_sessions
SET
    state = 'success',
    failure_reason = NULL,
    started_at = COALESCE(started_at, now()),
    completed_at = COALESCE(completed_at, now())
WHERE order_id = '01a1809a-8d6f-73ca-b3d3-76d7b96da51b'::uuid
  AND slot_index = 11
  AND state IN ('pending', 'in_progress');

\echo '=== 8d6f: paid → completed ==='
UPDATE orders
SET
    status = 'completed',
    updated_at = now()
WHERE id = '01a1809a-8d6f-73ca-b3d3-76d7b96da51b'::uuid
  AND status IN ('paid', 'vending');

\echo '=== b2e2: vend → failed TCN_NO_DROP (no cash capture; wallet already refunded) ==='
UPDATE vend_sessions
SET
    state = 'failed',
    failure_reason = 'TCN_NO_DROP',
    started_at = COALESCE(started_at, now()),
    completed_at = COALESCE(completed_at, now())
WHERE order_id = '01a1809a-b2e2-73cc-86cf-88b9169f558a'::uuid
  AND state IN ('pending', 'in_progress');

\echo '=== b2e2: created → failed ==='
UPDATE orders
SET
    status = 'failed',
    updated_at = now()
WHERE id = '01a1809a-b2e2-73cc-86cf-88b9169f558a'::uuid
  AND status IN ('created', 'quoted', 'paid', 'vending');

\echo '=== leftover QR payments: uncaptured → canceled ==='
UPDATE payments
SET
    state = 'canceled',
    outcome = 'superseded',
    updated_at = now()
WHERE order_id IN (
    '01a1809a-96f0-78c3-ad71-16cf6bf5c0c3'::uuid,
    '01a1809b-27cb-7130-9ab2-9206ccd20451'::uuid,
    '01a1809b-5584-7378-a4f6-280ba653afcf'::uuid
)
  AND state NOT IN ('captured', 'partially_refunded', 'refunded', 'canceled');

\echo '=== leftover QR: created → cancelled ==='
UPDATE orders
SET
    status = 'cancelled',
    updated_at = now()
WHERE id IN (
    '01a1809a-96f0-78c3-ad71-16cf6bf5c0c3'::uuid,
    '01a1809b-27cb-7130-9ab2-9206ccd20451'::uuid,
    '01a1809b-5584-7378-a4f6-280ba653afcf'::uuid
)
  AND status IN ('created', 'quoted')
  AND NOT EXISTS (
      SELECT 1
      FROM payments p
      WHERE p.order_id = orders.id
        AND p.state IN ('captured', 'partially_refunded', 'refunded')
  );

COMMIT;
\endif

\echo '=== Preview: session 10:42 orders after repair ==='
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
    '01a1809a-8d6f-73ca-b3d3-76d7b96da51b'::uuid,
    '01a1809a-b2e2-73cc-86cf-88b9169f558a'::uuid,
    '01a1809a-96f0-78c3-ad71-16cf6bf5c0c3'::uuid,
    '01a1809b-27cb-7130-9ab2-9206ccd20451'::uuid,
    '01a1809b-5584-7378-a4f6-280ba653afcf'::uuid,
    '01a1809b-6b03-79e3-8df9-e98019535cc7'::uuid
)
GROUP BY o.id, o.status, o.total_minor, o.winning_payment_id, p.provider, p.state, p.outcome
ORDER BY o.created_at;
