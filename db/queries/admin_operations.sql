-- Machine operational health (fleet-wide).
-- name: AdminOpsListMachineHealth :many
SELECT
    m.id AS machine_id,
    m.status::text AS status,
    m.last_seen_at AS last_seen_at,
    mcs.last_check_in_at AS last_checkin_at,
    mcs.app_version AS app_version,
    COALESCE(mcs.reported_state ->> 'config_version', '')::text AS config_version,
    COALESCE(mcs.reported_state ->> 'catalog_version', '')::text AS catalog_version,
    COALESCE(mcs.reported_state ->> 'media_version', '')::text AS media_version,
    (
        SELECT
            (
                ts.disconnected_at IS NULL
                AND ts.connected_at IS NOT NULL
            )
        FROM
            machine_transport_sessions ts
        WHERE
            ts.machine_id = m.id
        ORDER BY
            ts.connected_at DESC
        LIMIT
            1
    ) AS mqtt_connected,
    (
        SELECT
            COUNT(DISTINCT cl.id)::bigint
        FROM
            command_ledger cl
            INNER JOIN LATERAL (
                SELECT
                    mca.status
                FROM
                    machine_command_attempts mca
                WHERE
                    mca.command_id = cl.id
                ORDER BY
                    mca.attempt_no DESC
                LIMIT
                    1
            ) la ON TRUE
        WHERE
            cl.machine_id = m.id
            AND COALESCE(la.status, 'pending') IN ('pending', 'sent')
    ) AS pending_command_count,
    (
        SELECT
            COUNT(*)::bigint
        FROM
            command_ledger cl2
            INNER JOIN LATERAL (
                SELECT
                    mca.status
                FROM
                    machine_command_attempts mca
                WHERE
                    mca.command_id = cl2.id
                ORDER BY
                    mca.attempt_no DESC
                LIMIT
                    1
            ) la2 ON TRUE
        WHERE
            cl2.machine_id = m.id
            AND la2.status IN ('failed', 'nack', 'ack_timeout')
    ) AS failed_command_count,
    (
        SELECT
            COUNT(*)::bigint
        FROM
            inventory_anomalies ia
        WHERE
            ia.machine_id = m.id
            AND ia.status = 'open'
    ) AS inventory_anomaly_count,
    COALESCE(
        (
            SELECT
                mi.code
            FROM
                machine_incidents mi
            WHERE
                mi.machine_id = m.id
            ORDER BY
                mi.opened_at DESC
            LIMIT
                1
        ),
        ''
    )::text AS last_error_code,
    CASE
        WHEN mcs.updated_at IS NOT NULL THEN EXTRACT(
            epoch
            FROM
                (now() - mcs.updated_at)
        )::bigint
        ELSE (-1)::bigint
    END AS telemetry_freshness_seconds
FROM
    machines m
    LEFT JOIN machine_current_snapshot mcs ON mcs.machine_id = m.id
WHERE
    m.organization_id = $1
ORDER BY
    m.updated_at DESC
LIMIT
    $2 OFFSET $3;

-- name: AdminOpsGetMachineHealthByID :one
SELECT
    m.id AS machine_id,
    m.status::text AS status,
    m.last_seen_at AS last_seen_at,
    mcs.last_check_in_at AS last_checkin_at,
    mcs.app_version AS app_version,
    COALESCE(mcs.reported_state ->> 'config_version', '')::text AS config_version,
    COALESCE(mcs.reported_state ->> 'catalog_version', '')::text AS catalog_version,
    COALESCE(mcs.reported_state ->> 'media_version', '')::text AS media_version,
    (
        SELECT
            (
                ts.disconnected_at IS NULL
                AND ts.connected_at IS NOT NULL
            )
        FROM
            machine_transport_sessions ts
        WHERE
            ts.machine_id = m.id
        ORDER BY
            ts.connected_at DESC
        LIMIT
            1
    ) AS mqtt_connected,
    (
        SELECT
            COUNT(DISTINCT cl.id)::bigint
        FROM
            command_ledger cl
            INNER JOIN LATERAL (
                SELECT
                    mca.status
                FROM
                    machine_command_attempts mca
                WHERE
                    mca.command_id = cl.id
                ORDER BY
                    mca.attempt_no DESC
                LIMIT
                    1
            ) la ON TRUE
        WHERE
            cl.machine_id = m.id
            AND COALESCE(la.status, 'pending') IN ('pending', 'sent')
    ) AS pending_command_count,
    (
        SELECT
            COUNT(*)::bigint
        FROM
            command_ledger cl2
            INNER JOIN LATERAL (
                SELECT
                    mca.status
                FROM
                    machine_command_attempts mca
                WHERE
                    mca.command_id = cl2.id
                ORDER BY
                    mca.attempt_no DESC
                LIMIT
                    1
            ) la2 ON TRUE
        WHERE
            cl2.machine_id = m.id
            AND la2.status IN ('failed', 'nack', 'ack_timeout')
    ) AS failed_command_count,
    (
        SELECT
            COUNT(*)::bigint
        FROM
            inventory_anomalies ia
        WHERE
            ia.machine_id = m.id
            AND ia.status = 'open'
    ) AS inventory_anomaly_count,
    COALESCE(
        (
            SELECT
                mi.code
            FROM
                machine_incidents mi
            WHERE
                mi.machine_id = m.id
            ORDER BY
                mi.opened_at DESC
            LIMIT
                1
        ),
        ''
    )::text AS last_error_code,
    CASE
        WHEN mcs.updated_at IS NOT NULL THEN EXTRACT(
            epoch
            FROM
                (now() - mcs.updated_at)
        )::bigint
        ELSE (-1)::bigint
    END AS telemetry_freshness_seconds
FROM
    machines m
    LEFT JOIN machine_current_snapshot mcs ON mcs.machine_id = m.id
WHERE
    m.organization_id = $1
    AND m.id = $2;

-- name: AdminOpsGetMachineShadowDesiredJSON :one
SELECT
    desired_state
FROM
    machine_shadow
WHERE
    machine_id = $1;

-- Unified timeline for operator troubleshooting.
-- name: AdminOpsMachineTimeline :many
SELECT
    occurred_at,
    event_kind,
    title,
    payload,
    ref_id
FROM
    (
        SELECT
            cl.created_at AS occurred_at,
            'command'::text AS event_kind,
            cl.command_type AS title,
            jsonb_build_object(
                'commandId',
                cl.id,
                'sequence',
                cl.sequence,
                'correlationId',
                cl.correlation_id
            ) AS payload,
            cl.id::text AS ref_id
        FROM
            command_ledger cl
        WHERE
            cl.machine_id = $1
            AND cl.organization_id = $2
        UNION ALL
        SELECT
            mca.sent_at AS occurred_at,
            'command_attempt'::text AS event_kind,
            mca.status AS title,
            jsonb_build_object(
                'commandId',
                mca.command_id,
                'attemptNo',
                mca.attempt_no,
                'timeoutReason',
                mca.timeout_reason
            ) AS payload,
            mca.id::text AS ref_id
        FROM
            machine_command_attempts mca
            INNER JOIN machines mm ON mm.id = mca.machine_id
        WHERE
            mca.machine_id = $1
            AND mm.organization_id = $2
        UNION ALL
        SELECT
            ot.occurred_at AS occurred_at,
            'commerce'::text AS event_kind,
            ot.event_type AS title,
            ot.payload AS payload,
            ot.id::text AS ref_id
        FROM
            order_timelines ot
            INNER JOIN orders o ON o.id = ot.order_id
        WHERE
            o.machine_id = $1
            AND o.organization_id = $2
        UNION ALL
        SELECT
            ci.recorded_at AS occurred_at,
            'telemetry.check_in'::text AS event_kind,
            ci.package_name AS title,
            jsonb_build_object(
                'versionName',
                ci.version_name,
                'networkState',
                ci.network_state
            ) AS payload,
            ci.id::text AS ref_id
        FROM
            machine_check_ins ci
        WHERE
            ci.machine_id = $1
            AND ci.organization_id = $2
    ) u
ORDER BY
    occurred_at DESC
LIMIT
    $3 OFFSET $4;

-- name: AdminOpsInsertDetectedNegativeStockAnomalies :execrows
INSERT INTO
    inventory_anomalies (
        organization_id,
        machine_id,
        anomaly_type,
        fingerprint,
        slot_code,
        payload,
        detected_at
    )
SELECT
    m.organization_id,
    mss.machine_id,
    'negative_stock'::text,
    (
        'negative_stock|'::text || mss.machine_id::text || '|' || mss.planogram_id::text || '|' || mss.slot_index::text
    ),
    NULL::text,
    jsonb_build_object('current_quantity', mss.current_quantity),
    now()
FROM
    machine_slot_state mss
    INNER JOIN machines m ON m.id = mss.machine_id
WHERE
    m.organization_id = $1
    AND mss.current_quantity < 0
    AND NOT EXISTS (
        SELECT
            1
        FROM
            inventory_anomalies ia
        WHERE
            ia.machine_id = mss.machine_id
            AND ia.anomaly_type = 'negative_stock'
            AND ia.fingerprint = (
                'negative_stock|'::text || mss.machine_id::text || '|' || mss.planogram_id::text || '|' || mss.slot_index::text
            )
            AND ia.status = 'open'
    );

-- name: AdminOpsInsertDetectedManualAdjustmentAnomalies :execrows
INSERT INTO
    inventory_anomalies (
        organization_id,
        machine_id,
        anomaly_type,
        fingerprint,
        slot_code,
        payload,
        detected_at
    )
SELECT
    ie.organization_id,
    ie.machine_id,
    'manual_adjustment_above_threshold'::text,
    (
        'manual_adj|'::text || ie.id::text
    ),
    ie.slot_code,
    jsonb_build_object(
        'inventory_event_id',
        ie.id,
        'quantity_delta',
        ie.quantity_delta,
        'reason_code',
        ie.reason_code
    ),
    ie.occurred_at
FROM
    inventory_events ie
WHERE
    ie.organization_id = sqlc.arg('organization_id')
    AND ie.event_type = 'adjustment'
    AND abs(ie.quantity_delta) >= sqlc.arg('adjustment_abs_threshold')::int
    AND ie.occurred_at >= now() - (sqlc.arg('lookback_days')::bigint * interval '1 day')
    AND NOT EXISTS (
        SELECT
            1
        FROM
            inventory_anomalies ia
        WHERE
            ia.machine_id = ie.machine_id
            AND ia.anomaly_type = 'manual_adjustment_above_threshold'
            AND ia.fingerprint = ('manual_adj|'::text || ie.id::text)
            AND ia.status = 'open'
    );

-- name: AdminOpsInsertStaleInventorySyncAnomalies :execrows
INSERT INTO
    inventory_anomalies (
        organization_id,
        machine_id,
        anomaly_type,
        fingerprint,
        payload,
        detected_at
    )
SELECT
    m.organization_id,
    m.id,
    'stale_inventory_sync'::text,
    ('stale_sync|'::text || m.id::text),
    jsonb_build_object(
        'published_planogram_version_id',
        m.published_planogram_version_id,
        'last_acknowledged_planogram_version_id',
        snap.last_acknowledged_planogram_version_id
    ),
    now()
FROM
    machines m
    INNER JOIN machine_current_snapshot snap ON snap.machine_id = m.id
WHERE
    m.organization_id = $1
    AND m.published_planogram_version_id IS NOT NULL
    AND snap.last_acknowledged_planogram_version_id IS NOT NULL
    AND m.published_planogram_version_id <> snap.last_acknowledged_planogram_version_id
    AND NOT EXISTS (
        SELECT
            1
        FROM
            inventory_anomalies ia
        WHERE
            ia.machine_id = m.id
            AND ia.anomaly_type = 'stale_inventory_sync'
            AND ia.fingerprint = ('stale_sync|'::text || m.id::text)
            AND ia.status = 'open'
    );

-- name: AdminOpsListInventoryAnomaliesByOrg :many
SELECT
    ia.id,
    ia.organization_id,
    ia.machine_id,
    ia.anomaly_type,
    ia.status,
    ia.fingerprint,
    ia.slot_code,
    ia.product_id,
    ia.payload,
    ia.detected_at,
    ia.resolved_at,
    ia.resolved_by,
    ia.resolution_note,
    ia.created_at,
    ia.updated_at,
    m.name AS machine_name,
    m.serial_number AS machine_serial_number
FROM
    inventory_anomalies ia
    INNER JOIN machines m ON m.id = ia.machine_id
WHERE
    ia.organization_id = sqlc.arg('organization_id')
    AND (
        NOT sqlc.arg('filter_machine')
        OR ia.machine_id = sqlc.arg('machine_id')
    )
ORDER BY
    ia.detected_at DESC
LIMIT
    sqlc.arg('limit_val')
OFFSET
    sqlc.arg('offset_val');

-- name: AdminOpsGetInventoryAnomalyByID :one
SELECT
    ia.id,
    ia.organization_id,
    ia.machine_id,
    ia.anomaly_type,
    ia.status,
    ia.fingerprint,
    ia.slot_code,
    ia.product_id,
    ia.payload,
    ia.detected_at,
    ia.resolved_at,
    ia.resolved_by,
    ia.resolution_note,
    ia.created_at,
    ia.updated_at
FROM
    inventory_anomalies ia
WHERE
    ia.id = $1
    AND ia.organization_id = $2;

-- name: AdminOpsResolveInventoryAnomaly :one
UPDATE inventory_anomalies ia
SET
    status = 'resolved',
    resolved_at = now (),
    resolved_by = $3,
    resolution_note = $4,
    updated_at = now ()
WHERE
    ia.id = $1
    AND ia.organization_id = $2
    AND ia.status = 'open'
RETURNING
    ia.id;
