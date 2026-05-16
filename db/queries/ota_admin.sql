-- name: OtaAdminGetArtifactForOrg :one
SELECT *
FROM ota_artifacts
WHERE
    id = $1;

-- name: OtaAdminGetCampaign :one
SELECT
    c.*
FROM ota_campaigns c
WHERE
    c.id = $1;

-- name: OtaAdminInsertCampaign :one
INSERT INTO ota_campaigns (
    name,
    artifact_id,
    artifact_version,
    campaign_type,
    rollout_strategy,
    canary_percent,
    rollback_artifact_id,
    created_by,
    status
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
)
RETURNING *;

-- name: OtaAdminUpdateCampaignPatch :one
UPDATE ota_campaigns c
SET
    name = $1,
    artifact_version = $2,
    campaign_type = $3,
    rollout_strategy = $4,
    canary_percent = $5,
    rollback_artifact_id = $6,
    updated_at = now()
WHERE
    c.id = $7
RETURNING *;

-- name: OtaAdminUpdateCampaignStatusFields :one
UPDATE ota_campaigns c
SET
    status = $1,
    approved_by = $2,
    approved_at = $3,
    rollout_next_offset = $4,
    paused_at = $5,
    updated_at = now()
WHERE
    c.id = $6
RETURNING *;

-- name: OtaAdminListCampaigns :many
SELECT
    c.id AS campaign_id,
    c.name AS campaign_name,
    c.rollout_strategy,
    c.status AS campaign_status,
    c.campaign_type,
    c.canary_percent,
    c.rollout_next_offset,
    c.created_at,
    c.updated_at,
    c.approved_at,
    a.id AS artifact_id,
    a.semver AS artifact_semver,
    a.storage_key AS artifact_storage_key,
    c.artifact_version,
    c.rollback_artifact_id
FROM ota_campaigns c
INNER JOIN ota_artifacts a ON a.id = c.artifact_id
WHERE
    ($1::boolean IS FALSE OR c.status = $2::text)
    AND c.created_at >= $3::timestamptz
    AND c.created_at <= $4::timestamptz
ORDER BY
    c.created_at DESC
LIMIT $5 OFFSET $6;

-- name: OtaAdminCountCampaigns :one
SELECT
    count(*)::bigint AS cnt
FROM ota_campaigns c
WHERE
    ($1::boolean IS FALSE OR c.status = $2::text)
    AND c.created_at >= $3::timestamptz
    AND c.created_at <= $4::timestamptz;

-- name: OtaAdminDeleteTargetsForCampaign :exec
DELETE FROM ota_campaign_targets t
WHERE
    t.campaign_id = $1;

-- name: OtaAdminInsertCampaignTarget :one
INSERT INTO ota_campaign_targets (
    campaign_id,
    machine_id,
    state
)
VALUES (
    $1,
    $2,
    'pending'
)
RETURNING *;

-- name: OtaAdminListCampaignTargetsSorted :many
SELECT
    t.id,
    t.campaign_id,
    t.machine_id,
    t.state,
    t.last_error,
    t.updated_at
FROM ota_campaign_targets t
WHERE
    t.campaign_id = $1
ORDER BY
    t.machine_id ASC;

-- name: OtaAdminInsertCampaignEvent :execrows
INSERT INTO ota_campaign_events (
    campaign_id,
    event_type,
    payload,
    actor_id
)
VALUES (
    $1,
    $2,
    $3,
    $4
);

-- name: OtaAdminUpsertMachineResult :one
INSERT INTO ota_machine_results (
    campaign_id,
    machine_id,
    wave,
    command_id,
    status
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
)
ON CONFLICT ON CONSTRAINT ux_ota_machine_results_campaign_machine_wave DO UPDATE
SET
    command_id = EXCLUDED.command_id,
    status = EXCLUDED.status,
    updated_at = now()
RETURNING *;

-- name: OtaAdminListMachineResultsForCampaign :many
SELECT
    id,
    campaign_id,
    machine_id,
    wave,
    command_id,
    status,
    last_error,
    updated_at,
    created_at
FROM ota_machine_results
WHERE
    campaign_id = $1
ORDER BY
    machine_id ASC,
    wave ASC;

-- name: OtaAdminValidateMachinesBelongToOrg :many
SELECT
    m.id
FROM machines m
WHERE
    m.id = ANY ($1::uuid[]);
