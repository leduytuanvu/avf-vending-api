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
    ) AS low_stock
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
    ) AS low_stock
FROM machine_slot_state mss
JOIN slots s ON s.planogram_id = mss.planogram_id
    AND s.slot_index = mss.slot_index
JOIN products pr ON pr.id = s.product_id
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
    slot_code,
    product_id,
    event_type,
    quantity_delta,
    quantity_after,
    correlation_id,
    operator_session_id,
    refill_session_id,
    inventory_count_session_id,
    occurred_at,
    metadata
)
SELECT
    (e->>'organization_id')::uuid AS organization_id,
    (e->>'machine_id')::uuid AS machine_id,
    NULLIF (e->>'machine_cabinet_id', '')::uuid AS machine_cabinet_id,
    NULLIF (e->>'slot_code', '') AS slot_code,
    NULLIF (e->>'product_id', '')::uuid AS product_id,
    e->>'event_type' AS event_type,
    (e->>'quantity_delta')::int AS quantity_delta,
    NULLIF (e->>'quantity_after', '')::int AS quantity_after,
    NULLIF (e->>'correlation_id', '')::uuid AS correlation_id,
    NULLIF (e->>'operator_session_id', '')::uuid AS operator_session_id,
    NULLIF (e->>'refill_session_id', '')::uuid AS refill_session_id,
    NULLIF (e->>'inventory_count_session_id', '')::uuid AS inventory_count_session_id,
    coalesce((e->>'occurred_at')::timestamptz, now()) AS occurred_at,
    coalesce(e->'metadata', '{}'::jsonb) AS metadata
FROM
    jsonb_array_elements(sqlc.arg(events_json)::jsonb) AS e
RETURNING
    id;

-- name: InventoryAdminListInventoryEventsByMachine :many
SELECT
    id,
    organization_id,
    machine_id,
    machine_cabinet_id,
    slot_code,
    product_id,
    event_type,
    quantity_delta,
    quantity_after,
    correlation_id,
    operator_session_id,
    refill_session_id,
    inventory_count_session_id,
    occurred_at,
    metadata
FROM
    inventory_events
WHERE
    machine_id = $1
    AND ($2::boolean IS FALSE OR occurred_at >= $3::timestamptz)
    AND ($4::boolean IS FALSE OR occurred_at <= $5::timestamptz)
ORDER BY
    occurred_at DESC,
    id DESC
LIMIT $6 OFFSET $7;

-- name: InventoryAdminCountInventoryEventsByIdempotencyKey :one
SELECT
    count(*)::bigint
FROM
    inventory_events
WHERE
    machine_id = $1
    AND (metadata ->> 'idempotency_key') = $2::text;
