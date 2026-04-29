-- name: ListOutboxOpsRows :many
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
FROM
    outbox_events
ORDER BY
    id DESC
LIMIT $1
OFFSET $2;

-- name: AdminRetryOutboxDeadLetter :execrows
UPDATE outbox_events
SET
    status = 'pending',
    dead_lettered_at = NULL,
    next_publish_after = NULL,
    locked_by = NULL,
    locked_until = NULL,
    last_publish_error = NULL,
    publish_attempt_count = 0,
    updated_at = now()
WHERE
    id = $1
    AND (
        status = 'dead_letter'
        OR dead_lettered_at IS NOT NULL
    )
    AND published_at IS NULL;
