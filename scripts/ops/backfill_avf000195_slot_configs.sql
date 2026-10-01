-- Backfill machine_slot_configs.slot_index + products from published planogram for AVF000195.
-- Fixes ConfirmVendSuccess INTERNAL: no machine_slot_config for slot_index=9/10 (A9/A10).
--
-- Usage:
--   psql "$DATABASE_URL" -v dry_run=1 -f scripts/ops/backfill_avf000195_slot_configs.sql
--   psql "$DATABASE_URL" -v dry_run=0 -f scripts/ops/backfill_avf000195_slot_configs.sql

\set ON_ERROR_STOP on

\echo '=== AVF000195 topology before backfill ==='
SELECT m.id AS machine_id,
       m.code,
       m.active_layout_id,
       ml.grid_cols,
       (SELECT count(*) FROM machine_slot_configs msc WHERE msc.machine_id = m.id AND msc.is_current) AS current_configs,
       (SELECT count(*) FROM machine_slot_configs msc WHERE msc.machine_id = m.id AND msc.is_current AND msc.product_id IS NOT NULL AND msc.slot_index IS NULL) AS configs_missing_slot_index
FROM machines m
LEFT JOIN machine_layouts ml ON ml.id = m.active_layout_id AND ml.machine_id = m.id
WHERE m.code = 'AVF000195';

\echo '=== A9/A10 configs before backfill ==='
SELECT msc.slot_code,
       msc.slot_index,
       msc.product_id,
       pr.name AS product_name,
       msc.max_quantity,
       msc.price_minor
FROM machine_slot_configs msc
JOIN machines m ON m.id = msc.machine_id
LEFT JOIN products pr ON pr.id = msc.product_id
WHERE m.code = 'AVF000195'
  AND msc.is_current
  AND msc.slot_code IN ('A9', 'A10')
ORDER BY msc.slot_code;

\echo '=== Published planogram A9/A10 ==='
SELECT mps.slot_code,
       mps.legacy_slot_index,
       mps.product_id,
       pr.name AS product_name,
       mps.max_quantity,
       mps.price_minor
FROM machines m
JOIN machine_planogram_slots mps ON mps.version_id = m.published_planogram_version_id
LEFT JOIN products pr ON pr.id = mps.product_id
WHERE m.code = 'AVF000195'
  AND mps.slot_code IN ('A9', 'A10')
ORDER BY mps.slot_code;

\if :dry_run
\echo '=== DRY RUN: would backfill slot_index + sync products from planogram ==='
WITH machine AS (
    SELECT m.id, coalesce(ml.grid_cols, 10) AS grid_cols
    FROM machines m
    LEFT JOIN machine_layouts ml ON ml.id = m.active_layout_id AND ml.machine_id = m.id
    WHERE m.code = 'AVF000195'
)
SELECT msc.slot_code,
       msc.slot_index AS current_slot_index,
       CASE
           WHEN msc.slot_code ~ '^[A-Z][0-9]+$' THEN
               ((ascii(substring(msc.slot_code FROM 1 FOR 1)) - 65) * machine.grid_cols)
               + substring(msc.slot_code FROM 2)::int
           ELSE NULL
       END AS would_set_slot_index,
       msc.product_id AS current_product_id,
       mps.product_id AS planogram_product_id,
       mps.max_quantity AS planogram_max_quantity,
       mps.price_minor AS planogram_price_minor
FROM machine_slot_configs msc
JOIN machine ON machine.id = msc.machine_id
LEFT JOIN machines m ON m.id = machine.id
LEFT JOIN machine_planogram_slots mps
    ON mps.version_id = m.published_planogram_version_id
    AND mps.slot_code = msc.slot_code
WHERE msc.is_current
  AND msc.slot_code IN ('A9', 'A10')
ORDER BY msc.slot_code;
\else
BEGIN;

\echo '=== Backfill slot_index + sync A9/A10 from published planogram ==='
DO $$
DECLARE
    v_machine_id uuid;
    v_grid_cols int;
    v_cabinet_id uuid;
    v_layout_config_id uuid;
    v_version_id uuid;
BEGIN
    SELECT m.id,
           coalesce(ml.grid_cols, 10),
           m.published_planogram_version_id
    INTO v_machine_id, v_grid_cols, v_version_id
    FROM machines m
    LEFT JOIN machine_layouts ml ON ml.id = m.active_layout_id AND ml.machine_id = m.id
    WHERE m.code = 'AVF000195';

    IF v_machine_id IS NULL THEN
        RAISE EXCEPTION 'machine AVF000195 not found';
    END IF;

    SELECT msc.machine_cabinet_id, msc.machine_slot_layout_id
    INTO v_cabinet_id, v_layout_config_id
    FROM machine_slot_configs msc
    WHERE msc.machine_id = v_machine_id AND msc.is_current
    LIMIT 1;

    IF v_cabinet_id IS NULL THEN
        SELECT mc.id INTO v_cabinet_id
        FROM machine_cabinets mc
        WHERE mc.machine_id = v_machine_id
        ORDER BY mc.sort_order, mc.cabinet_code
        LIMIT 1;
    END IF;

    IF v_layout_config_id IS NULL AND v_cabinet_id IS NOT NULL THEN
        SELECT msl.id INTO v_layout_config_id
        FROM machine_slot_layouts msl
        WHERE msl.machine_id = v_machine_id AND msl.machine_cabinet_id = v_cabinet_id
        ORDER BY msl.revision DESC
        LIMIT 1;
    END IF;

    -- Backfill slot_index from slot_code for all current configs.
    UPDATE machine_slot_configs msc
    SET
        slot_index = ((ascii(substring(msc.slot_code FROM 1 FOR 1)) - 65) * v_grid_cols)
            + substring(msc.slot_code FROM 2)::int,
        updated_at = now()
    WHERE msc.machine_id = v_machine_id
      AND msc.is_current
      AND msc.slot_index IS NULL
      AND msc.slot_code ~ '^[A-Z][0-9]+$';

    -- Fallback: copy product from latest successful vend on this machine when planogram is empty.
    UPDATE machine_slot_configs msc
    SET
        product_id = recent.product_id,
        updated_at = now()
    FROM (
        SELECT DISTINCT ON (vs.slot_index)
            vs.slot_index,
            vs.product_id
        FROM vend_sessions vs
        JOIN orders o ON o.id = vs.order_id
        WHERE o.machine_id = v_machine_id
          AND vs.slot_index IN (9, 10)
          AND vs.state = 'success'
          AND vs.product_id IS NOT NULL
        ORDER BY vs.slot_index, vs.completed_at DESC NULLS LAST, vs.started_at DESC
    ) recent
    WHERE msc.machine_id = v_machine_id
      AND msc.is_current
      AND msc.slot_code IN ('A9', 'A10')
      AND msc.slot_index = recent.slot_index
      AND msc.product_id IS NULL;

    -- Sync product/price/qty from published planogram onto existing current configs.
    UPDATE machine_slot_configs msc
    SET
        product_id = mps.product_id,
        max_quantity = GREATEST(mps.max_quantity, 1),
        price_minor = mps.price_minor,
        slot_index = coalesce(
            msc.slot_index,
            mps.legacy_slot_index,
            ((ascii(substring(msc.slot_code FROM 1 FOR 1)) - 65) * v_grid_cols)
                + substring(msc.slot_code FROM 2)::int
        ),
        updated_at = now()
    FROM machine_planogram_slots mps
    WHERE msc.machine_id = v_machine_id
      AND msc.is_current
      AND mps.version_id = v_version_id
      AND mps.slot_code = msc.slot_code
      AND mps.product_id IS NOT NULL
      AND msc.slot_code IN ('A9', 'A10');

    -- Insert missing current configs for A9/A10 from published planogram.
    IF v_cabinet_id IS NOT NULL AND v_layout_config_id IS NOT NULL AND v_version_id IS NOT NULL THEN
        INSERT INTO machine_slot_configs (
            machine_id,
            machine_cabinet_id,
            machine_slot_layout_id,
            slot_code,
            slot_index,
            product_id,
            max_quantity,
            price_minor,
            effective_from,
            is_current,
            metadata
        )
        SELECT
            v_machine_id,
            v_cabinet_id,
            v_layout_config_id,
            mps.slot_code,
            coalesce(
                mps.legacy_slot_index,
                ((ascii(substring(mps.slot_code FROM 1 FOR 1)) - 65) * v_grid_cols)
                    + substring(mps.slot_code FROM 2)::int
            ),
            mps.product_id,
            GREATEST(mps.max_quantity, 1),
            mps.price_minor,
            now(),
            true,
            '{"source":"backfill_avf000195_slot_configs"}'::jsonb
        FROM machine_planogram_slots mps
        WHERE mps.version_id = v_version_id
          AND mps.slot_code IN ('A9', 'A10')
          AND mps.product_id IS NOT NULL
          AND NOT EXISTS (
              SELECT 1
              FROM machine_slot_configs existing
              WHERE existing.machine_id = v_machine_id
                AND existing.is_current
                AND existing.slot_code = mps.slot_code
          );
    ELSE
        RAISE NOTICE 'Skipped insert for missing configs (cabinet=%, layout=%, version=%)',
            v_cabinet_id, v_layout_config_id, v_version_id;
    END IF;
END $$;

COMMIT;
\endif

\echo '=== A9/A10 configs after backfill ==='
SELECT msc.slot_code,
       msc.slot_index,
       msc.product_id,
       pr.name AS product_name,
       msc.max_quantity,
       msc.price_minor
FROM machine_slot_configs msc
JOIN machines m ON m.id = msc.machine_id
LEFT JOIN products pr ON pr.id = msc.product_id
WHERE m.code = 'AVF000195'
  AND msc.is_current
  AND msc.slot_code IN ('A9', 'A10')
ORDER BY msc.slot_code;
