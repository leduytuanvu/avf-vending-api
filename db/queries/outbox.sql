-- name: AdminGetOutboxEventByID :one
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
WHERE
    id = sqlc.arg('id');

-- name: AdminMarkOutboxManualDeadLetter :execrows
UPDATE outbox_events
SET
    status = 'dead_letter'::text,
    dead_lettered_at = now(),
    locked_by = NULL,
    locked_until = NULL,
    next_publish_after = NULL,
    last_publish_error = COALESCE(NULLIF(trim(sqlc.arg('note')::text), ''), last_publish_error),
    updated_at = now()
WHERE
    id = sqlc.arg('id')
    AND published_at IS NULL
    AND dead_lettered_at IS NULL
    AND status NOT IN ('published', 'dead_letter');

-- name: AdminListOutboxEventsPendingWindow :many
-- Lists unpublished, non-terminal outbox rows in a created_at window for operator triage (read-only).
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
WHERE
    published_at IS NULL
    AND dead_lettered_at IS NULL
    AND status NOT IN ('published', 'dead_letter')
    AND created_at >= sqlc.arg('created_after')
    AND created_at < sqlc.arg('created_before')
    AND (
        sqlc.arg('status_filter')::text = ''
        OR status = sqlc.arg('status_filter')::text
    )
ORDER BY
    created_at ASC,
    id ASC
LIMIT sqlc.arg('limit');

-- name: AdminRequeueOutboxPendingByID :execrows
-- Clears lease/backoff on a stuck pending/failed/publishing row so cmd/worker can retry (not for terminal DLQ; use replay-dlq).
UPDATE outbox_events
SET
    status = 'pending',
    next_publish_after = NULL,
    locked_by = NULL,
    locked_until = NULL,
    updated_at = now(),
    last_publish_error = CASE
        WHEN NULLIF(trim(sqlc.arg('note')::text), '') IS NOT NULL THEN sqlc.arg('note')::text
        ELSE last_publish_error
    END
WHERE
    id = sqlc.arg('id')
    AND published_at IS NULL
    AND dead_lettered_at IS NULL
    AND status NOT IN ('published', 'dead_letter');
