-- Read-only: product sales counts for a machine and SKU list in a time window.
-- Usage:
--   psql ... -v machine_code='AVF000132' -v window_from='2026-10-09 00:00:00+07' \
--     -v product_skus='1702,1711' -f scripts/ops/inspect_machine_product_sales.sql
\set ON_ERROR_STOP on

\echo '=== Parameters ==='
\echo 'machine_code=' :machine_code
\echo 'window_from=' :window_from
\echo 'product_skus=' :product_skus
\echo 'query_time_utc=' 
SELECT now() AT TIME ZONE 'UTC' AS query_time_utc;
\echo 'query_time_vn='
SELECT now() AT TIME ZONE 'Asia/Ho_Chi_Minh' AS query_time_vn;

\echo ''
\echo '=== Machine ==='
SELECT id, code, name, status
FROM machines
WHERE lower(btrim(code)) = lower(btrim(:'machine_code'))
LIMIT 1;

\echo ''
\echo '=== Products (SKU map) ==='
SELECT id, sku, name, active
FROM products
WHERE sku = ANY (
    SELECT btrim(x)
    FROM unnest(string_to_array(:'product_skus', ',')) AS x
    WHERE btrim(x) <> ''
)
ORDER BY sku;

\echo ''
\echo '=== Metric A: vend success (inventory velocity style) ==='
SELECT p.sku,
       COUNT(*)::bigint AS units_vended_success
FROM vend_sessions vs
INNER JOIN machines m ON m.id = vs.machine_id
INNER JOIN products p ON p.id = vs.product_id
WHERE lower(btrim(m.code)) = lower(btrim(:'machine_code'))
  AND vs.state = 'success'
  AND COALESCE(vs.completed_at, vs.created_at) >= :'window_from'::timestamptz
  AND COALESCE(vs.completed_at, vs.created_at) < now()
  AND p.sku = ANY (
      SELECT btrim(x)
      FROM unnest(string_to_array(:'product_skus', ',')) AS x
      WHERE btrim(x) <> ''
  )
GROUP BY p.sku
ORDER BY p.sku;

\echo ''
\echo '=== Metric B: lines on terminal orders (created_at window) ==='
SELECT p.sku,
       COUNT(*) FILTER (WHERE vs.state = 'success') AS lines_success,
       COUNT(*) AS lines_on_terminal_orders
FROM vend_sessions vs
INNER JOIN orders o ON o.id = vs.order_id
INNER JOIN machines m ON m.id = o.machine_id
INNER JOIN products p ON p.id = vs.product_id
WHERE lower(btrim(m.code)) = lower(btrim(:'machine_code'))
  AND o.status IN ('completed', 'partially_completed')
  AND o.created_at >= :'window_from'::timestamptz
  AND o.created_at < now()
  AND p.sku = ANY (
      SELECT btrim(x)
      FROM unnest(string_to_array(:'product_skus', ',')) AS x
      WHERE btrim(x) <> ''
  )
GROUP BY p.sku
ORDER BY p.sku;

\echo ''
\echo '=== Metric B alt: terminal orders by order updated_at ==='
SELECT p.sku,
       COUNT(*) FILTER (WHERE vs.state = 'success') AS lines_success,
       COUNT(*) AS lines_on_terminal_orders
FROM vend_sessions vs
INNER JOIN orders o ON o.id = vs.order_id
INNER JOIN machines m ON m.id = o.machine_id
INNER JOIN products p ON p.id = vs.product_id
WHERE lower(btrim(m.code)) = lower(btrim(:'machine_code'))
  AND o.status IN ('completed', 'partially_completed')
  AND o.updated_at >= :'window_from'::timestamptz
  AND o.updated_at < now()
  AND p.sku = ANY (
      SELECT btrim(x)
      FROM unnest(string_to_array(:'product_skus', ',')) AS x
      WHERE btrim(x) <> ''
  )
GROUP BY p.sku
ORDER BY p.sku;

\echo ''
\echo '=== Vend state breakdown (Metric A window) ==='
SELECT p.sku, vs.state, COUNT(*)::bigint AS cnt
FROM vend_sessions vs
INNER JOIN machines m ON m.id = vs.machine_id
INNER JOIN products p ON p.id = vs.product_id
WHERE lower(btrim(m.code)) = lower(btrim(:'machine_code'))
  AND COALESCE(vs.completed_at, vs.created_at) >= :'window_from'::timestamptz
  AND COALESCE(vs.completed_at, vs.created_at) < now()
  AND p.sku = ANY (
      SELECT btrim(x)
      FROM unnest(string_to_array(:'product_skus', ',')) AS x
      WHERE btrim(x) <> ''
  )
GROUP BY p.sku, vs.state
ORDER BY p.sku, vs.state;

\echo ''
\echo '=== Recent vends (up to 5 per SKU) ==='
SELECT sku,
       vend_state,
       order_status,
       completed_at_vn,
       order_id
FROM (
    SELECT p.sku,
           vs.state AS vend_state,
           o.status AS order_status,
           COALESCE(vs.completed_at, vs.created_at) AT TIME ZONE 'Asia/Ho_Chi_Minh' AS completed_at_vn,
           vs.order_id,
           ROW_NUMBER() OVER (
               PARTITION BY p.sku
               ORDER BY COALESCE(vs.completed_at, vs.created_at) DESC
           ) AS rn
    FROM vend_sessions vs
    INNER JOIN orders o ON o.id = vs.order_id
    INNER JOIN machines m ON m.id = vs.machine_id
    INNER JOIN products p ON p.id = vs.product_id
    WHERE lower(btrim(m.code)) = lower(btrim(:'machine_code'))
      AND COALESCE(vs.completed_at, vs.created_at) >= :'window_from'::timestamptz
      AND COALESCE(vs.completed_at, vs.created_at) < now()
      AND p.sku = ANY (
          SELECT btrim(x)
          FROM unnest(string_to_array(:'product_skus', ',')) AS x
          WHERE btrim(x) <> ''
      )
) t
WHERE rn <= 5
ORDER BY sku, completed_at_vn DESC;
