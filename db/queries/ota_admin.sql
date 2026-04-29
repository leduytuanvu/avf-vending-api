-- name: OtaAdminGetArtifactForOrg :one
SELECT *
FROM ota_artifacts
WHERE
    organization_id = $1
    AND id = $2;

-- name: OtaAdminGetCampaign :one
SELECT
    c.*
FROM ota_campaigns c
WHERE
    c.organization_id = $1
    AND c.id = $2;

-- name: OtaAdminInsertCampaign :one
INSERT INTO ota_campaigns (
    organization_id,
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
    $9,
    $10
)
RETURNING *;

-- name: OtaAdminUpdateCampaignPatch :one
UPDATE ota_campaigns c
SET
    name = $3,
    artifact_version = $4,
    campaign_type = $5,
    rollout_strategy = $6,
    canary_percent = $7,
    rollback_artifact_id = $8,
    updated_at = now()
WHERE
    c.organization_id = $1
    AND c.id = $2
RETURNING *;

-- name: OtaAdminUpdateCampaignStatusFields :one
UPDATE ota_campaigns c
SET
    status = $3,
    approved_by = $4,
    approved_at = $5,
    rollout_next_offset = $6,
    paused_at = $7,
    updated_at = now()
WHERE
    c.organization_id = $1
    AND c.id = $2
RETURNING *;

-- name: OtaAdminListCampaigns :many
SELECT
    c.id AS campaign_id,
    c.organization_id,
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
    c.organization_id = $1
    AND ($2::boolean IS FALSE OR c.status = $3::text)
    AND c.created_at >= $4::timestamptz
    AND c.created_at <= $5::timestamptz
ORDER BY
    c.created_at DESC
LIMIT $6 OFFSET $7;

-- name: OtaAdminCountCampaigns :one
SELECT
    count(*)::bigint AS cnt
FROM ota_campaigns c
WHERE
    c.organization_id = $1
    AND ($2::boolean IS FALSE OR c.status = $3::text)
    AND c.created_at >= $4::timestamptz
    AND c.created_at <= $5::timestamptz;

-- name: OtaAdminDeleteTargetsForCampaign :exec
DELETE FROM ota_campaign_targets t
WHERE
    t.campaign_id = $1;

-- name: OtaAdminInsertCampaignTarget :one
INSERT INTO ota_campaign_targets (campaign_id, machine_id, state)
VALUES ($1, $2, 'pending')
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
INSERT INTO ota_campaign_events (organization_id, campaign_id, event_type, payload, actor_id)
VALUES ($1, $2, $3, $4, $5);

-- name: OtaAdminUpsertMachineResult :one
INSERT INTO ota_machine_results (
    organization_id,
    campaign_id,
    machine_id,
    wave,
    command_id,
    status
)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT ON CONSTRAINT ux_ota_machine_results_campaign_machine_wave DO UPDATE
SET
    command_id = EXCLUDED.command_id,
    status = EXCLUDED.status,
    updated_at = now()
RETURNING *;

-- name: OtaAdminListMachineResultsForCampaign :many
SELECT
    id,
    organization_id,
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
    organization_id = $1
    AND campaign_id = $2
ORDER BY
    machine_id ASC,
    wave ASC;

-- name: OtaAdminValidateMachinesBelongToOrg :many
SELECT
    m.id
FROM machines m
WHERE
    m.organization_id = $1
    AND m.id = ANY ($2::uuid[]);
