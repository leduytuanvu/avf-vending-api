-- name: TopologyListSlotLayoutsForMachine :many
SELECT
    id,
    organization_id,
    machine_id,
    machine_cabinet_id,
    layout_key,
    revision,
    layout_spec,
    status,
    created_at
FROM machine_slot_layouts
WHERE
    machine_id = $1
ORDER BY
    machine_cabinet_id,
    layout_key,
    revision;
