-- name: FinanceDailyCloseAggregate :one
WITH scoped_orders AS (
    SELECT
        o.*
    FROM
        orders o
        INNER JOIN machines m ON m.id = o.machine_id
            AND m.organization_id = o.organization_id
    WHERE
        o.organization_id = $1
        AND o.created_at >= $2::timestamptz
        AND o.created_at < $3::timestamptz
        AND (
            $4::uuid = '00000000-0000-0000-0000-000000000000'::uuid
            OR m.site_id = $4)
        AND (
            $5::uuid = '00000000-0000-0000-0000-000000000000'::uuid
            OR o.machine_id = $5)
),
scoped_payments AS (
    SELECT
        p.*
    FROM
        payments p
        INNER JOIN scoped_orders o ON o.id = p.order_id
),
scoped_refunds AS (
    SELECT
        r.*
    FROM
        refunds r
        INNER JOIN scoped_orders o ON o.id = r.order_id
    WHERE
        r.state = 'completed'
)
SELECT
    (SELECT
            COALESCE(SUM(total_minor), 0)::bigint
        FROM
            scoped_orders) AS gross_sales_minor,
    0::bigint AS discount_minor,
    (SELECT
            COALESCE(SUM(amount_minor), 0)::bigint
        FROM
            scoped_refunds) AS refund_minor,
    (SELECT
            COALESCE(SUM(p.amount_minor) FILTER (
                    WHERE
                        p.provider = 'cash'
                        AND p.state = 'captured'), 0)::bigint
        FROM
            scoped_payments p) AS cash_minor,
    (SELECT
            COALESCE(SUM(p.amount_minor) FILTER (
                    WHERE
                        p.provider <> 'cash'
                        AND p.state = 'captured'), 0)::bigint
        FROM
            scoped_payments p) AS qr_wallet_minor,
    (SELECT
            COALESCE(SUM(p.amount_minor) FILTER (
                    WHERE
                        p.state = 'failed'), 0)::bigint
        FROM
            scoped_payments p) AS failed_minor,
    (SELECT
            COALESCE(SUM(p.amount_minor) FILTER (
                    WHERE
                        p.state = 'authorized'), 0)::bigint
        FROM
            scoped_payments p) AS pending_minor;

-- name: InsertFinanceDailyClose :one
INSERT INTO finance_daily_closes (
    organization_id,
    close_date,
    timezone,
    site_id,
    machine_id,
    idempotency_key,
    gross_sales_minor,
    discount_minor,
    refund_minor,
    net_minor,
    cash_minor,
    qr_wallet_minor,
    failed_minor,
    pending_minor)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING
    id,
    organization_id,
    close_date,
    timezone,
    site_id,
    machine_id,
    idempotency_key,
    gross_sales_minor,
    discount_minor,
    refund_minor,
    net_minor,
    cash_minor,
    qr_wallet_minor,
    failed_minor,
    pending_minor,
    created_at;

-- name: GetFinanceDailyCloseByIDForOrg :one
SELECT
    id,
    organization_id,
    close_date,
    timezone,
    site_id,
    machine_id,
    idempotency_key,
    gross_sales_minor,
    discount_minor,
    refund_minor,
    net_minor,
    cash_minor,
    qr_wallet_minor,
    failed_minor,
    pending_minor,
    created_at
FROM
    finance_daily_closes
WHERE
    id = $1
    AND organization_id = $2;

-- name: GetFinanceDailyCloseByIdempotencyKey :one
SELECT
    id,
    organization_id,
    close_date,
    timezone,
    site_id,
    machine_id,
    idempotency_key,
    gross_sales_minor,
    discount_minor,
    refund_minor,
    net_minor,
    cash_minor,
    qr_wallet_minor,
    failed_minor,
    pending_minor,
    created_at
FROM
    finance_daily_closes
WHERE
    organization_id = $1
    AND idempotency_key = $2;

-- name: FinanceDailyCloseExistsForScope :one
SELECT
    EXISTS (
        SELECT
            1
        FROM
            finance_daily_closes
        WHERE
            organization_id = $1
            AND close_date = $2
            AND timezone = $3
            AND COALESCE(site_id, '00000000-0000-0000-0000-000000000000'::uuid) = COALESCE(NULLIF($4::uuid, '00000000-0000-0000-0000-000000000000'::uuid), '00000000-0000-0000-0000-000000000000'::uuid)
            AND COALESCE(machine_id, '00000000-0000-0000-0000-000000000000'::uuid) = COALESCE(NULLIF($5::uuid, '00000000-0000-0000-0000-000000000000'::uuid), '00000000-0000-0000-0000-000000000000'::uuid)) AS exists;

-- name: ListFinanceDailyClosesForOrg :many
SELECT
    id,
    organization_id,
    close_date,
    timezone,
    site_id,
    machine_id,
    idempotency_key,
    gross_sales_minor,
    discount_minor,
    refund_minor,
    net_minor,
    cash_minor,
    qr_wallet_minor,
    failed_minor,
    pending_minor,
    created_at
FROM
    finance_daily_closes
WHERE
    organization_id = $1
ORDER BY
    close_date DESC,
    created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountFinanceDailyClosesForOrg :one
SELECT
    count(*)::bigint AS cnt
FROM
    finance_daily_closes
WHERE
    organization_id = $1;
