package reliability

import (
	"time"

	"github.com/google/uuid"
)

// OutboxEvent is a durable async propagation record.
type OutboxEvent struct {
	ID             int64
	OrganizationID *uuid.UUID
	Topic          string
	EventType      string
	Payload        []byte
	AggregateType  string
	AggregateID    uuid.UUID
	IdempotencyKey *string
	CreatedAt      time.Time
	PublishedAt    *time.Time

	// Publish pipeline (Postgres outbox worker); zero values mean "never attempted / not scheduled".
	PublishAttemptCount  int32
	LastPublishError     *string
	LastPublishAttemptAt *time.Time
	NextPublishAfter     *time.Time
	DeadLetteredAt       *time.Time

	// Status is pending | publishing | published | failed | dead_letter (see migrations).
	Status string
	// LockedBy / LockedUntil are set while a worker holds a publish lease (SKIP LOCKED dispatch).
	LockedBy    *string
	LockedUntil *time.Time

	UpdatedAt          time.Time
	MaxPublishAttempts int32
}

// OutboxPipelineStats aggregates operator-facing health signals for the async outbox.
type OutboxPipelineStats struct {
	PendingTotal           int64
	PendingDueNow          int64
	DeadLetteredTotal      int64
	PublishingLeasedTotal  int64
	OldestPendingCreatedAt *time.Time
	MaxPendingAttempts     int64
	// FailedPendingTotal is unpublished rows in status=failed (after at least one publish attempt).
	FailedPendingTotal int64
}

// OutboxPublishFailureRecord captures one failed publish attempt for persistence.
type OutboxPublishFailureRecord struct {
	EventID          int64
	ErrorMessage     string
	NextPublishAfter *time.Time
	DeadLettered     bool
}
