-- Repair cash orders from log 2026-10-02 21:24–21:26 (all lines NO_DROP / LANE_VEND_AMBIGUOUS).
-- Payment captured on server; vend lines may be stuck in_progress or vending.
--
-- Orders:
--   01a0fd00-5de3-7506-87e6-c9fdeffd1cd3 (D5 5k, 1 line)
--   01a0fd00-aefe-7aea-ad23-d207ace2c86d (D6+D8 20k, 2 lines)
--   01a0fd01-9cf9-71a8-b04c-55242074cfd7 (E1+E2+E3 30k, 3 lines)
--
-- Usage:
--   psql "$DATABASE_URL" -v dry_run=1 -f scripts/ops/repair_cash_no_drop_orders_20261002_2124.sql
--   psql "$DATABASE_URL" -v dry_run=0 -f scripts/ops/repair_cash_no_drop_orders_20261002_2124.sql

\set ON_ERROR_STOP on

\echo '=== Preview: target orders before repair ==='
SELECT o.id,
       o.status,
       o.total_minor,
       m.code AS machine_code,
       array_agg(vs.line_sequence ORDER BY vs.line_sequence) AS line_sequences,
       array_agg(vs.state ORDER BY vs.line_sequence) AS line_states
FROM orders o
JOIN machines m ON m.id = o.machine_id
JOIN vend_sessions vs ON vs.order_id = o.id
WHERE o.id IN (
    '01a0fd00-5de3-7506-87e6-c9fdeffd1cd3'::uuid,
    '01a0fd00-aefe-7aea-ad23-d207ace2c86d'::uuid,
    '01a0fd01-9cf9-71a8-b04c-55242074cfd7'::uuid
)
GROUP BY o.id, o.status, o.total_minor, m.code
ORDER BY o.created_at;

\if :dry_run
\echo '=== DRY RUN: would mark non-terminal vend lines failed ==='
SELECT vs.order_id,
       vs.line_sequence,
       vs.state AS current_state,
       'failed' AS next_state,
       'TCN no-drop / LANE_VEND_AMBIGUOUS' AS next_failure_reason
FROM vend_sessions vs
WHERE vs.order_id IN (
    '01a0fd00-5de3-7506-87e6-c9fdeffd1cd3'::uuid,
    '01a0fd00-aefe-7aea-ad23-d207ace2c86d'::uuid,
    '01a0fd01-9cf9-71a8-b04c-55242074cfd7'::uuid
)
  AND vs.state NOT IN ('success', 'failed', 'cancelled', 'skipped')
ORDER BY vs.order_id, vs.line_sequence;
\else
BEGIN;

UPDATE vend_sessions
SET
    state = 'failed',
    failure_reason = 'TCN no-drop / LANE_VEND_AMBIGUOUS',
    completed_at = COALESCE(completed_at, now())
WHERE order_id IN (
    '01a0fd00-5de3-7506-87e6-c9fdeffd1cd3'::uuid,
    '01a0fd00-aefe-7aea-ad23-d207ace2c86d'::uuid,
    '01a0fd01-9cf9-71a8-b04c-55242074cfd7'::uuid
)
  AND state NOT IN ('success', 'failed', 'cancelled', 'skipped');

UPDATE orders o
SET status = 'failed'
WHERE o.id IN (
    '01a0fd00-5de3-7506-87e6-c9fdeffd1cd3'::uuid,
    '01a0fd00-aefe-7aea-ad23-d207ace2c86d'::uuid,
    '01a0fd01-9cf9-71a8-b04c-55242074cfd7'::uuid
)
  AND o.status IN ('created', 'paid', 'vending')
  AND NOT EXISTS (
      SELECT 1 FROM vend_sessions vs
      WHERE vs.order_id = o.id AND vs.state = 'success'
  );

COMMIT;

\echo '=== After repair ==='
SELECT o.id, o.status,
       array_agg(vs.line_sequence ORDER BY vs.line_sequence) AS line_sequences,
       array_agg(vs.state ORDER BY vs.line_sequence) AS line_states
FROM orders o
JOIN vend_sessions vs ON vs.order_id = o.id
WHERE o.id IN (
    '01a0fd00-5de3-7506-87e6-c9fdeffd1cd3'::uuid,
    '01a0fd00-aefe-7aea-ad23-d207ace2c86d'::uuid,
    '01a0fd01-9cf9-71a8-b04c-55242074cfd7'::uuid
)
GROUP BY o.id, o.status
ORDER BY o.id;
\endif
