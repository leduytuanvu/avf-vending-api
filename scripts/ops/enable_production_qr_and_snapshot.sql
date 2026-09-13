-- Enable MoMo/ZaloPay/VietQR payment methods and periodic 5m layout snapshot flag for one machine.
-- Invoked by scripts/ops/enable_production_qr_and_snapshot.sh (DB fallback on production runner).
\set ON_ERROR_STOP on

\set ON_ERROR_STOP off
SELECT id AS resolved_machine_id
FROM machines
WHERE
    (
        btrim(:'lookup_machine_id') <> ''
        AND id = :'lookup_machine_id'::uuid
    )
    OR (
        btrim(:'device_serial') <> ''
        AND lower(btrim(serial_number)) = lower(btrim(:'device_serial'))
    )
    OR (
        btrim(code) <> ''
        AND lower(btrim(code)) = lower(btrim(:'machine_code'))
    )
ORDER BY
    CASE
        WHEN btrim(:'lookup_machine_id') <> '' AND id = :'lookup_machine_id'::uuid THEN 0
        WHEN btrim(:'device_serial') <> '' AND lower(btrim(serial_number)) = lower(btrim(:'device_serial')) THEN 1
        ELSE 2
    END
LIMIT 1
\gset
\set ON_ERROR_STOP on

\if :{?resolved_machine_id}
\else
\echo enable-qr-snapshot: error: machine not found for code=:machine_code id=:lookup_machine_id serial=:device_serial
\echo enable-qr-snapshot: candidate machines (recent active/provisioned):
SELECT id, code, serial_number, status, updated_at
FROM machines
WHERE
    lower(code) LIKE lower('%' || btrim(:'machine_code') || '%')
    OR (
        btrim(:'device_serial') <> ''
        AND lower(serial_number) LIKE lower('%' || btrim(:'device_serial') || '%')
    )
    OR code ILIKE 'AVF%'
ORDER BY updated_at DESC NULLS LAST
LIMIT 10;
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
WHERE machine_id = :'resolved_machine_id'::uuid;

INSERT INTO machine_payment_methods (machine_id, method_key, enabled, sort_order)
VALUES
    (:'resolved_machine_id'::uuid, 'cash', true, 0),
    (:'resolved_machine_id'::uuid, 'momo', true, 1),
    (:'resolved_machine_id'::uuid, 'zalopay', true, 2),
    (:'resolved_machine_id'::uuid, 'vietqr', true, 3);

DELETE FROM feature_flag_targets
WHERE feature_flag_id = :'flag_id'::uuid
  AND target_type = 'machine'
  AND machine_id = :'resolved_machine_id'::uuid;

INSERT INTO feature_flag_targets (
    feature_flag_id,
    target_type,
    machine_id,
    priority,
    enabled
)
VALUES (:'flag_id'::uuid, 'machine', :'resolved_machine_id'::uuid, 100, true);

COMMIT;

\echo enable-qr-snapshot: configured machine_id=:resolved_machine_id code=:machine_code periodic_flag=:periodic_flag_key

SELECT
    m.id AS machine_id,
    m.code AS machine_code,
    m.serial_number,
    mpm.method_key,
    mpm.enabled,
    mpm.sort_order
FROM machines m
JOIN machine_payment_methods mpm ON mpm.machine_id = m.id
WHERE m.id = :'resolved_machine_id'::uuid
ORDER BY mpm.sort_order ASC, mpm.method_key ASC;

SELECT
    ff.flag_key,
    fft.target_type,
    fft.priority,
    fft.enabled
FROM feature_flags ff
JOIN feature_flag_targets fft ON fft.feature_flag_id = ff.id
WHERE ff.id = :'flag_id'::uuid
  AND fft.machine_id = :'resolved_machine_id'::uuid;
