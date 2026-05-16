-- name: CountRolloutTargetsByStatus :one
SELECT
    count(*)::bigint AS cnt
FROM rollout_targets
WHERE
    campaign_id = $1
    AND TRUE
    AND status = $2;

-- name: InsertRolloutCampaign :one
INSERT INTO rollout_campaigns (
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
    $5
)
RETURNING *;

-- name: GetRolloutCampaignByID :one
SELECT *
FROM rollout_campaigns
WHERE
    id = $1
    AND TRUE;

-- name: ListRolloutCampaigns :many
SELECT *
FROM rollout_campaigns
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: UpdateRolloutCampaignStatusOnly :one
UPDATE rollout_campaigns
SET
    status = $1,
    updated_at = now ()
WHERE
    id = $2
    AND TRUE
RETURNING *;

-- name: MarkRolloutCampaignStarted :one
UPDATE rollout_campaigns
SET
    started_at = COALESCE(started_at, now ()),
    updated_at = now ()
WHERE
    id = $1
    AND TRUE
RETURNING *;

-- name: MarkRolloutCampaignCompleted :one
UPDATE rollout_campaigns
SET
    completed_at = COALESCE(completed_at, now ()),
    status = 'completed',
    updated_at = now ()
WHERE
    id = $1
    AND TRUE
RETURNING *;

-- name: MarkRolloutCampaignCancelled :one
UPDATE rollout_campaigns
SET
    cancelled_at = COALESCE(cancelled_at, now ()),
    status = 'cancelled',
    updated_at = now ()
WHERE
    id = $1
    AND TRUE
RETURNING *;

-- name: MarkRolloutCampaignRolledBack :one
UPDATE rollout_campaigns
SET
    status = 'rolled_back',
    completed_at = COALESCE(completed_at, now ()),
    updated_at = now ()
WHERE
    id = $1
    AND TRUE
RETURNING *;

-- name: UpdateRolloutCampaignStrategy :exec
UPDATE rollout_campaigns
SET
    strategy = $1,
    updated_at = now ()
WHERE
    id = $2
    AND TRUE;

-- name: RolloutSkipPendingTargets :exec
UPDATE rollout_targets
SET
    status = 'skipped',
    updated_at = now ()
WHERE
    campaign_id = $1
    AND TRUE
    AND status = 'pending';

-- name: InsertRolloutTargetRow :one
INSERT INTO rollout_targets (
    campaign_id,
    machine_id,
    status
)
VALUES (
    $1,
    $2,
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
    AND TRUE
    AND status = 'succeeded';

-- name: UpdateRolloutTargetDispatch :one
UPDATE rollout_targets
SET
    status = $1,
    command_id = $2,
    err_message = $3,
    updated_at = now ()
WHERE
    id = $4
    AND TRUE
    AND campaign_id = $5
RETURNING *;

-- name: ListRolloutTargetsByCampaign :many
SELECT *
FROM rollout_targets
WHERE
    campaign_id = $1
    AND TRUE
ORDER BY created_at ASC;

-- name: ListRolloutPendingTargets :many
SELECT *
FROM rollout_targets
WHERE
    campaign_id = $1
    AND TRUE
    AND status = 'pending'
ORDER BY created_at ASC
LIMIT $2;

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
        AND TRUE
        AND rt2.command_id IS NOT NULL
        AND rt2.status NOT IN ('skipped', 'rolled_back')
) AS v
WHERE
    rt.id = v.id
    AND rt.status IN ('pending', 'dispatched', 'acknowledged')
    AND v.new_status IS NOT NULL
    AND v.new_status <> rt.status::text;

-- name: RolloutListMachines :many
SELECT
    m.id,
    m.site_id,
    m.hardware_profile_id,
    m.status,
    m.model
FROM machines m
ORDER BY
    m.id;

-- name: RolloutResolveTagIDsBySlugs :many
SELECT
    id,
    slug
FROM
    tags
WHERE
    lower(trim(slug)) = ANY (sqlc.arg('slugs')::text[]);

-- name: RolloutMatchTagIDs :many
SELECT
    id
FROM
    tags
WHERE
    id = ANY (sqlc.arg('tag_ids')::uuid[]);

-- name: RolloutListMachineIDsWithAllTags :many
SELECT
    m.id
FROM
    machines m
    INNER JOIN machine_tag_assignments mta ON mta.machine_id = m.id
    AND TRUE
WHERE
    mta.tag_id = ANY (sqlc.arg('tag_ids')::uuid[])
GROUP BY
    m.id
HAVING
    count(DISTINCT mta.tag_id) = sqlc.arg('required_count')::int;
