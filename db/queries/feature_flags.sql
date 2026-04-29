-- Feature flags (tenant-scoped).

-- name: FeatureFlagsInsert :one
INSERT INTO feature_flags (
    organization_id,
    flag_key,
    display_name,
    description,
    enabled,
    metadata
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: FeatureFlagsGetByID :one
SELECT *
FROM feature_flags
WHERE
    id = $1
    AND organization_id = $2;

-- name: FeatureFlagsGetByKey :one
SELECT *
FROM feature_flags
WHERE
    organization_id = $1
    AND flag_key = $2;

-- name: FeatureFlagsUpdate :one
UPDATE feature_flags
SET
    display_name = $3,
    description = $4,
    enabled = $5,
    metadata = $6,
    updated_at = now()
WHERE
    id = $1
    AND organization_id = $2
RETURNING *;

-- name: FeatureFlagsListByOrganization :many
SELECT *
FROM feature_flags
WHERE
    organization_id = $1
ORDER BY
    flag_key ASC
LIMIT $2
OFFSET $3;

-- name: FeatureFlagsCountByOrganization :one
SELECT count(*)::bigint
FROM feature_flags
WHERE
    organization_id = $1;

-- name: FeatureFlagTargetsDeleteByFlag :exec
DELETE FROM feature_flag_targets
WHERE
    feature_flag_id = $1
    AND organization_id = $2;

-- name: FeatureFlagTargetsInsert :one
INSERT INTO feature_flag_targets (
    organization_id,
    feature_flag_id,
    target_type,
    site_id,
    machine_id,
    hardware_profile_id,
    canary_percent,
    priority,
    enabled,
    metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING *;

-- name: FeatureFlagTargetsByFlagID :many
SELECT *
FROM feature_flag_targets
WHERE
    feature_flag_id = $1
ORDER BY
    priority DESC,
    created_at ASC;

-- name: FeatureFlagTargetsByOrganization :many
SELECT *
FROM feature_flag_targets
WHERE
    organization_id = $1;

-- name: FeatureFlagsResolveMachineContext :one
SELECT
    m.id AS machine_id,
    m.organization_id,
    m.site_id,
    m.hardware_profile_id
FROM machines m
WHERE
    m.id = $1;

-- name: MachineAppliedConfigRevision :one
SELECT
    COALESCE(MAX(config_revision), 0)::int AS rev
FROM machine_configs
WHERE
    machine_id = $1;

-- Machine config versions / rollouts.

-- name: MachineConfigVersionsInsert :one
INSERT INTO machine_config_versions (
    organization_id,
    version_label,
    config_payload,
    parent_version_id
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: MachineConfigVersionsGetByID :one
SELECT *
FROM machine_config_versions
WHERE
    id = $1
    AND organization_id = $2;

-- name: MachineConfigRolloutsInsert :one
INSERT INTO machine_config_rollouts (
    organization_id,
    target_version_id,
    previous_version_id,
    status,
    canary_percent,
    scope_type,
    site_id,
    machine_id,
    hardware_profile_id,
    metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING *;

-- name: MachineConfigRolloutsUpdateStatus :one
UPDATE machine_config_rollouts
SET
    status = $3,
    updated_at = now()
WHERE
    id = $1
    AND organization_id = $2
RETURNING *;

-- name: MachineConfigRolloutsListByOrganization :many
SELECT *
FROM machine_config_rollouts
WHERE
    organization_id = $1
ORDER BY
    created_at DESC
LIMIT $2
OFFSET $3;

-- name: MachineConfigRolloutsCountByOrganization :one
SELECT count(*)::bigint
FROM machine_config_rollouts
WHERE
    organization_id = $1;

-- name: MachineConfigRolloutsGetByID :one
SELECT *
FROM machine_config_rollouts
WHERE
    id = $1
    AND organization_id = $2;

-- name: MachineConfigRolloutsPendingForMachine :many
SELECT
    r.*
FROM
    machine_config_rollouts r
    INNER JOIN machines m ON m.id = $1
        AND m.organization_id = r.organization_id
WHERE
    r.organization_id = m.organization_id
    AND r.status IN ('pending', 'in_progress')
    AND (
        (
            r.scope_type = 'organization'
        )
        OR (
            r.scope_type = 'site'
            AND r.site_id = m.site_id
        )
        OR (
            r.scope_type = 'machine'
            AND r.machine_id = m.id
        )
        OR (
            r.scope_type = 'hardware_profile'
            AND r.hardware_profile_id IS NOT DISTINCT FROM m.hardware_profile_id
        )
    )
ORDER BY
    r.created_at DESC;
