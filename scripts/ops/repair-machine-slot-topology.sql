-- Idempotent machine-scoped backfill of missing physical machine_slot_configs rows.
-- Run diagnose-machine-slot-topology.sql first. Requires explicit machine_code.
-- Usage (dry-run): psql ... -v machine_code='AVF000132' -v dry_run=1 -f repair-machine-slot-topology.sql
-- Usage (commit):  psql ... -v machine_code='AVF000132' -v dry_run=0 -f repair-machine-slot-topology.sql

\set ON_ERROR_STOP on

BEGIN;

CREATE TEMP TABLE _repair_ctx ON COMMIT DROP AS
SELECT m.id AS machine_id,
       coalesce(max(msl.grid_rows), 6)::int AS grid_rows,
       coalesce(max(msl.grid_cols), 10)::int AS grid_cols
FROM machines m
LEFT JOIN machine_slot_layouts msl ON msl.machine_id = m.id
WHERE m.code = :'machine_code'
GROUP BY m.id;

DO $$
DECLARE
  v_machine_id uuid;
  v_rows int;
  v_cols int;
  v_dry boolean := coalesce(nullif(:'dry_run', '')::int, 1) <> 0;
BEGIN
  SELECT machine_id, grid_rows, grid_cols
  INTO v_machine_id, v_rows, v_cols
  FROM _repair_ctx;

  IF v_machine_id IS NULL THEN
    RAISE EXCEPTION 'machine not found for code %', :'machine_code';
  END IF;

  RAISE NOTICE 'repair machine_id=% grid=%x% dry_run=%', v_machine_id, v_cols, v_rows, v_dry;
END $$;

CREATE TEMP TABLE _missing ON COMMIT DROP AS
WITH ctx AS (SELECT * FROM _repair_ctx),
expected AS (
  SELECT chr(64 + row_n) || col_n::text AS slot_code,
         ((row_n - 1) * (SELECT grid_cols FROM ctx) + col_n)::int AS slot_index_guess
  FROM ctx,
       generate_series(1, (SELECT grid_rows FROM ctx)) AS row_n,
       generate_series(1, (SELECT grid_cols FROM ctx)) AS col_n
),
actual AS (
  SELECT slot_code
  FROM machine_slot_configs msc
  JOIN ctx ON ctx.machine_id = msc.machine_id
  WHERE msc.is_current
)
SELECT e.slot_code, e.slot_index_guess
FROM expected e
LEFT JOIN actual a ON a.slot_code = e.slot_code
WHERE a.slot_code IS NULL;

\echo '=== Missing slot codes to backfill ==='
SELECT * FROM _missing ORDER BY slot_index_guess;

-- Neighbor template: prefer left column in same row (e.g. A2 from A1)
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
  n.machine_id,
  n.machine_cabinet_id,
  n.machine_slot_layout_id,
  miss.slot_code,
  miss.slot_index_guess,
  NULL,
  n.max_quantity,
  0,
  now(),
  TRUE,
  jsonb_build_object(
    'repairSource', 'planogram-topology-backfill',
    'repairedAt', to_jsonb(now())
  )
FROM _missing miss
JOIN LATERAL (
  SELECT msc.machine_id,
         msc.machine_cabinet_id,
         msc.machine_slot_layout_id,
         msc.max_quantity
  FROM machine_slot_configs msc
  JOIN _repair_ctx ctx ON ctx.machine_id = msc.machine_id
  WHERE msc.is_current
    AND msc.slot_code = (
      regexp_replace(miss.slot_code, '[0-9]+$', '') ||
      greatest(1, (regexp_replace(miss.slot_code, '^[^0-9]+', ''))::int - 1)::text
    )
  LIMIT 1
) n ON TRUE
WHERE coalesce(nullif(:'dry_run', '')::int, 1) = 0
  AND NOT EXISTS (
    SELECT 1 FROM machine_slot_configs e
    JOIN _repair_ctx ctx ON ctx.machine_id = e.machine_id
    WHERE e.is_current AND e.slot_code = miss.slot_code
  );

\echo '=== Post-repair counts ==='
SELECT count(*) AS current_config_count
FROM machine_slot_configs msc
JOIN machines m ON m.id = msc.machine_id
WHERE m.code = :'machine_code' AND msc.is_current;

SELECT miss.slot_code
FROM _missing miss
LEFT JOIN machine_slot_configs msc
  ON msc.machine_id = (SELECT machine_id FROM _repair_ctx)
 AND msc.is_current
 AND msc.slot_code = miss.slot_code
WHERE msc.slot_code IS NULL;

DO $$
BEGIN
  IF coalesce(nullif(:'dry_run', '')::int, 1) <> 0 THEN
    RAISE NOTICE 'dry_run=1 — rolling back (no writes committed)';
    RAISE EXCEPTION 'dry_run_complete' USING ERRCODE = 'P0001';
  END IF;
END $$;

COMMIT;
