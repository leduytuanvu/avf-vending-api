-- name: AdminUnifiedMachineTimeline :many
SELECT
    occurred_at,
    event_type,
    severity,
    machine_id,
    actor_type,
    actor_account_id,
    operator_session_id,
    machine_session_id,
    resource_type,
    resource_id,
    order_id,
    payment_id,
    vend_session_id,
    request_id,
    correlation_id,
    reason,
    error_code,
    summary,
    metadata
FROM (
    SELECT
        ae.occurred_at,
        ae.action AS event_type,
        'info'::text AS severity,
        ae.machine_id,
        ae.actor_type,
        NULL::uuid AS actor_account_id,
        NULL::uuid AS operator_session_id,
        NULL::uuid AS machine_session_id,
        ae.resource_type,
        ae.resource_id,
        NULL::uuid AS order_id,
        NULL::uuid AS payment_id,
        NULL::uuid AS vend_session_id,
        ae.request_id,
        NULL::uuid AS correlation_id,
        NULL::text AS reason,
        NULL::text AS error_code,
        ae.action AS summary,
        ae.metadata
    FROM
        audit_events ae
    WHERE
        ae.machine_id = sqlc.arg (machine_id)
        AND (
            sqlc.narg (from_ts)::timestamptz IS NULL
            OR ae.occurred_at >= sqlc.narg (from_ts)
        )
        AND (
            sqlc.narg (to_ts)::timestamptz IS NULL
            OR ae.occurred_at <= sqlc.narg (to_ts)
        )
    UNION ALL
    SELECT
        mac.claimed_at AS occurred_at,
        'activation.claim'::text AS event_type,
        CASE
            WHEN mac.result = 'succeeded' THEN 'info'
            ELSE 'warning'
        END AS severity,
        mac.machine_id,
        CASE
            WHEN mac.activated_by_account_id IS NOT NULL THEN 'user'
            ELSE 'machine'
        END AS actor_type,
        mac.activated_by_account_id AS actor_account_id,
        mac.operator_session_id,
        NULL::uuid AS machine_session_id,
        'machine_activation_claim'::text AS resource_type,
        mac.id::text AS resource_id,
        NULL::uuid AS order_id,
        NULL::uuid AS payment_id,
        NULL::uuid AS vend_session_id,
        mac.request_id,
        mac.correlation_id,
        mac.reason,
        mac.failure_reason AS error_code,
        COALESCE(mac.activation_source, 'activation_code') AS summary,
        '{}'::jsonb AS metadata
    FROM
        machine_activation_claims mac
    WHERE
        mac.machine_id = sqlc.arg (machine_id)
        AND (
            sqlc.narg (from_ts)::timestamptz IS NULL
            OR mac.claimed_at >= sqlc.narg (from_ts)
        )
        AND (
            sqlc.narg (to_ts)::timestamptz IS NULL
            OR mac.claimed_at <= sqlc.narg (to_ts)
        )
    UNION ALL
    SELECT
        maa.occurred_at,
        'action.attribution'::text AS event_type,
        'info'::text AS severity,
        maa.machine_id,
        'operator'::text AS actor_type,
        NULL::uuid AS actor_account_id,
        maa.operator_session_id,
        NULL::uuid AS machine_session_id,
        maa.resource_type,
        maa.resource_id,
        NULL::uuid AS order_id,
        NULL::uuid AS payment_id,
        NULL::uuid AS vend_session_id,
        NULL::text AS request_id,
        maa.correlation_id,
        NULL::text AS reason,
        NULL::text AS error_code,
        maa.action_origin_type AS summary,
        maa.metadata
    FROM
        machine_action_attributions maa
    WHERE
        maa.machine_id = sqlc.arg (machine_id)
        AND (
            sqlc.narg (from_ts)::timestamptz IS NULL
            OR maa.occurred_at >= sqlc.narg (from_ts)
        )
        AND (
            sqlc.narg (to_ts)::timestamptz IS NULL
            OR maa.occurred_at <= sqlc.narg (to_ts)
        )
        AND (
            sqlc.narg (operator_session_id)::uuid IS NULL
            OR maa.operator_session_id = sqlc.narg (operator_session_id)
        )
    UNION ALL
    SELECT
        mda.attached_at AS occurred_at,
        'device.attachment.attached'::text AS event_type,
        'info'::text AS severity,
        mda.machine_id,
        CASE
            WHEN mda.attached_by_account_id IS NOT NULL THEN 'user'
            ELSE 'machine'
        END AS actor_type,
        mda.attached_by_account_id AS actor_account_id,
        mda.operator_session_id,
        NULL::uuid AS machine_session_id,
        'machine_device_attachment'::text AS resource_type,
        mda.id::text AS resource_id,
        NULL::uuid AS order_id,
        NULL::uuid AS payment_id,
        NULL::uuid AS vend_session_id,
        NULL::text AS request_id,
        mda.correlation_id,
        mda.reason,
        NULL::text AS error_code,
        mda.status AS summary,
        jsonb_build_object(
            'android_id', mda.android_id,
            'board_serial', mda.board_serial,
            'sim_iccid', mda.sim_iccid
        ) AS metadata
    FROM
        machine_device_attachments mda
    WHERE
        mda.machine_id = sqlc.arg (machine_id)
        AND (
            sqlc.narg (from_ts)::timestamptz IS NULL
            OR mda.attached_at >= sqlc.narg (from_ts)
        )
        AND (
            sqlc.narg (to_ts)::timestamptz IS NULL
            OR mda.attached_at <= sqlc.narg (to_ts)
        )
    UNION ALL
    SELECT
        COALESCE(mda.detached_at, mda.updated_at) AS occurred_at,
        'device.attachment.replaced'::text AS event_type,
        'info'::text AS severity,
        mda.machine_id,
        'system'::text AS actor_type,
        NULL::uuid AS actor_account_id,
        mda.operator_session_id,
        NULL::uuid AS machine_session_id,
        'machine_device_attachment'::text AS resource_type,
        mda.id::text AS resource_id,
        NULL::uuid AS order_id,
        NULL::uuid AS payment_id,
        NULL::uuid AS vend_session_id,
        NULL::text AS request_id,
        mda.correlation_id,
        mda.reason,
        NULL::text AS error_code,
        mda.status AS summary,
        jsonb_build_object(
            'android_id', mda.android_id,
            'board_serial', mda.board_serial
        ) AS metadata
    FROM
        machine_device_attachments mda
    WHERE
        mda.machine_id = sqlc.arg (machine_id)
        AND mda.status IN ('replaced', 'revoked', 'compromised')
        AND mda.detached_at IS NOT NULL
        AND (
            sqlc.narg (from_ts)::timestamptz IS NULL
            OR mda.detached_at >= sqlc.narg (from_ts)
        )
        AND (
            sqlc.narg (to_ts)::timestamptz IS NULL
            OR mda.detached_at <= sqlc.narg (to_ts)
        )
    UNION ALL
    SELECT
        mras.started_at AS occurred_at,
        'runtime.app_session.started'::text AS event_type,
        'info'::text AS severity,
        mras.machine_id,
        'machine'::text AS actor_type,
        NULL::uuid AS actor_account_id,
        mras.operator_session_id,
        mras.machine_session_id,
        'machine_runtime_app_session'::text AS resource_type,
        mras.id::text AS resource_id,
        NULL::uuid AS order_id,
        NULL::uuid AS payment_id,
        NULL::uuid AS vend_session_id,
        NULL::text AS request_id,
        NULL::uuid AS correlation_id,
        mras.start_reason AS reason,
        NULL::text AS error_code,
        mras.status AS summary,
        jsonb_build_object(
            'boot_id', mras.boot_id,
            'app_start_id', mras.app_start_id,
            'storefront_state', mras.storefront_state
        ) AS metadata
    FROM
        machine_runtime_app_sessions mras
    WHERE
        mras.machine_id = sqlc.arg (machine_id)
        AND (
            sqlc.narg (from_ts)::timestamptz IS NULL
            OR mras.started_at >= sqlc.narg (from_ts)
        )
        AND (
            sqlc.narg (to_ts)::timestamptz IS NULL
            OR mras.started_at <= sqlc.narg (to_ts)
        )
    UNION ALL
    SELECT
        mras.ended_at AS occurred_at,
        'runtime.app_session.ended'::text AS event_type,
        CASE
            WHEN mras.status = 'CRASHED' THEN 'warning'
            ELSE 'info'
        END AS severity,
        mras.machine_id,
        'machine'::text AS actor_type,
        NULL::uuid AS actor_account_id,
        mras.operator_session_id,
        mras.machine_session_id,
        'machine_runtime_app_session'::text AS resource_type,
        mras.id::text AS resource_id,
        NULL::uuid AS order_id,
        NULL::uuid AS payment_id,
        NULL::uuid AS vend_session_id,
        NULL::text AS request_id,
        NULL::uuid AS correlation_id,
        COALESCE(mras.end_reason, mras.status) AS reason,
        NULL::text AS error_code,
        mras.status AS summary,
        jsonb_build_object(
            'end_reason', mras.end_reason,
            'boot_id', mras.boot_id
        ) AS metadata
    FROM
        machine_runtime_app_sessions mras
    WHERE
        mras.machine_id = sqlc.arg (machine_id)
        AND mras.ended_at IS NOT NULL
        AND (
            sqlc.narg (from_ts)::timestamptz IS NULL
            OR mras.ended_at >= sqlc.narg (from_ts)
        )
        AND (
            sqlc.narg (to_ts)::timestamptz IS NULL
            OR mras.ended_at <= sqlc.narg (to_ts)
        )
    UNION ALL
    SELECT
        mos.started_at AS occurred_at,
        'operator.session.started'::text AS event_type,
        'info'::text AS severity,
        mos.machine_id,
        mos.actor_type,
        mos.actor_account_id,
        mos.id AS operator_session_id,
        NULL::uuid AS machine_session_id,
        'machine_operator_session'::text AS resource_type,
        mos.id::text AS resource_id,
        NULL::uuid AS order_id,
        NULL::uuid AS payment_id,
        NULL::uuid AS vend_session_id,
        NULL::text AS request_id,
        mos.correlation_id,
        NULL::text AS reason,
        NULL::text AS error_code,
        mos.status AS summary,
        jsonb_build_object(
            'technician_id', mos.technician_id
        ) AS metadata
    FROM
        machine_operator_sessions mos
    WHERE
        mos.machine_id = sqlc.arg (machine_id)
        AND (
            sqlc.narg (from_ts)::timestamptz IS NULL
            OR mos.started_at >= sqlc.narg (from_ts)
        )
        AND (
            sqlc.narg (to_ts)::timestamptz IS NULL
            OR mos.started_at <= sqlc.narg (to_ts)
        )
        AND (
            sqlc.narg (operator_session_id)::uuid IS NULL
            OR mos.id = sqlc.narg (operator_session_id)
        )
    UNION ALL
    SELECT
        COALESCE(mos.ended_at, mos.updated_at) AS occurred_at,
        'operator.session.ended'::text AS event_type,
        'info'::text AS severity,
        mos.machine_id,
        mos.actor_type,
        mos.actor_account_id,
        mos.id AS operator_session_id,
        NULL::uuid AS machine_session_id,
        'machine_operator_session'::text AS resource_type,
        mos.id::text AS resource_id,
        NULL::uuid AS order_id,
        NULL::uuid AS payment_id,
        NULL::uuid AS vend_session_id,
        NULL::text AS request_id,
        mos.correlation_id,
        mos.end_reason AS reason,
        NULL::text AS error_code,
        mos.status AS summary,
        '{}'::jsonb AS metadata
    FROM
        machine_operator_sessions mos
    WHERE
        mos.machine_id = sqlc.arg (machine_id)
        AND mos.ended_at IS NOT NULL
        AND (
            sqlc.narg (from_ts)::timestamptz IS NULL
            OR mos.ended_at >= sqlc.narg (from_ts)
        )
        AND (
            sqlc.narg (to_ts)::timestamptz IS NULL
            OR mos.ended_at <= sqlc.narg (to_ts)
        )
        AND (
            sqlc.narg (operator_session_id)::uuid IS NULL
            OR mos.id = sqlc.narg (operator_session_id)
        )
    UNION ALL
    SELECT
        ms.issued_at AS occurred_at,
        'runtime.session.issued'::text AS event_type,
        'info'::text AS severity,
        ms.machine_id,
        'system'::text AS actor_type,
        NULL::uuid AS actor_account_id,
        NULL::uuid AS operator_session_id,
        ms.id AS machine_session_id,
        'machine_session'::text AS resource_type,
        ms.id::text AS resource_id,
        NULL::uuid AS order_id,
        NULL::uuid AS payment_id,
        NULL::uuid AS vend_session_id,
        NULL::text AS request_id,
        NULL::uuid AS correlation_id,
        NULL::text AS reason,
        NULL::text AS error_code,
        ms.status AS summary,
        jsonb_build_object(
            'credential_version',
            ms.credential_version,
            'expires_at',
            ms.expires_at
        ) AS metadata
    FROM
        machine_sessions ms
    WHERE
        ms.machine_id = sqlc.arg (machine_id)
        AND (
            sqlc.narg (from_ts)::timestamptz IS NULL
            OR ms.issued_at >= sqlc.narg (from_ts)
        )
        AND (
            sqlc.narg (to_ts)::timestamptz IS NULL
            OR ms.issued_at <= sqlc.narg (to_ts)
        )
) AS unified
ORDER BY
    occurred_at DESC
LIMIT sqlc.arg (lim);
