-- Repair AVF000195 session 2026-11-01 02:39–02:44 ICT stuck after offline_sequence
-- collision with Oct-24 poison cash slots (1176/1186/1187/1189).
-- Hardware evidence from logcat; do not INSERT cash or delete poison ledger rows.
--
--   ff97b6ab-8d85-4f38-afc3-c46513f6292a  B9 slot 19 vend success, cash captured, order vending
--   b8239908-2466-40be-bca0-0fea72765d7d  C5 slot 25 TCN_NO_DROP + C6 slot 26 success, cash captured, order paid
--
-- Usage:
--   psql "$DATABASE_URL" -v dry_run=1 -f scripts/ops/repair_session_20261101_0239_stuck_orders.sql
--   psql "$DATABASE_URL" -v dry_run=0 -f scripts/ops/repair_session_20261101_0239_stuck_orders.sql

\set ON_ERROR_STOP on

\if :{?dry_run}
\else
\set dry_run 1
\endif

\echo '=== Preview: 2026-11-01 02:39 stuck orders before repair ==='
SELECT o.id,
       o.status,
       o.total_minor,
       o.winning_payment_id,
       m.code AS machine_code,
       p.provider,
       p.state AS payment_state,
       p.outcome,
       array_agg(vs.line_sequence ORDER BY vs.line_sequence) AS line_sequences,
       array_agg(vs.slot_index ORDER BY vs.line_sequence) AS slot_indexes,
       array_agg(vs.state ORDER BY vs.line_sequence) AS line_states
FROM orders o
JOIN machines m ON m.id = o.machine_id
LEFT JOIN payments p ON p.id = o.winning_payment_id
LEFT JOIN vend_sessions vs ON vs.order_id = o.id
WHERE o.id IN (
    'ff97b6ab-8d85-4f38-afc3-c46513f6292a'::uuid,
    'b8239908-2466-40be-bca0-0fea72765d7d'::uuid
)
GROUP BY o.id, o.status, o.total_minor, o.winning_payment_id, m.code, p.provider, p.state, p.outcome
ORDER BY o.created_at;

\if :dry_run
\echo '=== DRY RUN: ff97b6ab slot 19 in_progress → success, vending → completed ==='
SELECT vs.order_id,
       vs.line_sequence,
       vs.slot_index,
       vs.state AS current_state,
       'success' AS next_state
FROM vend_sessions vs
WHERE vs.order_id = 'ff97b6ab-8d85-4f38-afc3-c46513f6292a'::uuid
  AND vs.slot_index = 19
  AND vs.state IN ('pending', 'in_progress');

SELECT o.id,
       o.status AS current_status,
       'completed' AS next_status
FROM orders o
WHERE o.id = 'ff97b6ab-8d85-4f38-afc3-c46513f6292a'::uuid
  AND o.status IN ('paid', 'vending');

\echo '=== DRY RUN: b8239908 slot 25 pending → failed TCN_NO_DROP ==='
SELECT vs.order_id,
       vs.line_sequence,
       vs.slot_index,
       vs.state AS current_state,
       'failed' AS next_state
FROM vend_sessions vs
WHERE vs.order_id = 'b8239908-2466-40be-bca0-0fea72765d7d'::uuid
  AND vs.slot_index = 25
  AND vs.state NOT IN ('success', 'failed', 'cancelled', 'skipped');

\echo '=== DRY RUN: b8239908 slot 26 pending → success ==='
SELECT vs.order_id,
       vs.line_sequence,
       vs.slot_index,
       vs.state AS current_state,
       'success' AS next_state
FROM vend_sessions vs
WHERE vs.order_id = 'b8239908-2466-40be-bca0-0fea72765d7d'::uuid
  AND vs.slot_index = 26
  AND vs.state IN ('pending', 'in_progress');

SELECT o.id,
       o.status AS current_status,
       'partially_completed' AS next_status
FROM orders o
WHERE o.id = 'b8239908-2466-40be-bca0-0fea72765d7d'::uuid
  AND o.status IN ('paid', 'vending');
\else
BEGIN;

\echo '=== ff97b6ab: slot 19 success ==='
UPDATE vend_sessions
SET
    state = 'success',
    failure_reason = NULL,
    started_at = COALESCE(started_at, now()),
    completed_at = COALESCE(completed_at, now())
WHERE order_id = 'ff97b6ab-8d85-4f38-afc3-c46513f6292a'::uuid
  AND slot_index = 19
  AND state IN ('pending', 'in_progress');

\echo '=== ff97b6ab: vending → completed ==='
UPDATE orders
SET
    status = 'completed',
    updated_at = now()
WHERE id = 'ff97b6ab-8d85-4f38-afc3-c46513f6292a'::uuid
  AND status IN ('paid', 'vending');

\echo '=== b8239908: slot 25 failed TCN_NO_DROP ==='
UPDATE vend_sessions
SET
    state = 'failed',
    failure_reason = COALESCE(failure_reason, 'TCN_NO_DROP (logcat 2026-11-01 02:39)'),
    started_at = COALESCE(started_at, now()),
    completed_at = COALESCE(completed_at, now())
WHERE order_id = 'b8239908-2466-40be-bca0-0fea72765d7d'::uuid
  AND slot_index = 25
  AND state NOT IN ('success', 'failed', 'cancelled', 'skipped');

\echo '=== b8239908: slot 26 success ==='
UPDATE vend_sessions
SET
    state = 'success',
    failure_reason = NULL,
    started_at = COALESCE(started_at, now()),
    completed_at = COALESCE(completed_at, now())
WHERE order_id = 'b8239908-2466-40be-bca0-0fea72765d7d'::uuid
  AND slot_index = 26
  AND state IN ('pending', 'in_progress');

\echo '=== b8239908: paid → partially_completed ==='
UPDATE orders
SET
    status = 'partially_completed',
    updated_at = now()
WHERE id = 'b8239908-2466-40be-bca0-0fea72765d7d'::uuid
  AND status IN ('paid', 'vending');

COMMIT;
\endif

\echo '=== Preview: 2026-11-01 02:39 stuck orders after repair ==='
SELECT o.id,
       o.status,
       o.total_minor,
       p.provider,
       p.state AS payment_state,
       p.outcome,
       array_agg(vs.slot_index ORDER BY vs.line_sequence) AS slot_indexes,
       array_agg(vs.state ORDER BY vs.line_sequence) AS line_states
FROM orders o
LEFT JOIN payments p ON p.id = o.winning_payment_id
LEFT JOIN vend_sessions vs ON vs.order_id = o.id
WHERE o.id IN (
    'ff97b6ab-8d85-4f38-afc3-c46513f6292a'::uuid,
    'b8239908-2466-40be-bca0-0fea72765d7d'::uuid
)
GROUP BY o.id, o.status, o.total_minor, p.provider, p.state, p.outcome
ORDER BY o.created_at;
