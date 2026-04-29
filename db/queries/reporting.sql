-- name: ReportingSalesTotals :one
SELECT
    COALESCE(SUM(o.total_minor), 0)::bigint AS gross_total_minor,
    COALESCE(SUM(o.tax_minor), 0)::bigint AS tax_minor,
    COALESCE(SUM(o.subtotal_minor), 0)::bigint AS subtotal_minor,
    COUNT(*)::bigint AS order_count
FROM orders o
WHERE
    o.organization_id = $1
    AND o.created_at >= $2::timestamptz
    AND o.created_at < $3::timestamptz;

-- name: ReportingSalesByDay :many
SELECT
    (date_trunc('day', o.created_at AT TIME ZONE $4::text) AT TIME ZONE $4::text)::timestamptz AS bucket_start,
    COUNT(*)::bigint AS order_count,
    COALESCE(SUM(o.total_minor), 0)::bigint AS total_minor,
    COALESCE(SUM(o.subtotal_minor), 0)::bigint AS subtotal_minor,
    COALESCE(SUM(o.tax_minor), 0)::bigint AS tax_minor
FROM orders o
WHERE
    o.organization_id = $1
    AND o.created_at >= $2::timestamptz
    AND o.created_at < $3::timestamptz
GROUP BY
    1
ORDER BY
    1 ASC;

-- name: ReportingSalesBySite :many
SELECT
    m.site_id,
    COUNT(*)::bigint AS order_count,
    COALESCE(SUM(o.total_minor), 0)::bigint AS total_minor,
    COALESCE(SUM(o.subtotal_minor), 0)::bigint AS subtotal_minor,
    COALESCE(SUM(o.tax_minor), 0)::bigint AS tax_minor
FROM orders o
INNER JOIN machines m ON m.id = o.machine_id
WHERE
    o.organization_id = $1
    AND o.created_at >= $2::timestamptz
    AND o.created_at < $3::timestamptz
GROUP BY
    m.site_id
ORDER BY
    total_minor DESC;

-- name: ReportingSalesByPaymentProvider :many
SELECT
    p.provider,
    COUNT(DISTINCT o.id)::bigint AS order_count,
    COALESCE(SUM(o.total_minor), 0)::bigint AS total_minor,
    COALESCE(SUM(o.subtotal_minor), 0)::bigint AS subtotal_minor,
    COALESCE(SUM(o.tax_minor), 0)::bigint AS tax_minor
FROM orders o
INNER JOIN payments p ON p.order_id = o.id
    AND p.state IN ('authorized', 'captured')
WHERE
    o.organization_id = $1
    AND o.created_at >= $2::timestamptz
    AND o.created_at < $3::timestamptz
GROUP BY
    p.provider
ORDER BY
    total_minor DESC;

-- name: ReportingPaymentsByDay :many
SELECT
    (date_trunc('day', p.created_at AT TIME ZONE $4::text) AT TIME ZONE $4::text)::timestamptz AS bucket_start,
    COUNT(*)::bigint AS payment_count,
    COALESCE(SUM(p.amount_minor), 0)::bigint AS amount_minor
FROM payments p
INNER JOIN orders o ON o.id = p.order_id
WHERE
    o.organization_id = $1
    AND p.created_at >= $2::timestamptz
    AND p.created_at < $3::timestamptz
GROUP BY
    1
ORDER BY
    1 ASC;

-- name: ReportingSalesByMachine :many
SELECT
    o.machine_id,
    COUNT(*)::bigint AS order_count,
    COALESCE(SUM(o.total_minor), 0)::bigint AS total_minor,
    COALESCE(SUM(o.subtotal_minor), 0)::bigint AS subtotal_minor,
    COALESCE(SUM(o.tax_minor), 0)::bigint AS tax_minor
FROM orders o
WHERE
    o.organization_id = $1
    AND o.created_at >= $2::timestamptz
    AND o.created_at < $3::timestamptz
GROUP BY
    o.machine_id
ORDER BY
    total_minor DESC
LIMIT 500;

-- name: ReportingPaymentsTotals :one
SELECT
    COUNT(*) FILTER (
        WHERE
            p.state = 'authorized'
    )::bigint AS authorized_count,
    COUNT(*) FILTER (
        WHERE
            p.state = 'captured'
    )::bigint AS captured_count,
    COUNT(*) FILTER (
        WHERE
            p.state = 'failed'
    )::bigint AS failed_count,
    COUNT(*) FILTER (
        WHERE
            p.state = 'refunded'
    )::bigint AS refunded_count,
    COALESCE(SUM(p.amount_minor) FILTER (
        WHERE
            p.state = 'captured'
    ), 0)::bigint AS captured_amount_minor,
    COALESCE(SUM(p.amount_minor) FILTER (
        WHERE
            p.state = 'authorized'
    ), 0)::bigint AS authorized_amount_minor,
    COALESCE(SUM(p.amount_minor) FILTER (
        WHERE
            p.state = 'failed'
    ), 0)::bigint AS failed_amount_minor,
    COALESCE(SUM(p.amount_minor) FILTER (
        WHERE
            p.state = 'refunded'
    ), 0)::bigint AS refunded_amount_minor
FROM payments p
INNER JOIN orders o ON o.id = p.order_id
WHERE
    o.organization_id = $1
    AND p.created_at >= $2::timestamptz
    AND p.created_at < $3::timestamptz;

-- name: ReportingPaymentsByMethodAndState :many
SELECT
    p.provider,
    p.state,
    COUNT(*)::bigint AS payment_count,
    COALESCE(SUM(p.amount_minor), 0)::bigint AS amount_minor
FROM payments p
INNER JOIN orders o ON o.id = p.order_id
WHERE
    o.organization_id = $1
    AND p.created_at >= $2::timestamptz
    AND p.created_at < $3::timestamptz
GROUP BY
    p.provider,
    p.state
ORDER BY
    amount_minor DESC;

-- name: ReportingFleetMachinesByStatus :many
SELECT
    m.status,
    COUNT(*)::bigint AS machine_count
FROM machines m
WHERE
    m.organization_id = $1
GROUP BY
    m.status
ORDER BY
    machine_count DESC;

-- name: ReportingFleetIncidentsByStatus :many
SELECT
    i.status,
    COUNT(*)::bigint AS incident_count
FROM incidents i
WHERE
    i.organization_id = $1
    AND i.opened_at >= $2::timestamptz
    AND i.opened_at < $3::timestamptz
GROUP BY
    i.status
ORDER BY
    incident_count DESC;

-- name: ReportingFleetMachineIncidentsBySeverity :many
SELECT
    mi.severity,
    COUNT(*)::bigint AS incident_count
FROM machine_incidents mi
INNER JOIN machines m ON m.id = mi.machine_id
WHERE
    m.organization_id = $1
    AND mi.opened_at >= $2::timestamptz
    AND mi.opened_at < $3::timestamptz
GROUP BY
    mi.severity
ORDER BY
    incident_count DESC;

-- name: ReportingInventoryExceptionsCount :one
SELECT
    count(*)::bigint AS cnt
FROM machine_slot_state mss
INNER JOIN machines m ON m.id = mss.machine_id
LEFT JOIN slots s ON s.planogram_id = mss.planogram_id
    AND s.slot_index = mss.slot_index
WHERE
    m.organization_id = $1
    AND (
        (
            $2::boolean IS TRUE
            AND mss.current_quantity <= 0
        )
        OR (
            $3::boolean IS TRUE
            AND COALESCE(s.max_quantity, 0) > 0
            AND mss.current_quantity > 0
            AND mss.current_quantity::float / NULLIF(s.max_quantity, 0)::float < 0.15
        )
    );

-- name: ReportingInventoryExceptions :many
SELECT
    m.id AS machine_id,
    m.name AS machine_name,
    m.serial_number AS machine_serial_number,
    m.status AS machine_status,
    mss.planogram_id,
    pg.name AS planogram_name,
    mss.slot_index,
    mss.current_quantity,
    COALESCE(s.max_quantity, 0)::int AS max_quantity,
    s.product_id,
    pr.name AS product_name,
    pr.sku AS product_sku,
    (mss.current_quantity <= 0) AS out_of_stock,
    (
        COALESCE(s.max_quantity, 0) > 0
        AND mss.current_quantity > 0
        AND mss.current_quantity::float / NULLIF(s.max_quantity, 0)::float < 0.15
    ) AS low_stock
FROM machine_slot_state mss
INNER JOIN machines m ON m.id = mss.machine_id
INNER JOIN planograms pg ON pg.id = mss.planogram_id
LEFT JOIN slots s ON s.planogram_id = mss.planogram_id
    AND s.slot_index = mss.slot_index
LEFT JOIN products pr ON pr.id = s.product_id
WHERE
    m.organization_id = $1
    AND (
        (
            $2::boolean IS TRUE
            AND mss.current_quantity <= 0
        )
        OR (
            $3::boolean IS TRUE
            AND COALESCE(s.max_quantity, 0) > 0
            AND mss.current_quantity > 0
            AND mss.current_quantity::float / NULLIF(s.max_quantity, 0)::float < 0.15
        )
    )
ORDER BY
    m.name ASC,
    mss.slot_index ASC
LIMIT $4 OFFSET $5;

-- name: ReportingCashCollectionsForOrganization :many
SELECT
    cc.id,
    cc.organization_id,
    cc.machine_id,
    m.site_id,
    m.serial_number AS machine_serial_number,
    s.name AS site_name,
    cc.collected_at,
    cc.opened_at,
    cc.closed_at,
    cc.lifecycle_status,
    cc.amount_minor,
    cc.expected_amount_minor,
    cc.variance_amount_minor,
    cc.currency,
    cc.reconciliation_status,
    cc.created_at
FROM
    cash_collections cc
    INNER JOIN machines m ON m.id = cc.machine_id
        AND m.organization_id = cc.organization_id
    INNER JOIN sites s ON s.id = m.site_id
WHERE
    cc.organization_id = $1
    AND cc.collected_at >= $2::timestamptz
    AND cc.collected_at < $3::timestamptz
    AND (
        $4::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.site_id = $4::uuid)
    AND (
        $5::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR cc.machine_id = $5::uuid)
ORDER BY
    cc.collected_at DESC;

-- name: ReportingPaymentSettlement :many
SELECT
    (date_trunc('day', p.created_at AT TIME ZONE $4::text) AT TIME ZONE $4::text)::timestamptz AS bucket_start,
    p.provider,
    p.state,
    p.settlement_status,
    p.reconciliation_status,
    COUNT(*)::bigint AS payment_count,
    COALESCE(SUM(p.amount_minor), 0)::bigint AS amount_minor
FROM payments p
INNER JOIN orders o ON o.id = p.order_id
WHERE
    o.organization_id = $1
    AND p.created_at >= $2::timestamptz
    AND p.created_at < $3::timestamptz
GROUP BY
    1,
    p.provider,
    p.state,
    p.settlement_status,
    p.reconciliation_status
ORDER BY
    1 ASC,
    p.provider ASC,
    p.settlement_status ASC,
    p.state ASC;

-- name: ReportingRefundsCount :one
SELECT
    COUNT(*)::bigint AS cnt
FROM refunds r
INNER JOIN orders o ON o.id = r.order_id
WHERE
    o.organization_id = $1
    AND r.created_at >= $2::timestamptz
    AND r.created_at < $3::timestamptz;

-- name: ReportingRefunds :many
SELECT
    r.id AS refund_id,
    r.payment_id,
    r.order_id,
    o.machine_id,
    r.amount_minor,
    r.currency,
    r.state,
    r.reason,
    r.reconciliation_status,
    r.settlement_status,
    r.created_at
FROM refunds r
INNER JOIN orders o ON o.id = r.order_id
WHERE
    o.organization_id = $1
    AND r.created_at >= $2::timestamptz
    AND r.created_at < $3::timestamptz
ORDER BY
    r.created_at DESC
LIMIT $4 OFFSET $5;

-- name: ReportingCashCollectionsCount :one
SELECT
    COUNT(*)::bigint AS cnt
FROM cash_collections cc
INNER JOIN machines m ON m.id = cc.machine_id
    AND m.organization_id = cc.organization_id
WHERE
    cc.organization_id = $1
    AND cc.collected_at >= $2::timestamptz
    AND cc.collected_at < $3::timestamptz
    AND (
        $4::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.site_id = $4::uuid)
    AND (
        $5::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR cc.machine_id = $5::uuid);

-- name: ReportingMachineHealthCount :one
SELECT
    COUNT(*)::bigint AS cnt
FROM machines m
WHERE
    m.organization_id = $1;

-- name: ReportingMachineHealth :many
SELECT
    m.id AS machine_id,
    m.site_id,
    s.name AS site_name,
    m.serial_number,
    m.name AS machine_name,
    m.status,
    m.last_seen_at,
    CASE
        WHEN m.status IN ('offline', 'suspended', 'maintenance', 'retired', 'decommissioned', 'compromised') THEN TRUE
        WHEN m.last_seen_at IS NULL THEN TRUE
        WHEN m.last_seen_at < $2::timestamptz THEN TRUE
        ELSE FALSE
    END AS offline
FROM machines m
INNER JOIN sites s ON s.id = m.site_id
WHERE
    m.organization_id = $1
ORDER BY
    offline DESC,
    m.last_seen_at ASC NULLS FIRST,
    m.name ASC
LIMIT $3 OFFSET $4;

-- name: ReportingFailedVendsCount :one
SELECT
    COUNT(*)::bigint AS cnt
FROM vend_sessions vs
INNER JOIN orders o ON o.id = vs.order_id
WHERE
    o.organization_id = $1
    AND vs.state = 'failed'
    AND COALESCE(vs.completed_at, vs.created_at) >= $2::timestamptz
    AND COALESCE(vs.completed_at, vs.created_at) < $3::timestamptz;

-- name: ReportingFailedVends :many
SELECT
    vs.id AS vend_session_id,
    vs.order_id,
    vs.machine_id,
    vs.slot_index,
    vs.product_id,
    vs.failure_reason,
    vs.started_at,
    vs.completed_at,
    vs.created_at,
    o.total_minor,
    o.currency,
    o.status AS order_status
FROM vend_sessions vs
INNER JOIN orders o ON o.id = vs.order_id
WHERE
    o.organization_id = $1
    AND vs.state = 'failed'
    AND COALESCE(vs.completed_at, vs.created_at) >= $2::timestamptz
    AND COALESCE(vs.completed_at, vs.created_at) < $3::timestamptz
ORDER BY
    COALESCE(vs.completed_at, vs.created_at) DESC
LIMIT $4 OFFSET $5;

-- name: ReportingReconciliationQueueCount :one
SELECT
    COUNT(*)::bigint AS cnt
FROM commerce_reconciliation_cases c
WHERE
    c.organization_id = $1
    AND c.first_detected_at >= $2::timestamptz
    AND c.first_detected_at < $3::timestamptz
    AND c.status IN ('open', 'reviewing');

-- name: ReportingReconciliationQueue :many
SELECT
    c.id,
    c.case_type,
    c.status,
    c.severity,
    c.order_id,
    c.payment_id,
    c.vend_session_id,
    c.refund_id,
    c.provider,
    c.reason,
    c.first_detected_at,
    c.last_detected_at
FROM commerce_reconciliation_cases c
WHERE
    c.organization_id = $1
    AND c.first_detected_at >= $2::timestamptz
    AND c.first_detected_at < $3::timestamptz
    AND c.status IN ('open', 'reviewing')
ORDER BY
    c.severity DESC,
    c.last_detected_at DESC
LIMIT $4 OFFSET $5;
