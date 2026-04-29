package reliability

import (
	"context"
	"time"
)

// OutboxRepository supports the publish pipeline (list unpublished work items).
type OutboxRepository interface {
	ListUnpublished(ctx context.Context, limit int32) ([]OutboxEvent, error)
	// LeaseOutboxForPublish claims up to limit rows for this worker (SKIP LOCKED) and sets status=publishing.
	LeaseOutboxForPublish(ctx context.Context, workerID string, lockTTL time.Duration, minAge time.Duration, limit int32) ([]OutboxEvent, error)
	RecordOutboxPublishFailure(ctx context.Context, rec OutboxPublishFailureRecord) error
	GetOutboxPipelineStats(ctx context.Context) (OutboxPipelineStats, error)
}
