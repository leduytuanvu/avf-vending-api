-- Enable MoMo/ZaloPay/VietQR payment methods and periodic 5m snapshot flag for one machine.
-- Invoked by scripts/ops/enable_production_qr_and_snapshot.sh (DB fallback on production runner).
\set ON_ERROR_STOP on

SELECT id AS machine_id
FROM machines
WHERE code = :'machine_code'
LIMIT 1
\gset

\if :{?machine_id}
\else
\echo enable-qr-snapshot: error: machine not found for code :machine_code
\quit 1
\endif

SELECT id AS flag_id
FROM feature_flags
WHERE flag_key = :'periodic_flag_key'
LIMIT 1
\gset

\if :{?flag_id}
\else
\echo enable-qr-snapshot: error: feature flag not found: :periodic_flag_key
\quit 1
\endif

BEGIN;

DELETE FROM machine_payment_methods
WHERE machine_id = :'machine_id'::uuid;

INSERT INTO machine_payment_methods (machine_id, method_key, enabled, sort_order)
VALUES
    (:'machine_id'::uuid, 'cash', true, 0),
    (:'machine_id'::uuid, 'momo', true, 1),
    (:'machine_id'::uuid, 'zalopay', true, 2),
    (:'machine_id'::uuid, 'vietqr', true, 3);

DELETE FROM feature_flag_targets
WHERE feature_flag_id = :'flag_id'::uuid
  AND target_type = 'machine'
  AND machine_id = :'machine_id'::uuid;

INSERT INTO feature_flag_targets (
    feature_flag_id,
    target_type,
    machine_id,
    priority,
    enabled
)
VALUES (:'flag_id'::uuid, 'machine', :'machine_id'::uuid, 100, true);

COMMIT;

\echo enable-qr-snapshot: configured machine_id=:machine_id code=:machine_code periodic_flag=:periodic_flag_key

SELECT
    m.code AS machine_code,
    mpm.method_key,
    mpm.enabled,
    mpm.sort_order
FROM machines m
JOIN machine_payment_methods mpm ON mpm.machine_id = m.id
WHERE m.id = :'machine_id'::uuid
ORDER BY mpm.sort_order ASC, mpm.method_key ASC;

SELECT
    ff.flag_key,
    fft.target_type,
    fft.priority,
    fft.enabled
FROM feature_flags ff
JOIN feature_flag_targets fft ON fft.feature_flag_id = ff.id
WHERE ff.id = :'flag_id'::uuid
  AND fft.machine_id = :'machine_id'::uuid;
