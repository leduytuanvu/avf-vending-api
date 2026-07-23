-- name: PlanogramGetPublishedMetaForMachine :one
SELECT
    m.published_planogram_version_id,
    v.version_no
FROM machines m
LEFT JOIN machine_planogram_versions v ON v.id = m.published_planogram_version_id
WHERE
    m.id = $1;

-- name: PlanogramGetMachineDraftByID :one
SELECT
    id,
    machine_id,
    status,
    snapshot,
    created_at,
    updated_at
FROM machine_planogram_drafts
WHERE
    id = $1
    AND TRUE
    AND machine_id = $2;

-- name: PlanogramInsertDraft :one
INSERT INTO machine_planogram_drafts (
    machine_id,
    status,
    snapshot
)
VALUES (
    $1,
    $2,
    $3
)
RETURNING
    id,
    machine_id,
    status,
    snapshot,
    created_at,
    updated_at;

-- name: PlanogramPatchDraftSnapshot :one
UPDATE machine_planogram_drafts d
SET
    snapshot = sqlc.arg(snapshot)::jsonb,
    status = sqlc.arg(status)::text,
    updated_at = now ()
WHERE
    d.id = sqlc.arg(id)
    AND TRUE
    AND d.machine_id = sqlc.arg(machine_id)
RETURNING
    id,
    machine_id,
    status,
    snapshot,
    created_at,
    updated_at;

-- name: PlanogramNextMachineVersionNo :one
SELECT
    COALESCE(MAX(version_no), 0)::int AS next_seq
FROM machine_planogram_versions
WHERE
    machine_id = $1;

-- name: PlanogramInsertVersion :one
INSERT INTO machine_planogram_versions (
    machine_id,
    version_no,
    snapshot,
    source_draft_id,
    published_by
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
)
RETURNING
    id,
    machine_id,
    version_no,
    snapshot,
    source_draft_id,
    published_at,
    published_by;

-- name: PlanogramInsertVersionSlot :exec
INSERT INTO machine_planogram_slots (
    version_id,
    cabinet_code,
    layout_key,
    layout_revision,
    slot_code,
    legacy_slot_index,
    product_id,
    max_quantity,
    price_minor
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9
);

-- name: PlanogramSetMachinePublishedVersion :exec
UPDATE machines
SET
    published_planogram_version_id = $1,
    updated_at = now ()
WHERE
    id = $2
    AND TRUE;

-- name: PlanogramGetVersionByIDForMachine :one
SELECT
    id,
    machine_id,
    version_no,
    snapshot,
    source_draft_id,
    published_at,
    published_by
FROM machine_planogram_versions
WHERE
    id = $1
    AND TRUE
    AND machine_id = $2;

-- name: PlanogramListVersionsForMachine :many
SELECT
    id,
    machine_id,
    version_no,
    snapshot,
    source_draft_id,
    published_at,
    published_by
FROM machine_planogram_versions
WHERE
    machine_id = $1
ORDER BY
    version_no DESC;

-- name: PlanogramListDraftsForMachine :many
SELECT
    id,
    machine_id,
    status,
    snapshot,
    created_at,
    updated_at
FROM machine_planogram_drafts
WHERE
    machine_id = $1
ORDER BY
    updated_at DESC;

-- name: PlanogramInsertTemplate :one
INSERT INTO planogram_templates (
    name,
    description,
    snapshot
)
VALUES (
    $1,
    $2,
    $3
)
RETURNING
    id,
    name,
    description,
    snapshot,
    created_at,
    updated_at;

-- name: PlanogramSnapshotUpdateMachineAckPlanogram :exec
UPDATE machine_current_snapshot
SET
    last_acknowledged_planogram_version_id = $1,
    updated_at = now ()
WHERE
    machine_id = $2;

-- name: PlanogramSnapshotUpdateMachineAckConfigRevision :exec
UPDATE machine_current_snapshot
SET
    last_acknowledged_config_revision = $1,
    updated_at = now ()
WHERE
    machine_id = $2;

-- name: PlanogramListActiveMergePairsForMachine :many
SELECT
    id,
    machine_id,
    left_slot_code,
    right_slot_code,
    cabinet_code,
    layout_key,
    layout_revision,
    revision,
    operator_session_id,
    merged_at,
    split_at,
    is_active,
    created_at,
    updated_at
FROM machine_lane_merge_pairs
WHERE
    machine_id = $1
    AND is_active = true
ORDER BY
    left_slot_code ASC;

-- name: PlanogramNextMergePairRevision :one
SELECT
    COALESCE(MAX(revision), 0)::int + 1 AS next_revision
FROM machine_lane_merge_pairs
WHERE
    machine_id = $1;

-- name: PlanogramInsertActiveMergePair :one
INSERT INTO machine_lane_merge_pairs (
    machine_id,
    left_slot_code,
    right_slot_code,
    cabinet_code,
    layout_key,
    layout_revision,
    revision,
    operator_session_id,
    merged_at,
    is_active
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    now(),
    true
)
RETURNING
    id,
    machine_id,
    left_slot_code,
    right_slot_code,
    cabinet_code,
    layout_key,
    layout_revision,
    revision,
    operator_session_id,
    merged_at,
    split_at,
    is_active,
    created_at,
    updated_at;

-- name: PlanogramSplitActiveMergePair :execrows
UPDATE machine_lane_merge_pairs
SET
    is_active = false,
    split_at = now(),
    updated_at = now()
WHERE
    machine_id = $1
    AND left_slot_code = $2
    AND is_active = true;

-- name: PlanogramDeactivateAllMergePairsForMachine :exec
UPDATE machine_lane_merge_pairs
SET
    is_active = false,
    split_at = now(),
    updated_at = now()
WHERE
    machine_id = $1
    AND is_active = true;

-- name: PlanogramMirrorSlotConfigMetadata :exec
UPDATE machine_slot_configs
SET
    metadata = $3,
    updated_at = now()
WHERE
    machine_id = $1
    AND slot_code = $2
    AND is_current = true;
