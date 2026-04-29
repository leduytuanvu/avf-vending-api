-- name: CountRolloutTargetsByStatus :one
SELECT
    count(*)::bigint AS cnt
FROM rollout_targets
WHERE
    campaign_id = $1
    AND organization_id = $2
    AND status = $3;

-- name: InsertRolloutCampaign :one
INSERT INTO rollout_campaigns (
    organization_id,
    rollout_type,
    target_version,
    status,
    strategy,
    created_by
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING *;

-- name: GetRolloutCampaignByID :one
SELECT *
FROM rollout_campaigns
WHERE
    id = $1
    AND organization_id = $2;

-- name: ListRolloutCampaigns :many
SELECT *
FROM rollout_campaigns
WHERE
    organization_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateRolloutCampaignStatusOnly :one
UPDATE rollout_campaigns
SET
    status = $3,
    updated_at = now ()
WHERE
    id = $1
    AND organization_id = $2
RETURNING *;

-- name: MarkRolloutCampaignStarted :one
UPDATE rollout_campaigns
SET
    started_at = COALESCE(started_at, now ()),
    updated_at = now ()
WHERE
    id = $1
    AND organization_id = $2
RETURNING *;

-- name: MarkRolloutCampaignCompleted :one
UPDATE rollout_campaigns
SET
    completed_at = COALESCE(completed_at, now ()),
    status = 'completed',
    updated_at = now ()
WHERE
    id = $1
    AND organization_id = $2
RETURNING *;

-- name: MarkRolloutCampaignCancelled :one
UPDATE rollout_campaigns
SET
    cancelled_at = COALESCE(cancelled_at, now ()),
    status = 'cancelled',
    updated_at = now ()
WHERE
    id = $1
    AND organization_id = $2
RETURNING *;

-- name: MarkRolloutCampaignRolledBack :one
UPDATE rollout_campaigns
SET
    status = 'rolled_back',
    completed_at = COALESCE(completed_at, now ()),
    updated_at = now ()
WHERE
    id = $1
    AND organization_id = $2
RETURNING *;

-- name: UpdateRolloutCampaignStrategy :exec
UPDATE rollout_campaigns
SET
    strategy = $3,
    updated_at = now ()
WHERE
    id = $1
    AND organization_id = $2;

-- name: RolloutSkipPendingTargets :exec
UPDATE rollout_targets
SET
    status = 'skipped',
    updated_at = now ()
WHERE
    campaign_id = $1
    AND organization_id = $2
    AND status = 'pending';

-- name: InsertRolloutTargetRow :one
INSERT INTO rollout_targets (
    organization_id,
    campaign_id,
    machine_id,
    status
)
VALUES (
    $1,
    $2,
    $3,
    'pending'
)
RETURNING *;

-- name: RolloutPrepareRollbackWave :exec
UPDATE rollout_targets
SET
    status = 'pending',
    command_id = NULL,
    err_message = NULL,
    updated_at = now ()
WHERE
    campaign_id = $1
    AND organization_id = $2
    AND status = 'succeeded';

-- name: UpdateRolloutTargetDispatch :one
UPDATE rollout_targets
SET
    status = $4,
    command_id = $5,
    err_message = $6,
    updated_at = now ()
WHERE
    id = $1
    AND organization_id = $2
    AND campaign_id = $3
RETURNING *;

-- name: ListRolloutTargetsByCampaign :many
SELECT *
FROM rollout_targets
WHERE
    campaign_id = $1
    AND organization_id = $2
ORDER BY created_at ASC;

-- name: ListRolloutPendingTargets :many
SELECT *
FROM rollout_targets
WHERE
    campaign_id = $1
    AND organization_id = $2
    AND status = 'pending'
ORDER BY created_at ASC
LIMIT $3;

-- name: RolloutRefreshTargetFromLatestAttempt :exec
UPDATE rollout_targets rt
SET
    status = v.new_status,
    err_message = v.err_message,
    updated_at = now ()
FROM (
    SELECT
        rt2.id,
        CASE
            WHEN mca.status = 'completed' THEN 'succeeded'::text
            WHEN mca.status IN (
                'failed',
                'nack',
                'ack_timeout'
            ) THEN 'failed'::text
            WHEN mca.status IN ('pending', 'sent') THEN 'dispatched'::text
            ELSE rt2.status::text
        END AS new_status,
        CASE
            WHEN mca.status IN (
                'failed',
                'nack',
                'ack_timeout'
            ) THEN COALESCE(mca.timeout_reason, mca.status)
            ELSE rt2.err_message
        END AS err_message
    FROM rollout_targets rt2
    LEFT JOIN LATERAL (
        SELECT
            status,
            timeout_reason
        FROM machine_command_attempts
        WHERE
            command_id = rt2.command_id
        ORDER BY attempt_no DESC
        LIMIT 1
    ) mca ON rt2.command_id IS NOT NULL
    WHERE
        rt2.campaign_id = $1
        AND rt2.organization_id = $2
        AND rt2.command_id IS NOT NULL
        AND rt2.status NOT IN ('skipped', 'rolled_back')
) AS v
WHERE
    rt.id = v.id
    AND rt.status IN ('pending', 'dispatched', 'acknowledged')
    AND v.new_status IS NOT NULL
    AND v.new_status <> rt.status::text;

-- name: RolloutListOrgMachines :many
SELECT
    m.id,
    m.site_id,
    m.status,
    m.model
FROM machines m
WHERE
    m.organization_id = $1
ORDER BY
    m.id;

-- name: RolloutResolveTagIDsBySlugs :many
SELECT
    id,
    slug
FROM
    tags
WHERE
    organization_id = sqlc.arg('organization_id')
    AND lower(trim(slug)) = ANY (sqlc.arg('slugs')::text[]);

-- name: RolloutMatchTagIDsForOrg :many
SELECT
    id
FROM
    tags
WHERE
    organization_id = sqlc.arg('organization_id')
    AND id = ANY (sqlc.arg('tag_ids')::uuid[]);

-- name: RolloutListMachineIDsWithAllTags :many
SELECT
    m.id
FROM
    machines m
    INNER JOIN machine_tag_assignments mta ON mta.machine_id = m.id
    AND mta.organization_id = m.organization_id
WHERE
    m.organization_id = sqlc.arg('organization_id')
    AND mta.tag_id = ANY (sqlc.arg('tag_ids')::uuid[])
GROUP BY
    m.id
HAVING
    count(DISTINCT mta.tag_id) = sqlc.arg('required_count')::int;
