-- name: AdminListMachineOperationalOverview :many
SELECT
    m.id AS machine_id,
    m.code AS machine_code,
    m.name AS machine_name,
    m.status AS lifecycle_status,
    m.online_status,
    m.sale_enabled,
    m.machine_type,
    m.last_seen_at,
    m.credential_version,
    s.id AS site_id,
    s.name AS site_name,
    da.id AS device_attachment_id,
    da.android_id,
    da.board_serial,
    da.sim_iccid,
    da.sim_operator,
    ras.id AS runtime_app_session_id,
    ras.status AS runtime_session_status,
    ras.start_reason AS runtime_start_reason,
    ras.started_at AS runtime_started_at,
    ras.last_heartbeat_at AS runtime_last_heartbeat_at,
    ras.last_mqtt_seen_at,
    ras.last_mqtt_state,
    ras.storefront_state,
    ras.sell_ready,
    ras.blockers,
    ms.id AS credential_session_id,
    ms.status AS credential_session_status,
    ms.issued_at AS credential_issued_at,
    mos.id AS operator_session_id,
    mos.status AS operator_session_status,
    mos.started_at AS operator_started_at
FROM machines m
INNER JOIN sites s ON s.id = m.site_id
LEFT JOIN machine_device_attachments da
    ON da.id = m.current_device_attachment_id AND da.status = 'active'
LEFT JOIN machine_runtime_app_sessions ras
    ON ras.id = m.current_runtime_app_session_id AND ras.ended_at IS NULL
LEFT JOIN machine_sessions ms
    ON ms.machine_id = m.id AND ms.status = 'active' AND ms.revoked_at IS NULL
LEFT JOIN machine_operator_sessions mos
    ON mos.machine_id = m.id AND mos.status = 'ACTIVE' AND mos.ended_at IS NULL
WHERE
    ($1::boolean IS FALSE OR m.site_id = $2::uuid)
    AND ($3::boolean IS FALSE OR m.id = $4::uuid)
    AND ($5::boolean IS FALSE OR m.online_status = $6::text)
    AND ($7::boolean IS FALSE OR upper(regexp_replace(btrim(m.code), '\s+', '', 'g')) LIKE upper($8::text))
    AND ($9::boolean IS FALSE OR m.status = $10::text)
    AND ($11::boolean IS FALSE OR m.machine_type = $12::text)
    AND ($13::boolean IS FALSE OR EXISTS (
        SELECT 1 FROM machine_runtime_app_sessions ras2
        WHERE ras2.id = m.current_runtime_app_session_id
            AND ras2.ended_at IS NULL
            AND ras2.sell_ready = $14::boolean
    ))
    AND ($15::boolean IS FALSE OR EXISTS (
        SELECT 1 FROM machine_operator_sessions mos2
        WHERE mos2.machine_id = m.id AND mos2.status = 'ACTIVE' AND mos2.ended_at IS NULL
    ) = $16::boolean)
ORDER BY m.last_seen_at DESC NULLS LAST, m.name ASC
LIMIT sqlc.arg (limit_val) OFFSET sqlc.arg (offset_val);

-- name: AdminCountMachineOperationalOverview :one
SELECT count(*)::bigint AS count
FROM machines m
WHERE
    ($1::boolean IS FALSE OR m.site_id = $2::uuid)
    AND ($3::boolean IS FALSE OR m.id = $4::uuid)
    AND ($5::boolean IS FALSE OR m.online_status = $6::text)
    AND ($7::boolean IS FALSE OR upper(regexp_replace(btrim(m.code), '\s+', '', 'g')) LIKE upper($8::text))
    AND ($9::boolean IS FALSE OR m.status = $10::text)
    AND ($11::boolean IS FALSE OR m.machine_type = $12::text)
    AND ($13::boolean IS FALSE OR EXISTS (
        SELECT 1 FROM machine_runtime_app_sessions ras2
        WHERE ras2.id = m.current_runtime_app_session_id
            AND ras2.ended_at IS NULL
            AND ras2.sell_ready = $14::boolean
    ))
    AND ($15::boolean IS FALSE OR EXISTS (
        SELECT 1 FROM machine_operator_sessions mos2
        WHERE mos2.machine_id = m.id AND mos2.status = 'ACTIVE' AND mos2.ended_at IS NULL
    ) = $16::boolean);
