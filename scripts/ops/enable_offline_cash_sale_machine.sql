-- Enable offline_cash_sale_allowed for one machine via feature_flag_targets.
-- Usage (psql):
--   psql "$DATABASE_URL" -v lookup_machine_id='01a089ec-c7bb-7e0d-83a9-6f599f061f12' \
--     -f scripts/ops/enable_offline_cash_sale_machine.sql
\set ON_ERROR_STOP on

\set offline_cash_flag_key 'offline_cash_sale_allowed'

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
        btrim(:'machine_code') <> ''
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

\if :{?resolved_machine_id}
\else
\echo enable-offline-cash: error: machine not found id=:lookup_machine_id code=:machine_code serial=:device_serial
\quit 1
\endif

INSERT INTO feature_flags (flag_key, display_name, description, enabled, metadata)
VALUES (
    :'offline_cash_flag_key',
    'Offline cash sale allowed',
    'Allow local-first cash checkout and skip backend StartVend for local mirror orders.',
    true,
    '{}'::jsonb
)
ON CONFLICT (flag_key) DO UPDATE
SET
    enabled = EXCLUDED.enabled,
    updated_at = now();

SELECT id AS flag_id
FROM feature_flags
WHERE flag_key = :'offline_cash_flag_key'
LIMIT 1
\gset

BEGIN;

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

\echo enable-offline-cash: configured machine_id=:resolved_machine_id flag=:offline_cash_flag_key

SELECT
    ff.flag_key,
    ff.enabled AS master_enabled,
    fft.target_type,
    fft.priority,
    fft.enabled AS target_enabled,
    fft.machine_id
FROM feature_flags ff
JOIN feature_flag_targets fft ON fft.feature_flag_id = ff.id
WHERE ff.flag_key = :'offline_cash_flag_key'
  AND fft.machine_id = :'resolved_machine_id'::uuid;
