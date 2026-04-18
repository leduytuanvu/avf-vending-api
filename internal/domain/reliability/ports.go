package reliability

import "context"

// OutboxRepository supports the publish pipeline (list unpublished work items).
type OutboxRepository interface {
	ListUnpublished(ctx context.Context, limit int32) ([]OutboxEvent, error)
	RecordOutboxPublishFailure(ctx context.Context, rec OutboxPublishFailureRecord) error
	GetOutboxPipelineStats(ctx context.Context) (OutboxPipelineStats, error)
}
