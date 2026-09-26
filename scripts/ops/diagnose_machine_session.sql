-- Diagnose machine auth session / credential health for runtime-session recovery.
-- Usage:
--   psql "$DATABASE_URL" -v lookup_machine_id='01a089ec-c7bb-7e0d-83a9-6f599f061f12' \
--     -f scripts/ops/diagnose_machine_session.sql
\set ON_ERROR_STOP on

SELECT id AS resolved_machine_id, code, status, credential_version
FROM machines
WHERE
    (btrim(:'lookup_machine_id') <> '' AND id = :'lookup_machine_id'::uuid)
    OR (btrim(:'machine_code') <> '' AND lower(btrim(code)) = lower(btrim(:'machine_code')))
ORDER BY
    CASE WHEN btrim(:'lookup_machine_id') <> '' AND id = :'lookup_machine_id'::uuid THEN 0 ELSE 1 END
LIMIT 1
\gset

\if :{?resolved_machine_id}
\else
\echo diagnose_machine_session: machine not found
\quit 1
\endif

\echo '=== machine ==='
SELECT id, code, status, credential_version, site_id, updated_at
FROM machines
WHERE id = :'resolved_machine_id'::uuid;

\echo '=== active machine_sessions (auth JWT session_id) ==='
SELECT
    id,
    machine_id,
    credential_id,
    credential_version,
    status,
    expires_at,
    revoked_at,
    issued_at,
    last_used_at
FROM machine_sessions
WHERE machine_id = :'resolved_machine_id'::uuid
ORDER BY issued_at DESC
LIMIT 10;

\echo '=== machine_credentials ==='
SELECT
    id,
    machine_id,
    credential_version,
    status,
    revoked_at,
    created_at
FROM machine_credentials
WHERE machine_id = :'resolved_machine_id'::uuid
ORDER BY credential_version DESC
LIMIT 5;

\echo '=== offline_cash_sale_allowed feature flag ==='
SELECT
    ff.flag_key,
    ff.enabled AS master_enabled,
    fft.enabled AS target_enabled,
    fft.machine_id
FROM feature_flags ff
LEFT JOIN feature_flag_targets fft
    ON fft.feature_flag_id = ff.id
    AND fft.machine_id = :'resolved_machine_id'::uuid
WHERE ff.flag_key = 'offline_cash_sale_allowed';

\echo '=== recent machine_runtime_app_sessions ==='
SELECT
    id,
    machine_id,
    status,
    start_reason,
    started_at,
    ended_at,
    last_heartbeat_at
FROM machine_runtime_app_sessions
WHERE machine_id = :'resolved_machine_id'::uuid
ORDER BY started_at DESC NULLS LAST
LIMIT 5;
