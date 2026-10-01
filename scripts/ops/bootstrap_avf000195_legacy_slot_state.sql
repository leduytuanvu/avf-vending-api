-- Bootstrap legacy machine_slot_state for AVF000195 from current machine_slot_configs.
-- Fixes ConfirmVendSuccess INTERNAL: no machine_slot_state for slot_index=N.
--
-- Also repairs stuck order 01a0f8a2 (2026-10-02 QR→cash, physical vend OK).
--
-- Usage:
--   psql "$DATABASE_URL" -v dry_run=1 -f scripts/ops/bootstrap_avf000195_legacy_slot_state.sql
--   psql "$DATABASE_URL" -v dry_run=0 -f scripts/ops/bootstrap_avf000195_legacy_slot_state.sql

\set ON_ERROR_STOP on

\echo '=== Machine AVF000195: legacy state before bootstrap ==='
SELECT m.id AS machine_id,
       m.code,
       (SELECT count(*) FROM machine_slot_state mss WHERE mss.machine_id = m.id) AS legacy_slot_state_rows,
       (SELECT count(*) FROM machine_slot_configs msc WHERE msc.machine_id = m.id AND msc.is_current) AS current_slot_configs,
       (SELECT count(*) FROM machine_slot_configs msc WHERE msc.machine_id = m.id AND msc.is_current AND msc.product_id IS NOT NULL) AS configs_with_product
FROM machines m
WHERE m.code = 'AVF000195';

\echo '=== Current slot configs with products (preview) ==='
SELECT msc.slot_code,
       msc.slot_index,
       pr.name AS product_name,
       msc.max_quantity,
       msc.price_minor
FROM machine_slot_configs msc
JOIN machines m ON m.id = msc.machine_id
LEFT JOIN products pr ON pr.id = msc.product_id
WHERE m.code = 'AVF000195'
  AND msc.is_current
  AND msc.product_id IS NOT NULL
  AND msc.slot_index IS NOT NULL
ORDER BY msc.slot_index;

\echo '=== Stuck order 01a0f8a2 before repair ==='
SELECT o.id,
       o.status,
       vs.line_sequence,
       vs.slot_index,
       vs.state AS vend_state,
       pr.name AS product_name
FROM orders o
JOIN machines m ON m.id = o.machine_id
JOIN vend_sessions vs ON vs.order_id = o.id
JOIN products pr ON pr.id = vs.product_id
WHERE o.id = '01a0f8a2-f584-7cc5-a266-1ce1582997ba'::uuid
ORDER BY vs.line_sequence;

\if :dry_run
\echo '=== DRY RUN: would bootstrap legacy slot state + repair order ==='
WITH machine AS (
    SELECT id FROM machines WHERE code = 'AVF000195'
),
resolved_planogram AS (
    SELECT coalesce(
        (SELECT mss.planogram_id FROM machine_slot_state mss JOIN machines m ON m.id = mss.machine_id WHERE m.code = 'AVF000195' LIMIT 1),
        (SELECT pg.id FROM planograms pg WHERE pg.status = 'published' ORDER BY pg.created_at DESC LIMIT 1)
    ) AS planogram_id
)
SELECT msc.slot_index,
       msc.product_id,
       msc.max_quantity AS would_set_current_quantity,
       msc.price_minor,
       rp.planogram_id
FROM machine_slot_configs msc
JOIN machine m ON m.id = msc.machine_id
CROSS JOIN resolved_planogram rp
WHERE msc.is_current
  AND msc.product_id IS NOT NULL
  AND msc.slot_index IS NOT NULL
ORDER BY msc.slot_index;
\else
BEGIN;

\echo '=== Resolve or create planogram for AVF000195 ==='
DO $$
DECLARE
    v_machine_id uuid;
    v_planogram_id uuid;
BEGIN
    SELECT id INTO v_machine_id FROM machines WHERE code = 'AVF000195';
    IF v_machine_id IS NULL THEN
        RAISE EXCEPTION 'machine AVF000195 not found';
    END IF;

    SELECT mss.planogram_id INTO v_planogram_id
    FROM machine_slot_state mss
    WHERE mss.machine_id = v_machine_id
    LIMIT 1;

    IF v_planogram_id IS NULL THEN
        SELECT pg.id INTO v_planogram_id
        FROM planograms pg
        WHERE pg.status = 'published'
        ORDER BY pg.created_at DESC
        LIMIT 1;
    END IF;

    IF v_planogram_id IS NULL THEN
        INSERT INTO planograms (name, revision, status, meta)
        VALUES ('AVF000195 bootstrap', 1, 'published', '{"source":"bootstrap_avf000195_legacy_slot_state"}'::jsonb)
        RETURNING id INTO v_planogram_id;
        RAISE NOTICE 'Created bootstrap planogram %', v_planogram_id;
    ELSE
        RAISE NOTICE 'Using planogram %', v_planogram_id;
    END IF;

    INSERT INTO machine_slot_state (
        machine_id,
        planogram_id,
        slot_index,
        current_quantity,
        price_minor,
        planogram_revision_applied
    )
    SELECT
        msc.machine_id,
        v_planogram_id,
        msc.slot_index,
        GREATEST(msc.max_quantity, 1),
        msc.price_minor,
        1
    FROM machine_slot_configs msc
    WHERE msc.machine_id = v_machine_id
      AND msc.is_current
      AND msc.product_id IS NOT NULL
      AND msc.slot_index IS NOT NULL
    ON CONFLICT (machine_id, planogram_id, slot_index) DO UPDATE
    SET
        current_quantity = GREATEST(EXCLUDED.current_quantity, machine_slot_state.current_quantity),
        price_minor = EXCLUDED.price_minor,
        updated_at = now();

    -- Ensure slots table has product mapping for legacy queries
    INSERT INTO slots (planogram_id, slot_index, product_id, max_quantity)
    SELECT
        v_planogram_id,
        msc.slot_index,
        msc.product_id,
        msc.max_quantity
    FROM machine_slot_configs msc
    WHERE msc.machine_id = v_machine_id
      AND msc.is_current
      AND msc.product_id IS NOT NULL
      AND msc.slot_index IS NOT NULL
    ON CONFLICT (planogram_id, slot_index) DO UPDATE
    SET
        product_id = EXCLUDED.product_id,
        max_quantity = EXCLUDED.max_quantity;
END $$;

\echo '=== Repair stuck order 01a0f8a2 (physical vend confirmed) ==='
UPDATE vend_sessions
SET
    state = 'success',
    failure_reason = NULL,
    completed_at = COALESCE(completed_at, now())
WHERE order_id = '01a0f8a2-f584-7cc5-a266-1ce1582997ba'::uuid
  AND state = 'in_progress';

UPDATE orders
SET
    status = 'completed',
    updated_at = now()
WHERE id = '01a0f8a2-f584-7cc5-a266-1ce1582997ba'::uuid
  AND status = 'vending';

COMMIT;
\endif

\echo '=== Machine AVF000195: legacy state after bootstrap ==='
SELECT m.id AS machine_id,
       m.code,
       (SELECT count(*) FROM machine_slot_state mss WHERE mss.machine_id = m.id) AS legacy_slot_state_rows,
       (SELECT count(*) FROM machine_slot_configs msc WHERE msc.machine_id = m.id AND msc.is_current AND msc.product_id IS NOT NULL) AS configs_with_product
FROM machines m
WHERE m.code = 'AVF000195';

\echo '=== Legacy slot state rows (slots 3,4 highlighted) ==='
SELECT mss.slot_index,
       pr.name AS product_name,
       mss.current_quantity,
       mss.price_minor,
       mss.planogram_id
FROM machine_slot_state mss
JOIN machines m ON m.id = mss.machine_id
LEFT JOIN slots s ON s.planogram_id = mss.planogram_id AND s.slot_index = mss.slot_index
LEFT JOIN products pr ON pr.id = s.product_id
WHERE m.code = 'AVF000195'
  AND mss.slot_index IN (3, 4)
ORDER BY mss.slot_index;

\echo '=== Stuck order 01a0f8a2 after repair ==='
SELECT o.id,
       o.status,
       array_agg(vs.state ORDER BY vs.line_sequence) AS line_states
FROM orders o
JOIN vend_sessions vs ON vs.order_id = o.id
WHERE o.id = '01a0f8a2-f584-7cc5-a266-1ce1582997ba'::uuid
GROUP BY o.id, o.status;
