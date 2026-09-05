-- name: GetMachineLayoutState :one
SELECT
    machine_id,
    desired_source,
    desired_assignment_id,
    desired_layout_version_id,
    desired_revision,
    desired_fingerprint,
    desired_updated_at,
    reported_source,
    reported_assignment_id,
    reported_layout_version_id,
    reported_revision,
    reported_fingerprint,
    reported_at,
    reported_device_instance_id,
    apply_failure_reason,
    updated_at
FROM machine_layout_state
WHERE machine_id = sqlc.arg(machine_id);

-- name: UpsertMachineLayoutStateDesired :exec
INSERT INTO machine_layout_state (
    machine_id,
    desired_source,
    desired_assignment_id,
    desired_layout_version_id,
    desired_revision,
    desired_fingerprint,
    desired_updated_at,
    updated_at
) VALUES (
    sqlc.arg(machine_id),
    sqlc.arg(desired_source),
    sqlc.arg(desired_assignment_id),
    sqlc.arg(desired_layout_version_id),
    sqlc.arg(desired_revision),
    sqlc.arg(desired_fingerprint),
    now(),
    now()
)
ON CONFLICT (machine_id) DO UPDATE SET
    desired_source = EXCLUDED.desired_source,
    desired_assignment_id = EXCLUDED.desired_assignment_id,
    desired_layout_version_id = EXCLUDED.desired_layout_version_id,
    desired_revision = EXCLUDED.desired_revision,
    desired_fingerprint = EXCLUDED.desired_fingerprint,
    desired_updated_at = EXCLUDED.desired_updated_at,
    updated_at = now();

-- name: UpdateMachineLayoutStateReported :execrows
UPDATE machine_layout_state
SET
    reported_source = sqlc.arg(reported_source),
    reported_assignment_id = sqlc.arg(reported_assignment_id),
    reported_layout_version_id = sqlc.arg(reported_layout_version_id),
    reported_revision = sqlc.arg(reported_revision),
    reported_fingerprint = sqlc.arg(reported_fingerprint),
    reported_at = sqlc.arg(reported_at),
    reported_device_instance_id = sqlc.arg(reported_device_instance_id),
    apply_failure_reason = sqlc.narg(apply_failure_reason),
    updated_at = now()
WHERE
    machine_id = sqlc.arg(machine_id)
    AND (
        reported_revision IS NULL
        OR reported_revision < sqlc.arg(reported_revision)
    );

-- name: GetCurrentMachineLayoutAssignment :one
SELECT *
FROM machine_layout_assignments
WHERE
    machine_id = sqlc.arg(machine_id)
    AND source = sqlc.arg(source)
    AND is_current = true;

-- name: CloseCurrentMachineLayoutAssignment :exec
UPDATE machine_layout_assignments
SET
    is_current = false,
    effective_to = now()
WHERE
    machine_id = sqlc.arg(machine_id)
    AND source = sqlc.arg(source)
    AND is_current = true;

-- name: InsertMachineLayoutAssignment :one
INSERT INTO machine_layout_assignments (
    machine_id,
    source,
    layout_id,
    layout_version_id,
    org_layout_version_id,
    revision,
    grid_rows,
    grid_cols,
    fingerprint,
    is_current,
    created_by
) VALUES (
    sqlc.arg(machine_id),
    sqlc.arg(source),
    sqlc.narg(layout_id),
    sqlc.narg(layout_version_id),
    sqlc.narg(org_layout_version_id),
    sqlc.arg(revision),
    sqlc.arg(grid_rows),
    sqlc.arg(grid_cols),
    sqlc.arg(fingerprint),
    true,
    sqlc.narg(created_by)
)
RETURNING *;

-- name: NextMachineLayoutAssignmentRevision :one
SELECT COALESCE(MAX(revision), 0) + 1 AS next_revision
FROM machine_layout_assignments
WHERE
    machine_id = sqlc.arg(machine_id)
    AND source = sqlc.arg(source);

-- name: GetMachinePlanogramVersionByID :one
SELECT *
FROM machine_planogram_versions
WHERE id = sqlc.arg(id);

-- name: UpdateMachineSlotLayoutGridDimensions :exec
UPDATE machine_slot_layouts
SET
    grid_rows = sqlc.arg(grid_rows),
    grid_cols = sqlc.arg(grid_cols)
WHERE id = sqlc.arg(id);

-- name: UpsertMachineLocalLayoutMirror :exec
INSERT INTO machine_local_layout_mirror (
    machine_id,
    local_layout_id,
    revision,
    grid_rows,
    grid_cols,
    slots,
    fingerprint,
    reported_at,
    device_instance_id,
    updated_at
) VALUES (
    sqlc.arg(machine_id),
    sqlc.arg(local_layout_id),
    sqlc.arg(revision),
    sqlc.arg(grid_rows),
    sqlc.arg(grid_cols),
    COALESCE(NULLIF(sqlc.arg('slots')::text, '')::jsonb, '[]'::jsonb),
    sqlc.arg(fingerprint),
    sqlc.arg(reported_at),
    sqlc.arg(device_instance_id),
    now()
)
ON CONFLICT (machine_id) DO UPDATE SET
    local_layout_id = EXCLUDED.local_layout_id,
    revision = EXCLUDED.revision,
    grid_rows = EXCLUDED.grid_rows,
    grid_cols = EXCLUDED.grid_cols,
    slots = EXCLUDED.slots,
    fingerprint = EXCLUDED.fingerprint,
    reported_at = EXCLUDED.reported_at,
    device_instance_id = EXCLUDED.device_instance_id,
    updated_at = now()
WHERE
    machine_local_layout_mirror.revision < EXCLUDED.revision;

-- name: GetMachineLocalLayoutMirror :one
SELECT *
FROM machine_local_layout_mirror
WHERE machine_id = sqlc.arg(machine_id);

-- name: ListMachineSlotLayoutsForDimensionAudit :many
SELECT *
FROM machine_slot_layouts
WHERE grid_rows IS NULL OR grid_cols IS NULL;

-- name: InsertLayoutDimensionAudit :exec
INSERT INTO layout_dimension_migration_audit (machine_slot_layout_id, class, evidence)
VALUES (sqlc.arg(machine_slot_layout_id), sqlc.arg(class), sqlc.arg(evidence))
ON CONFLICT (machine_slot_layout_id) DO UPDATE SET
    class = EXCLUDED.class,
    evidence = EXCLUDED.evidence,
    audited_at = now();

-- name: GetLayoutAssignmentIdempotency :one
SELECT scope_id, idempotency_key, request_hash, response_json, created_at
FROM layout_assignment_idempotency
WHERE
    scope_id = sqlc.arg(scope_id)
    AND idempotency_key = sqlc.arg(idempotency_key);

-- name: InsertLayoutAssignmentIdempotency :exec
INSERT INTO layout_assignment_idempotency (scope_id, idempotency_key, request_hash, response_json)
VALUES (
    sqlc.arg(scope_id),
    sqlc.arg(idempotency_key),
    sqlc.arg(request_hash),
    sqlc.arg(response_json)
);

-- name: ListLayoutDimensionMigrationAuditSummary :many
SELECT class, count(*)::bigint AS count
FROM layout_dimension_migration_audit
GROUP BY class
ORDER BY class;

-- name: ListLayoutDimensionMigrationAuditRequiresReview :many
SELECT
    a.machine_slot_layout_id,
    a.class,
    a.evidence,
    a.audited_at,
    l.layout_key,
    l.grid_rows,
    l.grid_cols
FROM layout_dimension_migration_audit a
JOIN machine_slot_layouts l ON l.id = a.machine_slot_layout_id
WHERE a.class = 'REQUIRES_REVIEW'
ORDER BY a.audited_at DESC;

-- name: CountMachineSlotLayoutsMissingDimensionAudit :one
SELECT count(*)::bigint AS count
FROM machine_slot_layouts l
WHERE NOT EXISTS (
    SELECT 1
    FROM layout_dimension_migration_audit a
    WHERE a.machine_slot_layout_id = l.id
);

-- name: CountMachineSlotLayoutsMissingDimensions :one
SELECT count(*)::bigint AS count
FROM machine_slot_layouts
WHERE grid_rows IS NULL OR grid_cols IS NULL;
