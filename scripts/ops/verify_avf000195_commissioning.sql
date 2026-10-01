-- Read-only verification for AVF000195 commissioning session (2026-10-01).
-- Usage: psql "$DATABASE_URL" -f scripts/ops/verify_avf000195_commissioning.sql

\set ON_ERROR_STOP on

\echo '=== Machine ==='
SELECT id, code, status, machine_type, published_planogram_version_id, sale_enabled, activated_at
FROM machines
WHERE code = 'AVF000195';

\echo '=== Cabinet metadata (bootstrap protocol hints) ==='
SELECT mc.cabinet_code, mc.status, mc.metadata
FROM machine_cabinets mc
JOIN machines m ON m.id = mc.machine_id
WHERE m.code = 'AVF000195'
ORDER BY mc.sort_order, mc.cabinet_code;

\echo '=== Planogram / slot config summary ==='
SELECT count(*) AS current_slot_configs,
       count(*) FILTER (WHERE msc.product_id IS NOT NULL) AS with_product,
       count(*) FILTER (WHERE msc.product_id IS NULL) AS without_product
FROM machine_slot_configs msc
JOIN machines m ON m.id = msc.machine_id
WHERE m.code = 'AVF000195' AND msc.is_current;

SELECT count(*) AS legacy_slot_state_rows
FROM machine_slot_state mss
JOIN machines m ON m.id = mss.machine_id
WHERE m.code = 'AVF000195';

SELECT m.published_planogram_version_id,
       mpv.version_no,
       mpv.status,
       (SELECT count(*) FROM machine_planogram_slots mps WHERE mps.version_id = mpv.id) AS version_slot_count
FROM machines m
LEFT JOIN machine_planogram_versions mpv ON mpv.id = m.published_planogram_version_id
WHERE m.code = 'AVF000195';

\echo '=== Commissioning cash orders ==='
SELECT o.id,
       o.machine_id,
       m.code AS machine_code,
       o.status,
       o.total_minor,
       o.currency,
       wp.provider AS winning_payment_provider,
       wp.state AS winning_payment_state,
       o.created_at
FROM orders o
JOIN machines m ON m.id = o.machine_id
LEFT JOIN payments wp ON wp.id = o.winning_payment_id
WHERE o.id IN (
  '01a0f5fa-5f54-7d77-8a04-0a0352a51a43'::uuid,
  '01a0f5fb-53cc-7eff-835a-b59b01bb81d3'::uuid
)
ORDER BY o.created_at;

\echo '=== Vend sessions (line items) ==='
SELECT vs.order_id,
       vs.line_sequence,
       vs.slot_index,
       pr.name AS product_name,
       vs.state AS vend_state,
       vs.failure_reason,
       vs.started_at,
       vs.completed_at
FROM vend_sessions vs
JOIN products pr ON pr.id = vs.product_id
WHERE vs.order_id IN (
  '01a0f5fa-5f54-7d77-8a04-0a0352a51a43'::uuid,
  '01a0f5fb-53cc-7eff-835a-b59b01bb81d3'::uuid
)
ORDER BY vs.order_id, vs.line_sequence;

\echo '=== Payments ==='
SELECT p.order_id, p.provider, p.state, p.outcome, p.amount_minor, p.currency, p.captured_at, p.created_at
FROM payments p
WHERE p.order_id IN (
  '01a0f5fa-5f54-7d77-8a04-0a0352a51a43'::uuid,
  '01a0f5fb-53cc-7eff-835a-b59b01bb81d3'::uuid
)
ORDER BY p.order_id, p.created_at;
