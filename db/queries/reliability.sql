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
    dead_lettered_at;

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
    dead_lettered_at
FROM outbox_events
WHERE
    published_at IS NULL
    AND dead_lettered_at IS NULL
    AND (
        next_publish_after IS NULL
        OR next_publish_after <= now()
    )
ORDER BY
    created_at ASC,
    id ASC
LIMIT $1;

-- name: RecordOutboxPublishFailure :exec
UPDATE outbox_events
SET
    publish_attempt_count = publish_attempt_count + 1,
    last_publish_error = $2,
    last_publish_attempt_at = now(),
    next_publish_after = CASE
        WHEN $4 THEN NULL
        ELSE $3
    END,
    dead_lettered_at = CASE
        WHEN $4 THEN now()
        ELSE dead_lettered_at
    END
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
            AND (
                next_publish_after IS NULL
                OR next_publish_after <= now()
            )
    )::bigint AS pending_due_now,
    COUNT(*) FILTER (WHERE dead_lettered_at IS NOT NULL)::bigint AS dead_lettered_total,
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
    )::bigint AS max_pending_attempts
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
    dead_lettered_at
FROM outbox_events
WHERE
    topic = $1
    AND idempotency_key = $2;
