-- Feature flags (single-company-scoped).

-- name: FeatureFlagsInsert :one
INSERT INTO feature_flags (
    flag_key,
    display_name,
    description,
    enabled,
    metadata
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
)
RETURNING *;

-- name: FeatureFlagsGetByID :one
SELECT *
FROM feature_flags
WHERE
    id = $1
    AND TRUE;

-- name: FeatureFlagsGetByKey :one
SELECT *
FROM feature_flags
WHERE
    flag_key = $1;

-- name: FeatureFlagsUpdate :one
UPDATE feature_flags
SET
    display_name = $1,
    description = $2,
    enabled = $3,
    metadata = $4,
    updated_at = now()
WHERE
    id = $5
    AND TRUE
RETURNING *;

-- name: FeatureFlagsListAll :many
SELECT *
FROM feature_flags
ORDER BY
    flag_key ASC
LIMIT $1
OFFSET $2;

-- name: FeatureFlagsCountAll :one
SELECT count(*)::bigint
FROM feature_flags;

-- name: FeatureFlagTargetsDeleteByFlag :exec
DELETE FROM feature_flag_targets
WHERE
    feature_flag_id = $1
    AND TRUE;

-- name: FeatureFlagTargetsInsert :one
INSERT INTO feature_flag_targets (
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
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9
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

-- name: FeatureFlagTargetsAll :many
SELECT *
FROM feature_flag_targets;

-- name: FeatureFlagsResolveMachineContext :one
SELECT
    m.id AS machine_id,
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
    version_label,
    config_payload,
    parent_version_id
) VALUES (
    $1,
    $2,
    $3
)
RETURNING *;

-- name: MachineConfigVersionsGetByID :one
SELECT *
FROM machine_config_versions
WHERE
    id = $1
    AND TRUE;

-- name: MachineConfigRolloutsInsert :one
INSERT INTO machine_config_rollouts (
    target_version_id,
    previous_version_id,
    status,
    canary_percent,
    rollout_target_level,
    site_id,
    machine_id,
    hardware_profile_id,
    metadata
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9
)
RETURNING *;

-- name: MachineConfigRolloutsUpdateStatus :one
UPDATE machine_config_rollouts
SET
    status = $1,
    updated_at = now()
WHERE
    id = $2
    AND TRUE
RETURNING *;

-- name: MachineConfigRolloutsListAll :many
SELECT *
FROM machine_config_rollouts
ORDER BY
    created_at DESC
LIMIT $1
OFFSET $2;

-- name: MachineConfigRolloutsCountAll :one
SELECT count(*)::bigint
FROM machine_config_rollouts;

-- name: MachineConfigRolloutsGetByID :one
SELECT *
FROM machine_config_rollouts
WHERE
    id = $1
    AND TRUE;

-- name: MachineConfigRolloutsPendingForMachine :many
SELECT
    r.*
FROM
    machine_config_rollouts r
    INNER JOIN machines m ON m.id = $1
        AND TRUE
WHERE
    r.status IN ('pending', 'in_progress')
    AND (
        (
            r.rollout_target_level = 'global'
        )
        OR (
            r.rollout_target_level = 'site'
            AND r.site_id = m.site_id
        )
        OR (
            r.rollout_target_level = 'machine'
            AND r.machine_id = m.id
        )
        OR (
            r.rollout_target_level = 'hardware_profile'
            AND r.hardware_profile_id IS NOT DISTINCT FROM m.hardware_profile_id
        )
    )
ORDER BY
    r.created_at DESC;
