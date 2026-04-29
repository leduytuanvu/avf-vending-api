-- name: InventoryAdminGetMachineOrg :one
SELECT id, organization_id, name, status
FROM machines
WHERE id = $1;

-- name: InventoryAdminGetOrgDefaultCurrency :one
SELECT coalesce(
    (
        SELECT trim(both FROM pb.currency::text)
        FROM price_books pb
        WHERE pb.organization_id = $1
            AND pb.is_default = true
            AND pb.scope_type = 'organization'
            AND pb.site_id IS NULL
            AND pb.machine_id IS NULL
            AND (pb.effective_to IS NULL OR pb.effective_to > now())
        ORDER BY pb.priority DESC, pb.effective_from DESC
        LIMIT 1
    ),
    'USD'::text
)::text AS currency;

-- name: InventoryAdminListMachineSlots :many
SELECT
    mss.machine_id,
    mss.planogram_id,
    pg.name AS planogram_name,
    mss.slot_index,
    mss.current_quantity,
    COALESCE(s.max_quantity, 0)::int AS max_quantity,
    mss.price_minor,
    mss.planogram_revision_applied,
    mss.updated_at,
    s.product_id,
    pr.sku AS product_sku,
    pr.name AS product_name,
    (mss.current_quantity <= 0) AS is_empty,
    (
        COALESCE(s.max_quantity, 0) > 0
        AND mss.current_quantity::float / NULLIF(s.max_quantity, 0)::float < 0.15
    ) AS low_stock,
    coalesce(
        (
            SELECT
                w.cabinet_code
            FROM (
                SELECT
                    cabinet_code,
                    cabinet_index,
                    sort_order,
                    slot_capacity,
                    sum(coalesce(slot_capacity, 0)) OVER (
                        ORDER BY cabinet_index, sort_order ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING
                    ) AS cum_before
                FROM machine_cabinets
                WHERE
                    machine_id = mss.machine_id
            ) w
            WHERE
                EXISTS (
                    SELECT 1
                    FROM machine_cabinets h
                    WHERE
                        h.machine_id = mss.machine_id
                        AND h.slot_capacity IS NOT NULL
                )
                AND mss.slot_index >= coalesce(w.cum_before, 0)
                AND mss.slot_index < coalesce(w.cum_before, 0) + coalesce(w.slot_capacity, 2147483647)
            ORDER BY
                w.cabinet_index,
                w.sort_order
            LIMIT 1
        ),
        (
            SELECT
                mc2.cabinet_code
            FROM machine_cabinets mc2
            WHERE
                mc2.machine_id = mss.machine_id
            ORDER BY
                mc2.cabinet_index,
                mc2.sort_order
            LIMIT 1
        ),
        'CAB-A'::text
    )::text AS cabinet_code,
    coalesce(
        (
            SELECT
                w.cabinet_index
            FROM (
                SELECT
                    cabinet_code,
                    cabinet_index,
                    sort_order,
                    slot_capacity,
                    sum(coalesce(slot_capacity, 0)) OVER (
                        ORDER BY cabinet_index, sort_order ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING
                    ) AS cum_before
                FROM machine_cabinets
                WHERE
                    machine_id = mss.machine_id
            ) w
            WHERE
                EXISTS (
                    SELECT 1
                    FROM machine_cabinets h
                    WHERE
                        h.machine_id = mss.machine_id
                        AND h.slot_capacity IS NOT NULL
                )
                AND mss.slot_index >= coalesce(w.cum_before, 0)
                AND mss.slot_index < coalesce(w.cum_before, 0) + coalesce(w.slot_capacity, 2147483647)
            ORDER BY
                w.cabinet_index,
                w.sort_order
            LIMIT 1
        ),
        (
            SELECT
                mc2.cabinet_index
            FROM machine_cabinets mc2
            WHERE
                mc2.machine_id = mss.machine_id
            ORDER BY
                mc2.cabinet_index,
                mc2.sort_order
            LIMIT 1
        ),
        0
    )::int AS cabinet_index
FROM machine_slot_state mss
JOIN planograms pg ON pg.id = mss.planogram_id
LEFT JOIN slots s ON s.planogram_id = mss.planogram_id
    AND s.slot_index = mss.slot_index
LEFT JOIN products pr ON pr.id = s.product_id
WHERE mss.machine_id = $1
ORDER BY mss.planogram_id, mss.slot_index;

-- name: InventoryAdminAggregateMachineInventory :many
SELECT
    pr.id AS product_id,
    pr.name AS product_name,
    pr.sku AS product_sku,
    sum(mss.current_quantity)::bigint AS total_quantity,
    count(*)::bigint AS slot_count,
    max(COALESCE(s.max_quantity, 0))::int AS max_capacity_any_slot,
    bool_or(
        COALESCE(s.max_quantity, 0) > 0
        AND mss.current_quantity::float / NULLIF(s.max_quantity, 0)::float < 0.15
    ) AS low_stock,
    CASE
        WHEN count(DISTINCT cab.cabinet_code) = 1
        AND count(DISTINCT cab.cabinet_index) = 1 THEN max(cab.cabinet_code::text)
        ELSE ''
    END::text AS cabinet_code,
    CASE
        WHEN count(DISTINCT cab.cabinet_code) = 1
        AND count(DISTINCT cab.cabinet_index) = 1 THEN max(cab.cabinet_index)::int
        ELSE 0
    END::int AS cabinet_index
FROM machine_slot_state mss
JOIN slots s ON s.planogram_id = mss.planogram_id
    AND s.slot_index = mss.slot_index
JOIN products pr ON pr.id = s.product_id
LEFT JOIN LATERAL (
    SELECT
        coalesce(
            (
                SELECT
                    w.cabinet_code
                FROM (
                    SELECT
                        cabinet_code,
                        cabinet_index,
                        sort_order,
                        slot_capacity,
                        sum(coalesce(slot_capacity, 0)) OVER (
                            ORDER BY cabinet_index, sort_order ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING
                        ) AS cum_before
                    FROM machine_cabinets
                    WHERE
                        machine_id = mss.machine_id
                ) w
                WHERE
                    EXISTS (
                        SELECT 1
                        FROM machine_cabinets h
                        WHERE
                            h.machine_id = mss.machine_id
                            AND h.slot_capacity IS NOT NULL
                    )
                    AND mss.slot_index >= coalesce(w.cum_before, 0)
                    AND mss.slot_index < coalesce(w.cum_before, 0) + coalesce(w.slot_capacity, 2147483647)
                ORDER BY
                    w.cabinet_index,
                    w.sort_order
                LIMIT 1
            ),
            (
                SELECT
                    mc2.cabinet_code
                FROM machine_cabinets mc2
                WHERE
                    mc2.machine_id = mss.machine_id
                ORDER BY
                    mc2.cabinet_index,
                    mc2.sort_order
                LIMIT 1
            ),
            'CAB-A'::text
        ) AS cabinet_code,
        coalesce(
            (
                SELECT
                    w.cabinet_index
                FROM (
                    SELECT
                        cabinet_code,
                        cabinet_index,
                        sort_order,
                        slot_capacity,
                        sum(coalesce(slot_capacity, 0)) OVER (
                            ORDER BY cabinet_index, sort_order ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING
                        ) AS cum_before
                    FROM machine_cabinets
                    WHERE
                        machine_id = mss.machine_id
                ) w
                WHERE
                    EXISTS (
                        SELECT 1
                        FROM machine_cabinets h
                        WHERE
                            h.machine_id = mss.machine_id
                            AND h.slot_capacity IS NOT NULL
                    )
                    AND mss.slot_index >= coalesce(w.cum_before, 0)
                    AND mss.slot_index < coalesce(w.cum_before, 0) + coalesce(w.slot_capacity, 2147483647)
                ORDER BY
                    w.cabinet_index,
                    w.sort_order
                LIMIT 1
            ),
            (
                SELECT
                    mc2.cabinet_index
                FROM machine_cabinets mc2
                WHERE
                    mc2.machine_id = mss.machine_id
                ORDER BY
                    mc2.cabinet_index,
                    mc2.sort_order
                LIMIT 1
            ),
            0
        )::int AS cabinet_index
) cab ON TRUE
WHERE mss.machine_id = $1
GROUP BY pr.id, pr.name, pr.sku
ORDER BY pr.name;

-- name: InventoryAdminListCurrentMachineSlotConfigsByMachine :many
SELECT
    msc.id,
    msc.organization_id,
    msc.machine_id,
    msc.machine_cabinet_id,
    mc.cabinet_code,
    mc.cabinet_index,
    msc.machine_slot_layout_id,
    msc.slot_code,
    msc.slot_index,
    msc.product_id,
    pr.sku AS product_sku,
    pr.name AS product_name,
    msc.max_quantity,
    msc.price_minor,
    msc.effective_from,
    msc.effective_to,
    msc.is_current,
    msc.metadata,
    msc.created_at,
    msc.updated_at
FROM
    machine_slot_configs msc
    INNER JOIN machine_cabinets mc ON mc.id = msc.machine_cabinet_id
    LEFT JOIN products pr ON pr.id = msc.product_id
    AND pr.organization_id = msc.organization_id
WHERE
    msc.machine_id = $1
    AND msc.is_current
ORDER BY
    mc.sort_order ASC,
    mc.cabinet_code ASC,
    msc.slot_code ASC;

-- name: InventoryAdminUpsertMachineSlotState :one
INSERT INTO machine_slot_state (
    machine_id,
    planogram_id,
    slot_index,
    current_quantity,
    price_minor,
    planogram_revision_applied
)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (machine_id, planogram_id, slot_index) DO UPDATE
SET
    current_quantity = EXCLUDED.current_quantity,
    price_minor = EXCLUDED.price_minor,
    planogram_revision_applied = EXCLUDED.planogram_revision_applied,
    updated_at = now()
RETURNING
    id,
    machine_id,
    planogram_id,
    slot_index,
    current_quantity,
    price_minor,
    planogram_revision_applied,
    updated_at;

-- name: InventoryAdminInsertInventoryEventsBatch :many
INSERT INTO inventory_events (
    organization_id,
    machine_id,
    machine_cabinet_id,
    cabinet_code,
    slot_code,
    product_id,
    event_type,
    reason_code,
    quantity_before,
    quantity_delta,
    quantity_after,
    unit_price_minor,
    currency,
    correlation_id,
    operator_session_id,
    technician_id,
    refill_session_id,
    inventory_count_session_id,
    occurred_at,
    recorded_at,
    metadata
)
SELECT
    (e->>'organization_id')::uuid AS organization_id,
    (e->>'machine_id')::uuid AS machine_id,
    NULLIF (e->>'machine_cabinet_id', '')::uuid AS machine_cabinet_id,
    NULLIF (btrim(e->>'cabinet_code'), '') AS cabinet_code,
    NULLIF (e->>'slot_code', '') AS slot_code,
    NULLIF (e->>'product_id', '')::uuid AS product_id,
    e->>'event_type' AS event_type,
    NULLIF (btrim(e->>'reason_code'), '') AS reason_code,
    NULLIF (e->>'quantity_before', '')::int AS quantity_before,
    (e->>'quantity_delta')::int AS quantity_delta,
    NULLIF (e->>'quantity_after', '')::int AS quantity_after,
    coalesce((e->>'unit_price_minor')::bigint, 0) AS unit_price_minor,
    coalesce(
        NULLIF (btrim(e->>'currency'), ''),
        'USD'::text
    ) AS currency,
    NULLIF (e->>'correlation_id', '')::uuid AS correlation_id,
    NULLIF (e->>'operator_session_id', '')::uuid AS operator_session_id,
    NULLIF (e->>'technician_id', '')::uuid AS technician_id,
    NULLIF (e->>'refill_session_id', '')::uuid AS refill_session_id,
    NULLIF (e->>'inventory_count_session_id', '')::uuid AS inventory_count_session_id,
    coalesce((e->>'occurred_at')::timestamptz, now()) AS occurred_at,
    coalesce((e->>'recorded_at')::timestamptz, now()) AS recorded_at,
    coalesce(e->'metadata', '{}'::jsonb) AS metadata
FROM
    jsonb_array_elements(sqlc.arg(events_json)::jsonb) AS e
RETURNING
    id;

-- name: InventoryAdminListInventoryEventsByMachine :many
SELECT
    ie.id,
    ie.organization_id,
    ie.machine_id,
    ie.machine_cabinet_id,
    ie.cabinet_code,
    ie.slot_code,
    ie.product_id,
    ie.event_type,
    ie.reason_code,
    ie.quantity_before,
    ie.quantity_delta,
    ie.quantity_after,
    ie.unit_price_minor,
    ie.currency,
    ie.correlation_id,
    ie.operator_session_id,
    ie.technician_id,
    t.display_name AS technician_display_name,
    ie.refill_session_id,
    ie.inventory_count_session_id,
    ie.occurred_at,
    ie.recorded_at,
    ie.metadata
FROM
    inventory_events ie
    LEFT JOIN technicians t ON t.id = ie.technician_id
    AND t.organization_id = ie.organization_id
WHERE
    ie.machine_id = $1
    AND ($2::boolean IS FALSE OR ie.occurred_at >= $3::timestamptz)
    AND ($4::boolean IS FALSE OR ie.occurred_at <= $5::timestamptz)
ORDER BY
    ie.occurred_at DESC,
    ie.id DESC
LIMIT $6 OFFSET $7;

-- name: InventoryAdminCountInventoryEventsByIdempotencyKey :one
SELECT
    count(*)::bigint
FROM
    inventory_events
WHERE
    machine_id = $1
    AND (metadata ->> 'idempotency_key') = $2::text;

-- name: InventoryAdminGetInventoryIdempotencyPayloadHash :one
SELECT
    coalesce(metadata ->> 'idempotency_payload_sha256', '')::text AS payload_hash
FROM
    inventory_events
WHERE
    machine_id = $1
    AND (metadata ->> 'idempotency_key') = $2::text
ORDER BY
    id ASC
LIMIT 1;

-- Slot aggregates aligned with InventoryAdminListMachineSlots (machine_slot_state + slots).
-- name: InventoryAdminSummarizeSlotsForMachine :one
SELECT
    coalesce(count(*), 0)::bigint AS total_slots,
    coalesce(count(*) FILTER (WHERE mss.current_quantity > 0), 0)::bigint AS occupied_slots,
    coalesce(count(*) FILTER (WHERE
        COALESCE(s.max_quantity, 0) > 0
        AND mss.current_quantity::float / NULLIF(s.max_quantity, 0)::float < 0.15
    ), 0)::bigint AS low_stock_slots,
    coalesce(count(*) FILTER (WHERE mss.current_quantity <= 0), 0)::bigint AS out_of_stock_slots
FROM machine_slot_state mss
LEFT JOIN slots s ON s.planogram_id = mss.planogram_id
    AND s.slot_index = mss.slot_index
WHERE
    mss.machine_id = $1;

-- name: InventoryAdminSummarizeSlotsForMachines :many
SELECT
    mss.machine_id,
    coalesce(count(*), 0)::bigint AS total_slots,
    coalesce(count(*) FILTER (WHERE mss.current_quantity > 0), 0)::bigint AS occupied_slots,
    coalesce(count(*) FILTER (WHERE
        COALESCE(s.max_quantity, 0) > 0
        AND mss.current_quantity::float / NULLIF(s.max_quantity, 0)::float < 0.15
    ), 0)::bigint AS low_stock_slots,
    coalesce(count(*) FILTER (WHERE mss.current_quantity <= 0), 0)::bigint AS out_of_stock_slots
FROM machine_slot_state mss
LEFT JOIN slots s ON s.planogram_id = mss.planogram_id
    AND s.slot_index = mss.slot_index
WHERE
    mss.machine_id = ANY($1::uuid[])
GROUP BY
    mss.machine_id;

-- Refill forecasting: slot inventory joined to vend velocity (successful dispenses) in a lookback window.
-- Optional filters use uuid nil sentinel '00000000-0000-0000-0000-000000000000'. low_stock_only restricts to empty or <15% fill.
-- name: InventoryAdminRefillForecastSlots :many
SELECT
    m.id AS machine_id,
    m.name AS machine_name,
    m.site_id,
    st.name AS site_name,
    mss.planogram_id,
    pg.name AS planogram_name,
    mss.slot_index,
    s.product_id,
    pr.sku AS product_sku,
    pr.name AS product_name,
    mss.current_quantity,
    COALESCE(s.max_quantity, 0)::int AS max_quantity,
    COALESCE(vel.units_sold, 0)::bigint AS units_sold_window
FROM
    machine_slot_state mss
    INNER JOIN machines m ON m.id = mss.machine_id
    INNER JOIN sites st ON st.id = m.site_id
        AND st.organization_id = m.organization_id
    INNER JOIN planograms pg ON pg.id = mss.planogram_id
    LEFT JOIN slots s ON s.planogram_id = mss.planogram_id
        AND s.slot_index = mss.slot_index
    LEFT JOIN products pr ON pr.id = s.product_id
        AND pr.organization_id = m.organization_id
    LEFT JOIN (
        SELECT
            vs.machine_id,
            vs.product_id,
            COUNT(*)::bigint AS units_sold
        FROM
            vend_sessions vs
            INNER JOIN machines mm ON mm.id = vs.machine_id
        WHERE
            mm.organization_id = $1
            AND vs.state = 'success'
            AND COALESCE(vs.completed_at, vs.created_at) >= $2::timestamptz
            AND COALESCE(vs.completed_at, vs.created_at) < $3::timestamptz
        GROUP BY
            vs.machine_id,
            vs.product_id
    ) vel ON vel.machine_id = mss.machine_id
    AND vel.product_id = s.product_id
WHERE
    m.organization_id = $1
    AND s.product_id IS NOT NULL
    AND (
        $4::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.site_id = $4)
    AND (
        $5::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR m.id = $5)
    AND (
        $6::uuid = '00000000-0000-0000-0000-000000000000'::uuid
        OR s.product_id = $6)
    AND (
        $7::boolean IS FALSE
        OR mss.current_quantity <= 0
        OR (
            COALESCE(s.max_quantity, 0) > 0
            AND mss.current_quantity::float / NULLIF(s.max_quantity, 0)::float < 0.15))
ORDER BY
    m.name ASC,
    pr.name ASC,
    mss.slot_index ASC
LIMIT 10000;

