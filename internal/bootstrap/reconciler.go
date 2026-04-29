package bootstrap

import (
	"context"
	"fmt"
	"time"

	appbackground "github.com/avf/avf-vending-api/internal/app/background"
	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	platformnats "github.com/avf/avf-vending-api/internal/platform/nats"
	platformpayments "github.com/avf/avf-vending-api/internal/platform/payments"
	platformredis "github.com/avf/avf-vending-api/internal/platform/redis"
	platformrefunds "github.com/avf/avf-vending-api/internal/platform/refunds"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// BuildReconcilerDeps constructs commerce reconciliation dependencies for cmd/reconciler.
// When cfg.Reconciler.ActionsEnabled, wires a real HTTP payment probe gateway and NATS refund review sink.
func BuildReconcilerDeps(ctx context.Context, cfg *config.Config, pool *pgxpool.Pool, log *zap.Logger) (appbackground.ReconcilerDeps, func(), error) {
	if cfg == nil || pool == nil {
		return appbackground.ReconcilerDeps{}, func() {}, fmt.Errorf("bootstrap: nil config or pool")
	}
	if log == nil {
		log = zap.NewNop()
	}

	commerceRepo := postgres.NewCommerceReconcileRepository(pool)
	store := postgres.NewStore(pool)

	cleanup := func() {}

	lim := cfg.Reconciler.BatchLimit
	if lim <= 0 {
		lim = 200
	}

	deps := appbackground.ReconcilerDeps{
		Log:                        log,
		Reader:                     commerceRepo,
		CaseWriter:                 commerceRepo,
		OrderRead:                  commerceRepo,
		StableAge:                  2 * time.Minute,
		Limits:                     lim,
		UnresolvedOrdersTick:       0,
		PaymentProbeTick:           0,
		VendStuckTick:              0,
		DuplicatePaymentTick:       0,
		RefundReviewTick:           0,
		ActionsEnabled:             cfg.Reconciler.ActionsEnabled,
		DryRun:                     cfg.Reconciler.DryRun,
		PaymentOutboxTopic:         cfg.Commerce.PaymentOutboxTopic,
		PaymentOutboxAggregateType: cfg.Commerce.PaymentOutboxAggregateType,
		ObservabilityPool:          pool,
	}

	u, pp, v, d, r := appbackground.DefaultReconcilerTickSchedule()
	deps.UnresolvedOrdersTick = u
	deps.PaymentProbeTick = pp
	deps.VendStuckTick = v
	deps.DuplicatePaymentTick = d
	deps.RefundReviewTick = r

	rdb, err := platformredis.NewClient(&cfg.Redis)
	if err != nil {
		return appbackground.ReconcilerDeps{}, cleanup, fmt.Errorf("bootstrap: redis client: %w", err)
	}
	if rdb != nil {
		prev := cleanup
		cleanup = func() {
			prev()
			_ = rdb.Close()
		}
		if cfg.RedisRuntime.LocksEnabled {
			deps.DistributedLocker = platformredis.NewRedisLocker(rdb, cfg.Redis.KeyPrefix)
		}
	}

	if !cfg.Reconciler.ActionsEnabled {
		return deps, cleanup, nil
	}

	reg := platformpayments.NewRegistry(cfg)
	gw, err := platformpayments.NewHTTPStatusGateway(cfg.Reconciler.PaymentProbeURLTemplate, cfg.Reconciler.PaymentProbeBearerToken)
	if err != nil {
		return appbackground.ReconcilerDeps{}, cleanup, fmt.Errorf("bootstrap: payment probe gateway: %w", err)
	}
	reg.SetHTTPProbe(gw)
	deps.Gateway = reg.CompositePaymentGateway()

	natsURL := cfg.NATS.URL
	nc, err := platformnats.ConnectJetStream(ctx, natsURL, "avf-reconciler-refund")
	if err != nil {
		return appbackground.ReconcilerDeps{}, cleanup, fmt.Errorf("bootstrap: nats for reconciler: %w", err)
	}
	prev := cleanup
	cleanup = func() {
		prev()
		if nc != nil && nc.Conn != nil {
			_ = nc.Conn.Drain()
		}
	}

	sink, err := platformrefunds.NewNATSCoreRefundReviewSink(nc.Conn, cfg.Reconciler.RefundReviewSubject)
	if err != nil {
		cleanup()
		return appbackground.ReconcilerDeps{}, func() {}, fmt.Errorf("bootstrap: refund review sink: %w", err)
	}
	deps.RefundSink = sink

	commerceSvc := appcommerce.NewService(appcommerce.Deps{
		OrderVend:              store,
		PaymentOutbox:          store,
		Lifecycle:              store,
		WebhookPersist:         store,
		SaleLines:              store,
		PaymentSessionRegistry: reg,
	})
	deps.MarkOrderPaid = commerceSvc
	deps.PaymentApplier = store

	return deps, cleanup, nil
}
