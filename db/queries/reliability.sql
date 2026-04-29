-- name: InsertOutboxEvent :one
INSERT INTO outbox_events (
    organization_id,
    topic,
    event_type,
    payload,
    aggregate_type,
    aggregate_id,
    idempotency_key
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING
    id,
    organization_id,
    topic,
    event_type,
    payload,
    aggregate_type,
    aggregate_id,
    idempotency_key,
    created_at,
    published_at,
    publish_attempt_count,
    last_publish_error,
    last_publish_attempt_at,
    next_publish_after,
    dead_lettered_at,
    status,
    locked_by,
    locked_until,
    updated_at,
    max_publish_attempts;

-- name: ListOutboxUnpublished :many
SELECT
    id,
    organization_id,
    topic,
    event_type,
    payload,
    aggregate_type,
    aggregate_id,
    idempotency_key,
    created_at,
    published_at,
    publish_attempt_count,
    last_publish_error,
    last_publish_attempt_at,
    next_publish_after,
    dead_lettered_at,
    status,
    locked_by,
    locked_until,
    updated_at,
    max_publish_attempts
FROM outbox_events
WHERE
    published_at IS NULL
    AND dead_lettered_at IS NULL
    AND status NOT IN ('published', 'dead_letter')
    AND (
        next_publish_after IS NULL
        OR next_publish_after <= now()
    )
    AND (
        status IN ('pending', 'failed')
        OR (
            status = 'publishing'
            AND (
                locked_until IS NULL
                OR locked_until < now()
            )
        )
    )
ORDER BY
    created_at ASC,
    id ASC
LIMIT $1;

-- name: LeaseOutboxForPublish :many
WITH candidates AS (
    SELECT
        id
    FROM
        outbox_events
    WHERE
        published_at IS NULL
        AND dead_lettered_at IS NULL
        AND status NOT IN ('published', 'dead_letter')
        AND (
            next_publish_after IS NULL
            OR next_publish_after <= now()
        )
        AND (
            status IN ('pending', 'failed')
            OR (
                status = 'publishing'
                AND (
                    locked_until IS NULL
                    OR locked_until < now()
                )
            )
        )
        AND created_at <= (now() - (sqlc.arg('min_age_seconds')::bigint * interval '1 second'))
    ORDER BY
        created_at ASC,
        id ASC
    LIMIT sqlc.arg('batch_limit')
    FOR UPDATE
        SKIP LOCKED
)
UPDATE outbox_events AS o
SET
    status = 'publishing',
    locked_by = sqlc.arg('worker_id'),
    locked_until = now() + (sqlc.arg('lock_ttl_seconds')::bigint * interval '1 second'),
    updated_at = now()
FROM
    candidates AS c
WHERE
    o.id = c.id
RETURNING
    o.id,
    o.organization_id,
    o.topic,
    o.event_type,
    o.payload,
    o.aggregate_type,
    o.aggregate_id,
    o.idempotency_key,
    o.created_at,
    o.published_at,
    o.publish_attempt_count,
    o.last_publish_error,
    o.last_publish_attempt_at,
    o.next_publish_after,
    o.dead_lettered_at,
    o.status,
    o.locked_by,
    o.locked_until,
    o.updated_at,
    o.max_publish_attempts;

-- name: RecordOutboxPublishFailure :exec
UPDATE outbox_events
SET
    publish_attempt_count = publish_attempt_count + 1,
    last_publish_error = $2,
    last_publish_attempt_at = now(),
    next_publish_after = CASE
        WHEN $4::boolean THEN NULL
        ELSE $3
    END,
    dead_lettered_at = CASE
        WHEN $4::boolean THEN now()
        ELSE dead_lettered_at
    END,
    status = CASE
        WHEN $4::boolean THEN 'dead_letter'::text
        ELSE 'failed'::text
    END,
    locked_by = NULL,
    locked_until = NULL,
    updated_at = now()
WHERE
    id = $1
    AND published_at IS NULL
    AND dead_lettered_at IS NULL;

-- name: GetOutboxPipelineStats :one
SELECT
    COUNT(*) FILTER (
        WHERE
            published_at IS NULL
            AND dead_lettered_at IS NULL
    )::bigint AS pending_total,
    COUNT(*) FILTER (
        WHERE
            published_at IS NULL
            AND dead_lettered_at IS NULL
            AND status IN ('pending', 'failed')
            AND (
                next_publish_after IS NULL
                OR next_publish_after <= now()
            )
            AND (
                locked_until IS NULL
                OR locked_until < now()
            )
    )::bigint AS pending_due_now,
    COUNT(*) FILTER (
        WHERE
            dead_lettered_at IS NOT NULL
            OR status = 'dead_letter'
    )::bigint AS dead_lettered_total,
    COUNT(*) FILTER (
        WHERE
            published_at IS NULL
            AND dead_lettered_at IS NULL
            AND status = 'publishing'
            AND locked_until IS NOT NULL
            AND locked_until >= now()
    )::bigint AS publishing_leased_total,
    MIN(created_at) FILTER (
        WHERE
            published_at IS NULL
            AND dead_lettered_at IS NULL
    ) AS oldest_pending_created_at,
    COALESCE(
        MAX(publish_attempt_count) FILTER (
            WHERE
                published_at IS NULL
                AND dead_lettered_at IS NULL
        ),
        0
    )::bigint AS max_pending_attempts,
    COUNT(*) FILTER (
        WHERE
            published_at IS NULL
            AND dead_lettered_at IS NULL
            AND status = 'failed'
    )::bigint AS failed_pending_total
FROM
    outbox_events;

-- name: GetOutboxByTopicAndIdempotency :one
SELECT
    id,
    organization_id,
    topic,
    event_type,
    payload,
    aggregate_type,
    aggregate_id,
    idempotency_key,
    created_at,
    published_at,
    publish_attempt_count,
    last_publish_error,
    last_publish_attempt_at,
    next_publish_after,
    dead_lettered_at,
    status,
    locked_by,
    locked_until,
    updated_at,
    max_publish_attempts
FROM outbox_events
WHERE
    topic = $1
    AND idempotency_key = $2;
