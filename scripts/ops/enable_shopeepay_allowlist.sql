-- Enable ShopeePay on machines whose code is in the SPP store_ext_id allowlist.
-- Does not wipe other payment methods and does not enable the whole fleet.
-- Invoked by scripts/ops/enable_shopeepay_allowlist.sh
\set ON_ERROR_STOP on

WITH codes AS (
    SELECT btrim(x) AS code
    FROM unnest(string_to_array(:'allowlist_csv', ',')) AS x
    WHERE btrim(x) <> ''
      AND btrim(x) NOT LIKE '#%'
),
matched AS (
    SELECT m.id, m.code
    FROM machines m
    JOIN codes c ON lower(btrim(m.code)) = lower(c.code)
)
INSERT INTO machine_payment_methods (machine_id, method_key, enabled, sort_order)
SELECT matched.id, 'shopeepay', true, 4
FROM matched
ON CONFLICT (machine_id, method_key)
DO UPDATE SET
    enabled = true,
    sort_order = EXCLUDED.sort_order,
    updated_at = now();

\echo enable-shopeepay-allowlist: upserted shopeepay for matching machine codes

SELECT
    m.code AS machine_code,
    mpm.method_key,
    mpm.enabled,
    mpm.sort_order
FROM machines m
JOIN machine_payment_methods mpm ON mpm.machine_id = m.id
WHERE mpm.method_key = 'shopeepay'
  AND mpm.enabled
ORDER BY m.code;
