-- Read-only planogram / physical slot topology diagnostics.
-- Usage: psql "$DATABASE_URL" -v machine_code='AVF000132' -f scripts/ops/diagnose-machine-slot-topology.sql

\set ON_ERROR_STOP on

\echo '=== B0: Resolve machine ==='
SELECT id, code, status, machine_type, published_planogram_version_id
FROM machines
WHERE code = :'machine_code';

\echo '=== B1: Counts ==='
SELECT count(*) AS current_config_count
FROM machine_slot_configs msc
JOIN machines m ON m.id = msc.machine_id
WHERE m.code = :'machine_code' AND msc.is_current;

SELECT count(*) AS configs_with_product
FROM machine_slot_configs msc
JOIN machines m ON m.id = msc.machine_id
WHERE m.code = :'machine_code' AND msc.is_current AND msc.product_id IS NOT NULL;

SELECT count(*) AS configs_without_product
FROM machine_slot_configs msc
JOIN machines m ON m.id = msc.machine_id
WHERE m.code = :'machine_code' AND msc.is_current AND msc.product_id IS NULL;

\echo '=== B1: Layout dimensions ==='
SELECT mc.cabinet_code, msl.layout_key, msl.revision, msl.grid_rows, msl.grid_cols, msl.status
FROM machine_slot_layouts msl
JOIN machine_cabinets mc ON mc.id = msl.machine_cabinet_id
JOIN machines m ON m.id = msl.machine_id
WHERE m.code = :'machine_code'
ORDER BY mc.sort_order, msl.revision DESC;

\echo '=== B1: Legacy inventory + merges ==='
SELECT count(*) AS legacy_slot_state_count
FROM machine_slot_state mss
JOIN machines m ON m.id = mss.machine_id
WHERE m.code = :'machine_code';

SELECT left_slot_code, right_slot_code, cabinet_code, layout_key, revision, merged_at
FROM machine_lane_merge_pairs mlp
JOIN machines m ON m.id = mlp.machine_id
WHERE m.code = :'machine_code' AND mlp.is_active
ORDER BY left_slot_code;

\echo '=== B2: Row A detail ==='
SELECT msc.slot_code, msc.slot_index, msc.product_id, msc.price_minor, msc.max_quantity,
       msc.metadata->>'mergeRole' AS merge_role,
       msc.metadata->>'mergeWith' AS merge_with
FROM machine_slot_configs msc
JOIN machines m ON m.id = msc.machine_id
WHERE m.code = :'machine_code' AND msc.is_current
  AND msc.slot_code ~ '^A[0-9]+$'
ORDER BY msc.slot_index NULLS LAST, msc.slot_code;

\echo '=== B3: Missing slot codes (10x6 expected) ==='
WITH machine AS (
  SELECT id FROM machines WHERE code = :'machine_code'
),
layout AS (
  SELECT coalesce(max(msl.grid_rows), 6) AS grid_rows,
         coalesce(max(msl.grid_cols), 10) AS grid_cols
  FROM machine_slot_layouts msl
  JOIN machine m ON m.id = msl.machine_id
  WHERE msl.machine_id = (SELECT id FROM machine)
),
expected AS (
  SELECT chr(64 + row_n) || col_n::text AS slot_code, row_n, col_n
  FROM layout,
       generate_series(1, (SELECT grid_rows FROM layout)) AS row_n,
       generate_series(1, (SELECT grid_cols FROM layout)) AS col_n
),
actual AS (
  SELECT msc.slot_code
  FROM machine_slot_configs msc
  WHERE msc.machine_id = (SELECT id FROM machine) AND msc.is_current
)
SELECT e.slot_code, 'MISSING' AS backend_db
FROM expected e
LEFT JOIN actual a ON a.slot_code = e.slot_code
WHERE a.slot_code IS NULL
ORDER BY e.row_n, e.col_n;

\echo '=== B4: Catalog builder eligibility ==='
SELECT msc.slot_code, (msc.product_id IS NOT NULL) AS would_enter_catalog_builder
FROM machine_slot_configs msc
JOIN machines m ON m.id = msc.machine_id
WHERE m.code = :'machine_code' AND msc.is_current
ORDER BY msc.slot_code;
