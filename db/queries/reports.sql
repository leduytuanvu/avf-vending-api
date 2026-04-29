-- Enterprise BI reports (P2.3). Optional UUID filters use nil UUID '00000000-0000-0000-0000-000000000000'::uuid as sentinel (unset).

-- name: ReportingSalesByProduct :many
SELECT
    vs.product_id,
    COALESCE(MAX(pr.name), '')::text AS product_name,
    COALESCE(MAX(pr.sku), '')::text AS product_sku,
    COUNT(*) FILTER (
        WHERE
            vs.state = 'success'
    )::bigint AS success_vends,
    COUNT(*) FILTER (
        WHERE
            vs.state = 'failed'
    )::bigint AS failed_vends,
    COALESCE(
        SUM(
            CASE
                WHEN vs.state = 'success' THEN ROUND(o.total_minor::numeric / NULLIF(vc.n, 0))::bigint
                ELSE 0::bigint
            END
        ),
        0
    )::bigint AS allocated_revenue_minor
FROM
    vend_sessions vs
    INNER JOIN orders o ON o.id = vs.order_id
    INNER JOIN machines m ON m.id = o.machine_id
    INNER JOIN (
        SELECT
            order_id,
            COUNT(*)::bigint AS n
        FROM
            vend_sessions
        GROUP BY
            order_id
    ) vc ON vc.order_id = o.id
    LEFT JOIN products pr ON pr.organization_id = o.organization_id
        AND pr.id = vs.product_id
WHERE
    o.organization_id = $1
    AND o.created_at >= $2::timestamptz
    AND o.created_at < $3::timestamptz
    AND (
        $4::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.site_id = $4::uuid)
    AND (
        $5::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR o.machine_id = $5::uuid)
    AND (
        $6::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR vs.product_id = $6::uuid)
GROUP BY
    vs.product_id
ORDER BY
    allocated_revenue_minor DESC
LIMIT 500;

-- name: ReportingVendSummaryTotals :one
SELECT
    COUNT(*) FILTER (
        WHERE
            vs.state = 'success'
    )::bigint AS success_count,
    COUNT(*) FILTER (
        WHERE
            vs.state = 'failed'
    )::bigint AS failed_count,
    COUNT(*) FILTER (
        WHERE
            vs.state IN ('pending', 'in_progress')
    )::bigint AS in_progress_count
FROM
    vend_sessions vs
    INNER JOIN orders o ON o.id = vs.order_id
    INNER JOIN machines m ON m.id = o.machine_id
WHERE
    o.organization_id = $1
    AND COALESCE(vs.completed_at, vs.created_at) >= $2::timestamptz
    AND COALESCE(vs.completed_at, vs.created_at) < $3::timestamptz
    AND (
        $4::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.site_id = $4::uuid)
    AND (
        $5::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR o.machine_id = $5::uuid)
    AND (
        $6::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR vs.product_id = $6::uuid);

-- name: ReportingStockMovementCount :one
SELECT
    count(*)::bigint AS cnt
FROM
    inventory_events ie
    INNER JOIN machines m ON m.id = ie.machine_id
WHERE
    ie.organization_id = $1
    AND ie.occurred_at >= $2::timestamptz
    AND ie.occurred_at < $3::timestamptz
    AND (
        $4::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.site_id = $4::uuid)
    AND (
        $5::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR ie.machine_id = $5::uuid)
    AND (
        $6::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR ie.product_id = $6::uuid);

-- name: ReportingStockMovement :many
SELECT
    ie.id AS inventory_event_id,
    ie.machine_id,
    m.site_id,
    ie.product_id,
    COALESCE(pr.sku, '')::text AS product_sku,
    COALESCE(pr.name, '')::text AS product_name,
    ie.event_type,
    ie.slot_code,
    ie.quantity_delta,
    ie.quantity_before,
    ie.quantity_after,
    ie.occurred_at
FROM
    inventory_events ie
    INNER JOIN machines m ON m.id = ie.machine_id
    LEFT JOIN products pr ON pr.organization_id = ie.organization_id
        AND pr.id = ie.product_id
WHERE
    ie.organization_id = $1
    AND ie.occurred_at >= $2::timestamptz
    AND ie.occurred_at < $3::timestamptz
    AND (
        $4::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.site_id = $4::uuid)
    AND (
        $5::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR ie.machine_id = $5::uuid)
    AND (
        $6::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR ie.product_id = $6::uuid)
ORDER BY
    ie.occurred_at DESC
LIMIT $7 OFFSET $8;

-- name: ReportingCommandFailuresCount :one
SELECT
    count(*)::bigint AS cnt
FROM
    machine_command_attempts a
    INNER JOIN machines m ON m.id = a.machine_id
WHERE
    m.organization_id = $1
    AND a.sent_at >= $2::timestamptz
    AND a.sent_at < $3::timestamptz
    AND a.status IN ('failed', 'ack_timeout', 'expired', 'nack')
    AND (
        $4::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.site_id = $4::uuid)
    AND (
        $5::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.id = $5::uuid);

-- name: ReportingCommandFailures :many
SELECT
    a.id AS attempt_id,
    a.command_id,
    a.machine_id,
    m.site_id,
    a.attempt_no,
    a.sent_at,
    a.status,
    COALESCE(a.timeout_reason, '')::text AS timeout_reason
FROM
    machine_command_attempts a
    INNER JOIN machines m ON m.id = a.machine_id
WHERE
    m.organization_id = $1
    AND a.sent_at >= $2::timestamptz
    AND a.sent_at < $3::timestamptz
    AND a.status IN ('failed', 'ack_timeout', 'expired', 'nack')
    AND (
        $4::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.site_id = $4::uuid)
    AND (
        $5::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.id = $5::uuid)
ORDER BY
    a.sent_at DESC
LIMIT $6 OFFSET $7;

-- name: ReportingReconciliationSummary :one
SELECT
    COUNT(*) FILTER (
        WHERE
            status IN ('open', 'reviewing', 'escalated')
    )::bigint AS open_count,
    COUNT(*) FILTER (
        WHERE
            status IN ('resolved', 'dismissed', 'ignored')
    )::bigint AS closed_count
FROM
    commerce_reconciliation_cases c
WHERE
    c.organization_id = $1
    AND c.first_detected_at >= $2::timestamptz
    AND c.first_detected_at < $3::timestamptz;

-- name: ReportingReconciliationCasesCount :one
SELECT
    count(*)::bigint AS cnt
FROM
    commerce_reconciliation_cases c
WHERE
    c.organization_id = $1
    AND c.first_detected_at >= $2::timestamptz
    AND c.first_detected_at < $3::timestamptz
    AND (
        $4::text = 'all'
        OR (
            $4::text = 'open'
            AND c.status IN ('open', 'reviewing', 'escalated'))
        OR (
            $4::text = 'closed'
            AND c.status IN ('resolved', 'dismissed', 'ignored')));

-- name: ReportingReconciliationCases :many
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
    c.last_detected_at,
    c.resolved_at
FROM
    commerce_reconciliation_cases c
WHERE
    c.organization_id = $1
    AND c.first_detected_at >= $2::timestamptz
    AND c.first_detected_at < $3::timestamptz
    AND (
        $4::text = 'all'
        OR (
            $4::text = 'open'
            AND c.status IN ('open', 'reviewing', 'escalated'))
        OR (
            $4::text = 'closed'
            AND c.status IN ('resolved', 'dismissed', 'ignored')))
ORDER BY
    c.last_detected_at DESC
LIMIT $5 OFFSET $6;

-- Filtered sales/payments (site, machine, product) for admin reports. Sentinel nil UUID = unset.

-- name: ReportingSalesTotalsFiltered :one
SELECT
    COALESCE(SUM(o.total_minor), 0)::bigint AS gross_total_minor,
    COALESCE(SUM(o.tax_minor), 0)::bigint AS tax_minor,
    COALESCE(SUM(o.subtotal_minor), 0)::bigint AS subtotal_minor,
    COUNT(*)::bigint AS order_count
FROM
    orders o
    INNER JOIN machines m ON m.id = o.machine_id
WHERE
    o.organization_id = $1
    AND o.created_at >= $2::timestamptz
    AND o.created_at < $3::timestamptz
    AND (
        $4::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.site_id = $4::uuid)
    AND (
        $5::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR o.machine_id = $5::uuid)
    AND (
        $6::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR EXISTS (
            SELECT
                1
            FROM
                vend_sessions vs
            WHERE
                vs.order_id = o.id
                AND vs.product_id = $6::uuid));

-- name: ReportingSalesByDayFiltered :many
SELECT
    (date_trunc('day', o.created_at AT TIME ZONE $7::text) AT TIME ZONE $7::text)::timestamptz AS bucket_start,
    COUNT(*)::bigint AS order_count,
    COALESCE(SUM(o.total_minor), 0)::bigint AS total_minor,
    COALESCE(SUM(o.subtotal_minor), 0)::bigint AS subtotal_minor,
    COALESCE(SUM(o.tax_minor), 0)::bigint AS tax_minor
FROM
    orders o
    INNER JOIN machines m ON m.id = o.machine_id
WHERE
    o.organization_id = $1
    AND o.created_at >= $2::timestamptz
    AND o.created_at < $3::timestamptz
    AND (
        $4::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.site_id = $4::uuid)
    AND (
        $5::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR o.machine_id = $5::uuid)
    AND (
        $6::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR EXISTS (
            SELECT
                1
            FROM
                vend_sessions vs
            WHERE
                vs.order_id = o.id
                AND vs.product_id = $6::uuid))
GROUP BY
    1
ORDER BY
    1 ASC;

-- name: ReportingSalesBySiteFiltered :many
SELECT
    m.site_id,
    COUNT(*)::bigint AS order_count,
    COALESCE(SUM(o.total_minor), 0)::bigint AS total_minor,
    COALESCE(SUM(o.subtotal_minor), 0)::bigint AS subtotal_minor,
    COALESCE(SUM(o.tax_minor), 0)::bigint AS tax_minor
FROM
    orders o
    INNER JOIN machines m ON m.id = o.machine_id
WHERE
    o.organization_id = $1
    AND o.created_at >= $2::timestamptz
    AND o.created_at < $3::timestamptz
    AND (
        $4::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.site_id = $4::uuid)
    AND (
        $5::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR o.machine_id = $5::uuid)
    AND (
        $6::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR EXISTS (
            SELECT
                1
            FROM
                vend_sessions vs
            WHERE
                vs.order_id = o.id
                AND vs.product_id = $6::uuid))
GROUP BY
    m.site_id
ORDER BY
    total_minor DESC;

-- name: ReportingSalesByMachineFiltered :many
SELECT
    o.machine_id,
    COUNT(*)::bigint AS order_count,
    COALESCE(SUM(o.total_minor), 0)::bigint AS total_minor,
    COALESCE(SUM(o.subtotal_minor), 0)::bigint AS subtotal_minor,
    COALESCE(SUM(o.tax_minor), 0)::bigint AS tax_minor
FROM
    orders o
    INNER JOIN machines m ON m.id = o.machine_id
WHERE
    o.organization_id = $1
    AND o.created_at >= $2::timestamptz
    AND o.created_at < $3::timestamptz
    AND (
        $4::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.site_id = $4::uuid)
    AND (
        $5::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR o.machine_id = $5::uuid)
    AND (
        $6::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR EXISTS (
            SELECT
                1
            FROM
                vend_sessions vs
            WHERE
                vs.order_id = o.id
                AND vs.product_id = $6::uuid))
GROUP BY
    o.machine_id
ORDER BY
    total_minor DESC
LIMIT 500;

-- name: ReportingSalesByPaymentProviderFiltered :many
SELECT
    p.provider,
    COUNT(DISTINCT o.id)::bigint AS order_count,
    COALESCE(SUM(o.total_minor), 0)::bigint AS total_minor,
    COALESCE(SUM(o.subtotal_minor), 0)::bigint AS subtotal_minor,
    COALESCE(SUM(o.tax_minor), 0)::bigint AS tax_minor
FROM
    orders o
    INNER JOIN machines m ON m.id = o.machine_id
    INNER JOIN payments p ON p.order_id = o.id
        AND p.state IN ('authorized', 'captured')
WHERE
    o.organization_id = $1
    AND o.created_at >= $2::timestamptz
    AND o.created_at < $3::timestamptz
    AND (
        $4::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.site_id = $4::uuid)
    AND (
        $5::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR o.machine_id = $5::uuid)
    AND (
        $6::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR EXISTS (
            SELECT
                1
            FROM
                vend_sessions vs
            WHERE
                vs.order_id = o.id
                AND vs.product_id = $6::uuid))
GROUP BY
    p.provider
ORDER BY
    total_minor DESC;

-- name: ReportingPaymentsTotalsFiltered :one
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
FROM
    payments p
    INNER JOIN orders o ON o.id = p.order_id
    INNER JOIN machines m ON m.id = o.machine_id
WHERE
    o.organization_id = $1
    AND p.created_at >= $2::timestamptz
    AND p.created_at < $3::timestamptz
    AND (
        $4::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.site_id = $4::uuid)
    AND (
        $5::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR o.machine_id = $5::uuid)
    AND (
        $6::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR EXISTS (
            SELECT
                1
            FROM
                vend_sessions vs
            WHERE
                vs.order_id = o.id
                AND vs.product_id = $6::uuid));

-- name: ReportingPaymentsByDayFiltered :many
SELECT
    (date_trunc('day', p.created_at AT TIME ZONE $7::text) AT TIME ZONE $7::text)::timestamptz AS bucket_start,
    COUNT(*)::bigint AS payment_count,
    COALESCE(SUM(p.amount_minor), 0)::bigint AS amount_minor
FROM
    payments p
    INNER JOIN orders o ON o.id = p.order_id
    INNER JOIN machines m ON m.id = o.machine_id
WHERE
    o.organization_id = $1
    AND p.created_at >= $2::timestamptz
    AND p.created_at < $3::timestamptz
    AND (
        $4::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.site_id = $4::uuid)
    AND (
        $5::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR o.machine_id = $5::uuid)
    AND (
        $6::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR EXISTS (
            SELECT
                1
            FROM
                vend_sessions vs
            WHERE
                vs.order_id = o.id
                AND vs.product_id = $6::uuid))
GROUP BY
    1
ORDER BY
    1 ASC;

-- name: ReportingPaymentsByMethodAndStateFiltered :many
SELECT
    p.provider,
    p.state,
    COUNT(*)::bigint AS payment_count,
    COALESCE(SUM(p.amount_minor), 0)::bigint AS amount_minor
FROM
    payments p
    INNER JOIN orders o ON o.id = p.order_id
    INNER JOIN machines m ON m.id = o.machine_id
WHERE
    o.organization_id = $1
    AND p.created_at >= $2::timestamptz
    AND p.created_at < $3::timestamptz
    AND (
        $4::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.site_id = $4::uuid)
    AND (
        $5::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR o.machine_id = $5::uuid)
    AND (
        $6::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR EXISTS (
            SELECT
                1
            FROM
                vend_sessions vs
            WHERE
                vs.order_id = o.id
                AND vs.product_id = $6::uuid))
GROUP BY
    p.provider,
    p.state
ORDER BY
    amount_minor DESC;

-- name: ReportingPaymentSettlementFiltered :many
SELECT
    (date_trunc('day', p.created_at AT TIME ZONE $7::text) AT TIME ZONE $7::text)::timestamptz AS bucket_start,
    p.provider,
    p.state,
    p.settlement_status,
    p.reconciliation_status,
    COUNT(*)::bigint AS payment_count,
    COALESCE(SUM(p.amount_minor), 0)::bigint AS amount_minor
FROM
    payments p
    INNER JOIN orders o ON o.id = p.order_id
    INNER JOIN machines m ON m.id = o.machine_id
WHERE
    o.organization_id = $1
    AND p.created_at >= $2::timestamptz
    AND p.created_at < $3::timestamptz
    AND (
        $4::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.site_id = $4::uuid)
    AND (
        $5::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR o.machine_id = $5::uuid)
    AND (
        $6::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR EXISTS (
            SELECT
                1
            FROM
                vend_sessions vs
            WHERE
                vs.order_id = o.id
                AND vs.product_id = $6::uuid))
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

-- name: ReportingMachineHealthFilteredCount :one
SELECT
    COUNT(*)::bigint AS cnt
FROM
    machines m
WHERE
    m.organization_id = $1
    AND (
        $2::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.site_id = $2::uuid)
    AND (
        $3::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.id = $3::uuid)
    AND (
        $4::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR EXISTS (
            SELECT
                1
            FROM
                vend_sessions vs
                INNER JOIN orders o ON o.id = vs.order_id
            WHERE
                o.machine_id = m.id
                AND o.organization_id = m.organization_id
                AND vs.product_id = $4::uuid
                AND COALESCE(vs.completed_at, vs.created_at) >= $5::timestamptz
                AND COALESCE(vs.completed_at, vs.created_at) < $6::timestamptz));

-- name: ReportingMachineHealthFiltered :many
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
        WHEN m.last_seen_at < $7::timestamptz THEN TRUE
        ELSE FALSE
    END AS offline
FROM
    machines m
    INNER JOIN sites s ON s.id = m.site_id
WHERE
    m.organization_id = $1
    AND (
        $2::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.site_id = $2::uuid)
    AND (
        $3::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.id = $3::uuid)
    AND (
        $4::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR EXISTS (
            SELECT
                1
            FROM
                vend_sessions vs
                INNER JOIN orders o ON o.id = vs.order_id
            WHERE
                o.machine_id = m.id
                AND o.organization_id = m.organization_id
                AND vs.product_id = $4::uuid
                AND COALESCE(vs.completed_at, vs.created_at) >= $5::timestamptz
                AND COALESCE(vs.completed_at, vs.created_at) < $6::timestamptz))
ORDER BY
    offline DESC,
    m.last_seen_at ASC NULLS FIRST,
    m.name ASC
LIMIT $8 OFFSET $9;

-- name: ReportingFailedVendsCountFiltered :one
SELECT
    COUNT(*)::bigint AS cnt
FROM
    vend_sessions vs
    INNER JOIN orders o ON o.id = vs.order_id
    INNER JOIN machines m ON m.id = vs.machine_id
WHERE
    o.organization_id = $1
    AND vs.state = 'failed'
    AND COALESCE(vs.completed_at, vs.created_at) >= $2::timestamptz
    AND COALESCE(vs.completed_at, vs.created_at) < $3::timestamptz
    AND (
        $4::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.site_id = $4::uuid)
    AND (
        $5::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR o.machine_id = $5::uuid)
    AND (
        $6::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR vs.product_id = $6::uuid);

-- name: ReportingFailedVendsFiltered :many
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
FROM
    vend_sessions vs
    INNER JOIN orders o ON o.id = vs.order_id
    INNER JOIN machines m ON m.id = vs.machine_id
WHERE
    o.organization_id = $1
    AND vs.state = 'failed'
    AND COALESCE(vs.completed_at, vs.created_at) >= $2::timestamptz
    AND COALESCE(vs.completed_at, vs.created_at) < $3::timestamptz
    AND (
        $4::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.site_id = $4::uuid)
    AND (
        $5::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR o.machine_id = $5::uuid)
    AND (
        $6::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR vs.product_id = $6::uuid)
ORDER BY
    COALESCE(vs.completed_at, vs.created_at) DESC
LIMIT $7 OFFSET $8;

-- name: ReportingInventoryExceptionsFilteredCount :one
SELECT
    COUNT(*)::bigint AS cnt
FROM
    machine_slot_state mss
    INNER JOIN machines m ON m.id = mss.machine_id
    INNER JOIN planograms pg ON pg.id = mss.planogram_id
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
    )
    AND (
        $4::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.site_id = $4::uuid)
    AND (
        $5::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.id = $5::uuid)
    AND (
        $6::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR s.product_id = $6::uuid);

-- name: ReportingInventoryExceptionsFiltered :many
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
FROM
    machine_slot_state mss
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
    AND (
        $4::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.site_id = $4::uuid)
    AND (
        $5::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.id = $5::uuid)
    AND (
        $6::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR s.product_id = $6::uuid)
ORDER BY
    m.name ASC,
    mss.slot_index ASC
LIMIT $7 OFFSET $8;

-- Technician / refill / operator-attributed inventory operations (excludes sale ledger rows).
-- name: ReportingTechnicianFillOpsCount :one
SELECT
    count(*)::bigint AS cnt
FROM
    inventory_events ie
    INNER JOIN machines m ON m.id = ie.machine_id
WHERE
    ie.organization_id = $1
    AND ie.occurred_at >= $2::timestamptz
    AND ie.occurred_at < $3::timestamptz
    AND ie.event_type <> 'sale'
    AND (
        ie.technician_id IS NOT NULL
        OR ie.refill_session_id IS NOT NULL
        OR ie.operator_session_id IS NOT NULL
        OR ie.event_type IN (
            'restock',
            'transfer_in',
            'count',
            'reconcile',
            'correction',
            'adjustment'))
    AND (
        $4::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.site_id = $4::uuid)
    AND (
        $5::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR ie.machine_id = $5::uuid)
    AND (
        $6::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR ie.product_id = $6::uuid);

-- name: ReportingTechnicianFillOps :many
SELECT
    ie.id AS inventory_event_id,
    ie.machine_id,
    m.site_id,
    ie.product_id,
    COALESCE(pr.sku, '')::text AS product_sku,
    COALESCE(pr.name, '')::text AS product_name,
    ie.event_type,
    ie.slot_code,
    ie.quantity_delta,
    ie.quantity_before,
    ie.quantity_after,
    ie.operator_session_id,
    ie.technician_id,
    COALESCE(t.display_name, '')::text AS technician_display_name,
    ie.refill_session_id,
    ie.occurred_at
FROM
    inventory_events ie
    INNER JOIN machines m ON m.id = ie.machine_id
    LEFT JOIN products pr ON pr.organization_id = ie.organization_id
        AND pr.id = ie.product_id
    LEFT JOIN technicians t ON t.id = ie.technician_id
        AND t.organization_id = ie.organization_id
WHERE
    ie.organization_id = $1
    AND ie.occurred_at >= $2::timestamptz
    AND ie.occurred_at < $3::timestamptz
    AND ie.event_type <> 'sale'
    AND (
        ie.technician_id IS NOT NULL
        OR ie.refill_session_id IS NOT NULL
        OR ie.operator_session_id IS NOT NULL
        OR ie.event_type IN (
            'restock',
            'transfer_in',
            'count',
            'reconcile',
            'correction',
            'adjustment'))
    AND (
        $4::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.site_id = $4::uuid)
    AND (
        $5::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR ie.machine_id = $5::uuid)
    AND (
        $6::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR ie.product_id = $6::uuid)
ORDER BY
    ie.occurred_at DESC
LIMIT $7 OFFSET $8;
