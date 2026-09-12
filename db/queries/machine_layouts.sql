-- name: InsertMachineLayout :one
INSERT INTO machine_layouts (
    machine_id,
    name,
    status,
    grid_rows,
    grid_cols,
    layout_revision,
    fingerprint,
    source_template_version_id
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8
)
RETURNING *;

-- name: GetMachineLayoutByID :one
SELECT *
FROM machine_layouts
WHERE id = $1
    AND machine_id = $2
    AND NOT is_archived;

-- name: ListMachineLayoutsForMachine :many
SELECT *
FROM machine_layouts
WHERE machine_id = $1
    AND NOT is_archived
ORDER BY created_at ASC;

-- name: CountMachineLayoutsForMachine :one
SELECT count(*)::bigint AS count
FROM machine_layouts
WHERE machine_id = $1
    AND NOT is_archived;

-- name: UpdateMachineLayoutMetadata :one
UPDATE machine_layouts
SET
    name = COALESCE(sqlc.narg(name), name),
    status = COALESCE(sqlc.narg(status), status),
    layout_revision = COALESCE(sqlc.narg(layout_revision), layout_revision),
    fingerprint = COALESCE(sqlc.narg(fingerprint), fingerprint),
    updated_at = now()
WHERE id = sqlc.arg(id)
    AND machine_id = sqlc.arg(machine_id)
    AND NOT is_archived
RETURNING *;

-- name: ArchiveMachineLayout :execrows
UPDATE machine_layouts
SET
    is_archived = true,
    updated_at = now()
WHERE id = $1
    AND machine_id = $2
    AND NOT is_archived;

-- name: SetMachineActiveLayoutPointers :exec
UPDATE machines
SET
    active_layout_id = sqlc.narg(active_layout_id),
    desired_active_layout_id = sqlc.narg(desired_active_layout_id),
    reported_active_layout_id = sqlc.narg(reported_active_layout_id),
    updated_at = now()
WHERE id = $1;

-- name: InsertMachineLayoutSlot :exec
INSERT INTO machine_layout_slots (
    layout_id,
    slot_code,
    slot_ordinal,
    slot_row,
    slot_column,
    physical_lane,
    board_address,
    channel_address,
    product_id,
    max_quantity,
    price_minor,
    current_inventory,
    enabled,
    operational_state
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    $10,
    $11,
    $12,
    $13,
    $14
);

-- name: ListMachineLayoutSlots :many
SELECT *
FROM machine_layout_slots
WHERE layout_id = $1
ORDER BY slot_ordinal ASC;

-- name: InsertMachineLayoutMergePair :exec
INSERT INTO machine_layout_merge_pairs (
    layout_id,
    left_slot_code,
    right_slot_code
) VALUES (
    $1,
    $2,
    $3
);

-- name: ListMachineLayoutMergePairs :many
SELECT *
FROM machine_layout_merge_pairs
WHERE layout_id = $1
ORDER BY left_slot_code ASC;

-- name: GetMachineLayoutSnapshotHistoryBySnapshotID :one
SELECT *
FROM machine_layout_snapshot_history
WHERE snapshot_id = $1;

-- name: GetMachineLayoutSnapshotHistoryByDeviceSequence :one
SELECT *
FROM machine_layout_snapshot_history
WHERE machine_id = $1
    AND device_instance_id = $2
    AND capture_sequence = $3;

-- name: InsertMachineLayoutSnapshotHistory :one
INSERT INTO machine_layout_snapshot_history (
    snapshot_id,
    machine_id,
    layout_id,
    device_instance_id,
    capture_sequence,
    interval_key,
    captured_at,
    device_generation,
    base_server_revision,
    fingerprint,
    snapshot_reason,
    payload_version,
    payload
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    $10,
    $11,
    $12,
    $13
)
RETURNING *;

-- name: UpsertMachineLayoutDeviceState :one
INSERT INTO machine_layout_device_state (
    machine_id,
    layout_id,
    latest_snapshot_id,
    latest_capture_sequence,
    device_generation,
    fingerprint,
    reported_at,
    device_instance_id,
    updated_at
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    now()
)
ON CONFLICT (machine_id) DO UPDATE
SET
    layout_id = EXCLUDED.layout_id,
    latest_snapshot_id = EXCLUDED.latest_snapshot_id,
    latest_capture_sequence = CASE
        WHEN EXCLUDED.latest_capture_sequence >= machine_layout_device_state.latest_capture_sequence
            THEN EXCLUDED.latest_capture_sequence
        ELSE machine_layout_device_state.latest_capture_sequence
    END,
    device_generation = CASE
        WHEN EXCLUDED.latest_capture_sequence >= machine_layout_device_state.latest_capture_sequence
            THEN EXCLUDED.device_generation
        ELSE machine_layout_device_state.device_generation
    END,
    fingerprint = CASE
        WHEN EXCLUDED.latest_capture_sequence >= machine_layout_device_state.latest_capture_sequence
            THEN EXCLUDED.fingerprint
        ELSE machine_layout_device_state.fingerprint
    END,
    reported_at = CASE
        WHEN EXCLUDED.latest_capture_sequence >= machine_layout_device_state.latest_capture_sequence
            THEN EXCLUDED.reported_at
        ELSE machine_layout_device_state.reported_at
    END,
    device_instance_id = CASE
        WHEN EXCLUDED.latest_capture_sequence >= machine_layout_device_state.latest_capture_sequence
            THEN EXCLUDED.device_instance_id
        ELSE machine_layout_device_state.device_instance_id
    END,
    updated_at = now()
RETURNING *;

-- name: GetMachineLayoutDeviceState :one
SELECT *
FROM machine_layout_device_state
WHERE machine_id = $1;

-- name: ListMachineLayoutSnapshotHistoryPage :many
SELECT *
FROM machine_layout_snapshot_history
WHERE machine_id = $1
    AND (
        sqlc.narg(layout_id)::uuid IS NULL
        OR layout_id = sqlc.narg(layout_id)
    )
ORDER BY captured_at DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountMachineLayoutSnapshotHistory :one
SELECT count(*)::bigint AS count
FROM machine_layout_snapshot_history
WHERE machine_id = $1
    AND (
        sqlc.narg(layout_id)::uuid IS NULL
        OR layout_id = sqlc.narg(layout_id)
    );

-- name: ListMachinesWithoutNamedLayout :many
SELECT m.id
FROM machines m
WHERE NOT EXISTS (
    SELECT 1
    FROM machine_layouts ml
    WHERE ml.machine_id = m.id
        AND NOT ml.is_archived
);
