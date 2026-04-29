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
    organization_id,
    machine_id,
    status,
    snapshot,
    created_at,
    updated_at
FROM machine_planogram_drafts
WHERE
    id = $1
    AND organization_id = $2
    AND machine_id = $3;

-- name: PlanogramInsertDraft :one
INSERT INTO machine_planogram_drafts (organization_id, machine_id, status, snapshot)
VALUES ($1, $2, $3, $4)
RETURNING
    id,
    organization_id,
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
    AND d.organization_id = sqlc.arg(organization_id)
    AND d.machine_id = sqlc.arg(machine_id)
RETURNING
    id,
    organization_id,
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
    organization_id,
    machine_id,
    version_no,
    snapshot,
    source_draft_id,
    published_by
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING
    id,
    organization_id,
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
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: PlanogramSetMachinePublishedVersion :exec
UPDATE machines
SET
    published_planogram_version_id = $2,
    updated_at = now ()
WHERE
    id = $1
    AND organization_id = $3;

-- name: PlanogramGetVersionByIDForMachine :one
SELECT
    id,
    organization_id,
    machine_id,
    version_no,
    snapshot,
    source_draft_id,
    published_at,
    published_by
FROM machine_planogram_versions
WHERE
    id = $1
    AND organization_id = $2
    AND machine_id = $3;

-- name: PlanogramListVersionsForMachine :many
SELECT
    id,
    organization_id,
    machine_id,
    version_no,
    snapshot,
    source_draft_id,
    published_at,
    published_by
FROM machine_planogram_versions
WHERE
    organization_id = $1
    AND machine_id = $2
ORDER BY
    version_no DESC;

-- name: PlanogramListDraftsForMachine :many
SELECT
    id,
    organization_id,
    machine_id,
    status,
    snapshot,
    created_at,
    updated_at
FROM machine_planogram_drafts
WHERE
    organization_id = $1
    AND machine_id = $2
ORDER BY
    updated_at DESC;

-- name: PlanogramInsertTemplate :one
INSERT INTO planogram_templates (organization_id, name, description, snapshot)
VALUES ($1, $2, $3, $4)
RETURNING
    id,
    organization_id,
    name,
    description,
    snapshot,
    created_at,
    updated_at;

-- name: PlanogramSnapshotUpdateMachineAckPlanogram :exec
UPDATE machine_current_snapshot
SET
    last_acknowledged_planogram_version_id = $2,
    updated_at = now ()
WHERE
    machine_id = $1;

-- name: PlanogramSnapshotUpdateMachineAckConfigRevision :exec
UPDATE machine_current_snapshot
SET
    last_acknowledged_config_revision = $2,
    updated_at = now ()
WHERE
    machine_id = $1;
