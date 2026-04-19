-- name: InventoryAdminGetMachineOrg :one
SELECT id, organization_id, name, status
FROM machines
WHERE id = $1;

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
