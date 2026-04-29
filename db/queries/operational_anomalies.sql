-- P2.4 operational detectors (inventory_anomalies rows; dedupe via NOT EXISTS on open fingerprint).
-- name: AnomaliesInsertMachineOfflineTooLong :execrows
INSERT INTO inventory_anomalies (
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
    'machine_offline_too_long'::text,
    ('offline_long|'::text || m.id::text),
    jsonb_build_object(
        'last_seen_at',
        m.last_seen_at,
        'threshold',
        '2 hours'
    ),
    now()
FROM
    machines m
WHERE
    m.organization_id = $1
    AND m.last_seen_at IS NOT NULL
    AND m.last_seen_at < now() - interval '2 hours'
    AND m.status NOT IN ('retired', 'decommissioned', 'draft', 'suspended')
    AND NOT EXISTS (
        SELECT
            1
        FROM
            inventory_anomalies ia
        WHERE
            ia.machine_id = m.id
            AND ia.anomaly_type = 'machine_offline_too_long'
            AND ia.fingerprint = ('offline_long|'::text || m.id::text)
            AND ia.status = 'open'
    );

-- name: AnomaliesInsertRepeatedVendFailure :execrows
INSERT INTO inventory_anomalies (
    organization_id,
    machine_id,
    anomaly_type,
    fingerprint,
    payload,
    detected_at
)
SELECT
    m.organization_id,
    c.machine_id,
    'repeated_vend_failure'::text,
    ('repeated_vend_failure|'::text || c.machine_id::text),
    jsonb_build_object('failed_vend_count_24h', c.cnt),
    now()
FROM (
    SELECT
        vs.machine_id,
        COUNT(*)::bigint AS cnt
    FROM
        vend_sessions vs
    WHERE
        vs.state = 'failed'
        AND vs.completed_at IS NOT NULL
        AND vs.completed_at >= now() - interval '24 hours'
        AND vs.completed_at <= now()
    GROUP BY
        vs.machine_id
    HAVING
        COUNT(*) >= 3
) c
    INNER JOIN machines m ON m.id = c.machine_id
WHERE
    m.organization_id = $1
    AND NOT EXISTS (
        SELECT
            1
        FROM
            inventory_anomalies ia
        WHERE
            ia.machine_id = c.machine_id
            AND ia.anomaly_type = 'repeated_vend_failure'
            AND ia.fingerprint = ('repeated_vend_failure|'::text || c.machine_id::text)
            AND ia.status = 'open'
    );

-- name: AnomaliesInsertRepeatedPaymentFailure :execrows
INSERT INTO inventory_anomalies (
    organization_id,
    machine_id,
    anomaly_type,
    fingerprint,
    payload,
    detected_at
)
SELECT
    m.organization_id,
    c.machine_id,
    'repeated_payment_failure'::text,
    ('repeated_payment_failure|'::text || c.machine_id::text),
    jsonb_build_object('failed_payment_count_24h', c.cnt),
    now()
FROM (
    SELECT
        o.machine_id,
        COUNT(*)::bigint AS cnt
    FROM
        payments p
        INNER JOIN orders o ON o.id = p.order_id
    WHERE
        p.state = 'failed'
        AND p.updated_at >= now() - interval '24 hours'
        AND p.updated_at <= now()
    GROUP BY
        o.machine_id
    HAVING
        COUNT(*) >= 3
) c
    INNER JOIN machines m ON m.id = c.machine_id
WHERE
    m.organization_id = $1
    AND NOT EXISTS (
        SELECT
            1
        FROM
            inventory_anomalies ia
        WHERE
            ia.machine_id = c.machine_id
            AND ia.anomaly_type = 'repeated_payment_failure'
            AND ia.fingerprint = ('repeated_payment_failure|'::text || c.machine_id::text)
            AND ia.status = 'open'
    );

-- name: AnomaliesInsertStockMismatch :execrows
INSERT INTO inventory_anomalies (
    organization_id,
    machine_id,
    anomaly_type,
    fingerprint,
    slot_code,
    product_id,
    payload,
    detected_at
)
SELECT
    m.organization_id,
    mss.machine_id,
    'stock_mismatch'::text,
    (
        'stock_mismatch|'::text || mss.machine_id::text || '|' || mss.planogram_id::text || '|' || mss.slot_index::text
    ),
    NULL::text,
    s.product_id,
    jsonb_build_object(
        'current_quantity',
        mss.current_quantity,
        'max_quantity',
        s.max_quantity
    ),
    now()
FROM
    machine_slot_state mss
    INNER JOIN slots s ON s.planogram_id = mss.planogram_id
    AND s.slot_index = mss.slot_index
    INNER JOIN machines m ON m.id = mss.machine_id
WHERE
    m.organization_id = $1
    AND s.max_quantity > 0
    AND mss.current_quantity > s.max_quantity
    AND NOT EXISTS (
        SELECT
            1
        FROM
            inventory_anomalies ia
        WHERE
            ia.machine_id = mss.machine_id
            AND ia.anomaly_type = 'stock_mismatch'
            AND ia.fingerprint = (
                'stock_mismatch|'::text || mss.machine_id::text || '|' || mss.planogram_id::text || '|' || mss.slot_index::text
            )
            AND ia.status = 'open'
    );

-- name: AnomaliesInsertNegativeStockAttempt :execrows
INSERT INTO inventory_anomalies (
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
    'negative_stock_attempt'::text,
    ('negative_stock_attempt|'::text || m.id::text),
    jsonb_build_object('window', '7 days', 'hint', 'failed_vend_with_stock_like_reason'),
    now()
FROM
    machines m
WHERE
    m.organization_id = $1
    AND EXISTS (
        SELECT
            1
        FROM
            vend_sessions vs
        WHERE
            vs.machine_id = m.id
            AND vs.state = 'failed'
            AND vs.completed_at >= now() - interval '7 days'
            AND vs.failure_reason IS NOT NULL
            AND (
                vs.failure_reason ILIKE '%stock%'
                OR vs.failure_reason ILIKE '%empty%'
                OR vs.failure_reason ILIKE '%out%'
            )
    )
    AND NOT EXISTS (
        SELECT
            1
        FROM
            inventory_anomalies ia
        WHERE
            ia.machine_id = m.id
            AND ia.anomaly_type = 'negative_stock_attempt'
            AND ia.fingerprint = ('negative_stock_attempt|'::text || m.id::text)
            AND ia.status = 'open'
    );

-- name: AnomaliesInsertHighCashVariance :execrows
INSERT INTO inventory_anomalies (
    organization_id,
    machine_id,
    anomaly_type,
    fingerprint,
    payload,
    detected_at
)
SELECT
    cc.organization_id,
    cc.machine_id,
    'high_cash_variance'::text,
    ('high_cash_variance|'::text || cc.id::text),
    jsonb_build_object(
        'cash_collection_id',
        cc.id,
        'variance_amount_minor',
        cc.variance_amount_minor,
        'requires_review',
        cc.requires_review
    ),
    now()
FROM
    cash_collections cc
WHERE
    cc.organization_id = $1
    AND cc.lifecycle_status = 'closed'
    AND abs(cc.variance_amount_minor) >= 50000
    AND cc.collected_at >= now() - interval '90 days'
    AND NOT EXISTS (
        SELECT
            1
        FROM
            inventory_anomalies ia
        WHERE
            ia.machine_id = cc.machine_id
            AND ia.anomaly_type = 'high_cash_variance'
            AND ia.fingerprint = ('high_cash_variance|'::text || cc.id::text)
            AND ia.status = 'open'
    );

-- name: AnomaliesInsertCommandFailureSpike :execrows
INSERT INTO inventory_anomalies (
    organization_id,
    machine_id,
    anomaly_type,
    fingerprint,
    payload,
    detected_at
)
SELECT
    m.organization_id,
    c.machine_id,
    'command_failure_spike'::text,
    ('command_failure_spike|'::text || c.machine_id::text),
    jsonb_build_object('failed_command_attempts_24h', c.cnt),
    now()
FROM (
    SELECT
        mca.machine_id,
        COUNT(*)::bigint AS cnt
    FROM
        machine_command_attempts mca
    WHERE
        mca.status IN ('failed', 'nack', 'ack_timeout')
        AND mca.sent_at >= now() - interval '24 hours'
        AND mca.sent_at <= now()
    GROUP BY
        mca.machine_id
    HAVING
        COUNT(*) >= 5
) c
    INNER JOIN machines m ON m.id = c.machine_id
WHERE
    m.organization_id = $1
    AND NOT EXISTS (
        SELECT
            1
        FROM
            inventory_anomalies ia
        WHERE
            ia.machine_id = c.machine_id
            AND ia.anomaly_type = 'command_failure_spike'
            AND ia.fingerprint = ('command_failure_spike|'::text || c.machine_id::text)
            AND ia.status = 'open'
    );

-- name: AnomaliesInsertTelemetryMissing :execrows
INSERT INTO inventory_anomalies (
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
    'telemetry_missing'::text,
    ('telemetry_missing|'::text || m.id::text),
    jsonb_build_object(
        'snapshot_updated_at',
        mcs.updated_at,
        'threshold',
        '6 hours'
    ),
    now()
FROM
    machines m
    LEFT JOIN machine_current_snapshot mcs ON mcs.machine_id = m.id
WHERE
    m.organization_id = $1
    AND m.status IN ('active', 'online', 'provisioned')
    AND (
        mcs.updated_at IS NULL
        OR mcs.updated_at < now() - interval '6 hours'
    )
    AND NOT EXISTS (
        SELECT
            1
        FROM
            inventory_anomalies ia
        WHERE
            ia.machine_id = m.id
            AND ia.anomaly_type = 'telemetry_missing'
            AND ia.fingerprint = ('telemetry_missing|'::text || m.id::text)
            AND ia.status = 'open'
    );

-- name: AnomaliesInsertLowStockThreshold :execrows
INSERT INTO inventory_anomalies (
    organization_id,
    machine_id,
    anomaly_type,
    fingerprint,
    slot_code,
    product_id,
    payload,
    detected_at
)
SELECT
    m.organization_id,
    mss.machine_id,
    'low_stock_threshold'::text,
    (
        'low_stock_threshold|'::text || mss.machine_id::text || '|' || mss.planogram_id::text || '|' || mss.slot_index::text
    ),
    NULL::text,
    s.product_id,
    jsonb_build_object(
        'current_quantity',
        mss.current_quantity,
        'max_quantity',
        s.max_quantity,
        'threshold_ratio',
        0.10
    ),
    now()
FROM
    machine_slot_state mss
    INNER JOIN slots s ON s.planogram_id = mss.planogram_id
    AND s.slot_index = mss.slot_index
    INNER JOIN machines m ON m.id = mss.machine_id
WHERE
    m.organization_id = $1
    AND s.max_quantity >= 5
    AND mss.current_quantity > 0
    AND mss.current_quantity::float <= CEIL(s.max_quantity::float * 0.1)
    AND NOT EXISTS (
        SELECT
            1
        FROM
            inventory_anomalies ia
        WHERE
            ia.machine_id = mss.machine_id
            AND ia.anomaly_type = 'low_stock_threshold'
            AND ia.fingerprint = (
                'low_stock_threshold|'::text || mss.machine_id::text || '|' || mss.planogram_id::text || '|' || mss.slot_index::text
            )
            AND ia.status = 'open'
    );

-- name: AnomaliesInsertSoldOutSoonEstimate :execrows
INSERT INTO inventory_anomalies (
    organization_id,
    machine_id,
    anomaly_type,
    fingerprint,
    slot_code,
    product_id,
    payload,
    detected_at
)
SELECT
    m.organization_id,
    mss.machine_id,
    'product_sold_out_soon_estimate'::text,
    (
        'sold_out_soon|'::text || mss.machine_id::text || '|' || mss.planogram_id::text || '|' || mss.slot_index::text
    ),
    NULL::text,
    s.product_id,
    jsonb_build_object(
        'current_quantity',
        mss.current_quantity,
        'daily_sales_velocity',
        ROUND(v.daily_vel::numeric, 4),
        'estimated_days_of_supply',
        CASE WHEN v.daily_vel > 0 THEN
            ROUND((mss.current_quantity::float / v.daily_vel)::numeric, 2)
        ELSE
            NULL
        END,
        'threshold_days',
        3
    ),
    now()
FROM
    machine_slot_state mss
    INNER JOIN (
        SELECT
            vs.machine_id,
            vs.slot_index,
            (COUNT(*)::float / 7.0) AS daily_vel
        FROM
            vend_sessions vs
        WHERE
            vs.state = 'success'
            AND vs.completed_at >= now() - interval '7 days'
            AND vs.completed_at <= now()
        GROUP BY
            vs.machine_id,
            vs.slot_index
    ) v ON v.machine_id = mss.machine_id
    AND v.slot_index = mss.slot_index
    INNER JOIN slots s ON s.planogram_id = mss.planogram_id
    AND s.slot_index = mss.slot_index
    INNER JOIN machines m ON m.id = mss.machine_id
WHERE
    m.organization_id = $1
    AND mss.current_quantity > 0
    AND v.daily_vel > 0
    AND (mss.current_quantity::float / v.daily_vel) <= 3
    AND NOT EXISTS (
        SELECT
            1
        FROM
            inventory_anomalies ia
        WHERE
            ia.machine_id = mss.machine_id
            AND ia.anomaly_type = 'product_sold_out_soon_estimate'
            AND ia.fingerprint = (
                'sold_out_soon|'::text || mss.machine_id::text || '|' || mss.planogram_id::text || '|' || mss.slot_index::text
            )
            AND ia.status = 'open'
    );

-- name: AnomaliesGetByOrgAndID :one
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
    AND ia.id = sqlc.arg('anomaly_id');

-- name: AdminOpsIgnoreInventoryAnomaly :one
UPDATE inventory_anomalies ia
SET
    status = 'ignored',
    resolved_at = now(),
    resolved_by = $3,
    resolution_note = $4,
    updated_at = now()
WHERE
    ia.id = $1
    AND ia.organization_id = $2
    AND ia.status = 'open'
RETURNING
    ia.id;
