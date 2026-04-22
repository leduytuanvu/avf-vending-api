package bootstrap

import (
	"context"
	"fmt"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/workfloworch"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	platformnats "github.com/avf/avf-vending-api/internal/platform/nats"
	platformrefunds "github.com/avf/avf-vending-api/internal/platform/refunds"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// BuildTemporalWorkerActivityDeps wires workflow activities to existing stores/sinks.
func BuildTemporalWorkerActivityDeps(ctx context.Context, cfg *config.Config, pool *pgxpool.Pool, log *zap.Logger) (workfloworch.ActivityDeps, func(), error) {
	if cfg == nil || pool == nil {
		return workfloworch.ActivityDeps{}, func() {}, fmt.Errorf("bootstrap: nil config or pool")
	}
	if log == nil {
		log = zap.NewNop()
	}
	natsURL := strings.TrimSpace(cfg.NATS.URL)
	if natsURL == "" {
		return workfloworch.ActivityDeps{}, func() {}, fmt.Errorf("bootstrap: %s is required for Temporal workflow review/refund activities", platformnats.EnvNATSURL)
	}
	nc, err := platformnats.ConnectJetStream(ctx, natsURL, "avf-temporal-worker-refund")
	if err != nil {
		return workfloworch.ActivityDeps{}, func() {}, fmt.Errorf("bootstrap: nats for temporal worker: %w", err)
	}
	sink, err := platformrefunds.NewNATSCoreRefundReviewSink(nc.Conn, cfg.Reconciler.RefundReviewSubject)
	if err != nil {
		_ = nc.Conn.Drain()
		return workfloworch.ActivityDeps{}, func() {}, fmt.Errorf("bootstrap: temporal worker refund sink: %w", err)
	}
	return workfloworch.ActivityDeps{
			Lifecycle:  postgres.NewStore(pool),
			RefundSink: sink,
		}, func() {
			if nc != nil && nc.Conn != nil {
				_ = nc.Conn.Drain()
			}
		}, nil
}
