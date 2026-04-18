package db

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type InsertOutboxEventParams struct {
	OrganizationID *uuid.UUID
	Topic          string
	EventType      string
	Payload        []byte
	AggregateType  string
	AggregateID    uuid.UUID
	IdempotencyKey *string
}

const insertOutboxEvent = `-- name: InsertOutboxEvent :one
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
RETURNING id, organization_id, topic, event_type, payload, aggregate_type, aggregate_id, idempotency_key, created_at, published_at, publish_attempt_count, last_publish_error, last_publish_attempt_at, next_publish_after, dead_lettered_at
`

func (q *Queries) InsertOutboxEvent(ctx context.Context, arg InsertOutboxEventParams) (OutboxEvent, error) {
	row := q.db.QueryRow(ctx, insertOutboxEvent,
		arg.OrganizationID,
		arg.Topic,
		arg.EventType,
		arg.Payload,
		arg.AggregateType,
		arg.AggregateID,
		arg.IdempotencyKey,
	)
	var e OutboxEvent
	err := row.Scan(
		&e.ID,
		&e.OrganizationID,
		&e.Topic,
		&e.EventType,
		&e.Payload,
		&e.AggregateType,
		&e.AggregateID,
		&e.IdempotencyKey,
		&e.CreatedAt,
		&e.PublishedAt,
		&e.PublishAttemptCount,
		&e.LastPublishError,
		&e.LastPublishAttemptAt,
		&e.NextPublishAfter,
		&e.DeadLetteredAt,
	)
	return e, err
}

const listOutboxUnpublished = `-- name: ListOutboxUnpublished :many
SELECT id, organization_id, topic, event_type, payload, aggregate_type, aggregate_id, idempotency_key, created_at, published_at, publish_attempt_count, last_publish_error, last_publish_attempt_at, next_publish_after, dead_lettered_at
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
LIMIT $1
`

type GetOutboxByTopicAndIdempotencyParams struct {
	Topic          string
	IdempotencyKey string
}

const getOutboxByTopicAndIdempotency = `-- name: GetOutboxByTopicAndIdempotency :one
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
    AND idempotency_key = $2
`

func (q *Queries) GetOutboxByTopicAndIdempotency(ctx context.Context, arg GetOutboxByTopicAndIdempotencyParams) (OutboxEvent, error) {
	row := q.db.QueryRow(ctx, getOutboxByTopicAndIdempotency, arg.Topic, arg.IdempotencyKey)
	var e OutboxEvent
	err := row.Scan(
		&e.ID,
		&e.OrganizationID,
		&e.Topic,
		&e.EventType,
		&e.Payload,
		&e.AggregateType,
		&e.AggregateID,
		&e.IdempotencyKey,
		&e.CreatedAt,
		&e.PublishedAt,
		&e.PublishAttemptCount,
		&e.LastPublishError,
		&e.LastPublishAttemptAt,
		&e.NextPublishAfter,
		&e.DeadLetteredAt,
	)
	return e, err
}

type RecordOutboxPublishFailureParams struct {
	ID               int64
	LastPublishError string
	NextPublishAfter *time.Time
	DeadLettered     bool
}

const recordOutboxPublishFailure = `-- name: RecordOutboxPublishFailure :exec
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
    AND dead_lettered_at IS NULL
`

func (q *Queries) RecordOutboxPublishFailure(ctx context.Context, arg RecordOutboxPublishFailureParams) error {
	_, err := q.db.Exec(ctx, recordOutboxPublishFailure,
		arg.ID,
		arg.LastPublishError,
		arg.NextPublishAfter,
		arg.DeadLettered,
	)
	return err
}

const getOutboxPipelineStats = `-- name: GetOutboxPipelineStats :one
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
    outbox_events
`

type GetOutboxPipelineStatsRow struct {
	PendingTotal           int64
	PendingDueNow          int64
	DeadLetteredTotal      int64
	OldestPendingCreatedAt *time.Time
	MaxPendingAttempts     int64
}

func (q *Queries) GetOutboxPipelineStats(ctx context.Context) (GetOutboxPipelineStatsRow, error) {
	row := q.db.QueryRow(ctx, getOutboxPipelineStats)
	var i GetOutboxPipelineStatsRow
	err := row.Scan(
		&i.PendingTotal,
		&i.PendingDueNow,
		&i.DeadLetteredTotal,
		&i.OldestPendingCreatedAt,
		&i.MaxPendingAttempts,
	)
	return i, err
}

func (q *Queries) ListOutboxUnpublished(ctx context.Context, limit int32) ([]OutboxEvent, error) {
	rows, err := q.db.Query(ctx, listOutboxUnpublished, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OutboxEvent
	for rows.Next() {
		var e OutboxEvent
		if err := rows.Scan(
			&e.ID,
			&e.OrganizationID,
			&e.Topic,
			&e.EventType,
			&e.Payload,
			&e.AggregateType,
			&e.AggregateID,
			&e.IdempotencyKey,
			&e.CreatedAt,
			&e.PublishedAt,
			&e.PublishAttemptCount,
			&e.LastPublishError,
			&e.LastPublishAttemptAt,
			&e.NextPublishAfter,
			&e.DeadLetteredAt,
		); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
