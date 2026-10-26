-- Enable cash + MoMo + ZaloPay + VietQR + ShopeePay on every machine.
-- Usage:
--   psql ... -v dry_run=1 -f scripts/ops/enable_fleet_payment_methods.sql
--   psql ... -v dry_run=0 -f scripts/ops/enable_fleet_payment_methods.sql
\set ON_ERROR_STOP on

\echo '=== dry_run=' :dry_run
\echo '=== Machines ==='
SELECT count(*) AS machine_count FROM machines;

\echo ''
\echo '=== Before: method counts ==='
SELECT method_key, enabled, count(*) AS machines
FROM machine_payment_methods
GROUP BY method_key, enabled
ORDER BY method_key, enabled;

\echo ''
\echo '=== Before: AVF000109 ==='
SELECT m.code, mpm.method_key, mpm.enabled, mpm.sort_order
FROM machines m
LEFT JOIN machine_payment_methods mpm ON mpm.machine_id = m.id
WHERE m.code = 'AVF000109'
ORDER BY mpm.sort_order NULLS LAST, mpm.method_key;

INSERT INTO machine_payment_methods (machine_id, method_key, enabled, sort_order)
SELECT m.id, v.method_key, true, v.sort_order
FROM machines m
CROSS JOIN (
    VALUES
        ('cash', 0),
        ('momo', 1),
        ('zalopay', 2),
        ('vietqr', 3),
        ('shopeepay', 4)
) AS v(method_key, sort_order)
WHERE :'dry_run' <> '1'
ON CONFLICT (machine_id, method_key)
DO UPDATE SET
    enabled = true,
    sort_order = EXCLUDED.sort_order,
    updated_at = now();

\echo ''
\echo '=== After: method counts (enabled) ==='
SELECT method_key, enabled, count(*) AS machines
FROM machine_payment_methods
GROUP BY method_key, enabled
ORDER BY method_key, enabled;

\echo ''
\echo '=== After: AVF000109 ==='
SELECT m.code, mpm.method_key, mpm.enabled, mpm.sort_order
FROM machines m
JOIN machine_payment_methods mpm ON mpm.machine_id = m.id
WHERE m.code = 'AVF000109'
ORDER BY mpm.sort_order, mpm.method_key;
