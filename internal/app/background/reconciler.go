package background

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/avf/avf-vending-api/internal/app/workfloworch"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// OrderReader loads an order aggregate for refund ticket enrichment.
type OrderReader interface {
	GetOrderByID(ctx context.Context, id uuid.UUID) (domaincommerce.Order, error)
}

// PaymentReconciliationApplier persists reconciler-driven payment terminal transitions (Postgres implementation in modules/postgres).
type PaymentReconciliationApplier interface {
	ApplyReconciledPaymentTransition(ctx context.Context, in domaincommerce.ReconciledPaymentTransitionInput) (domaincommerce.Payment, error)
}

// OrderPaidAfterCapture promotes orders after capture when reconciliation closes payment state.
type OrderPaidAfterCapture interface {
	MarkOrderPaidAfterPaymentCapture(ctx context.Context, organizationID, orderID uuid.UUID) (domaincommerce.Order, error)
}

// ReconcilerTelemetry is optional metrics for reconciler ticks (implemented in internal/observability/reconcilerprom; wired from cmd/reconciler when METRICS_ENABLED).
type ReconcilerTelemetry interface {
	CycleEnd(job string, duration time.Duration, err error, result string)
	JobSummary(job string, selected, failed int, atBatchLimit bool)
}

// ReconcilerDeps wires commerce reconciliation reads to optional PSP and refund sinks.
type ReconcilerDeps struct {
	Log *zap.Logger

	// Telemetry is nil unless the process sets it (cmd/reconciler + METRICS_ENABLED).
	Telemetry ReconcilerTelemetry

	Reader                domaincommerce.ReconciliationReader
	OrderRead             OrderReader
	Gateway               domaincommerce.PaymentProviderGateway
	RefundSink            domaincommerce.RefundReviewSink
	WorkflowOrchestration workfloworch.Boundary

	// ActionsEnabled turns on PSP probes and refund routing (Gateway, RefundSink, PaymentApplier must be wired).
	ActionsEnabled bool
	// DryRun when ActionsEnabled: HTTP probe still runs; payment updates use Apply dry_run; refund and duplicate NATS publishes are skipped.
	DryRun bool

	PaymentApplier PaymentReconciliationApplier
	MarkOrderPaid  OrderPaidAfterCapture

	// StableAge avoids selecting rows still being mutated in the same request lifecycle.
	StableAge time.Duration
	Limits    int32

	// CycleTimeout caps one pass (queries + per-row side effects). Zero uses EffectivePeriodicCycleTimeout(tick).
	// Duplicate/overlap protection: each job runs in a single goroutine; ticks do not overlap, but if a pass
	// exceeds the tick interval the next tick is deferred until the current pass returns (Go ticker behavior).
	CycleTimeout time.Duration

	UnresolvedOrdersTick           time.Duration
	PaymentProbeTick               time.Duration
	VendStuckTick                  time.Duration
	DuplicatePaymentTick           time.Duration
	RefundReviewTick               time.Duration
	ScheduleRefundOrchestration    bool
	ScheduleManualReviewEscalation bool
}

// DefaultReconcilerTickSchedule returns conservative polling defaults.
func DefaultReconcilerTickSchedule() (unresolved, paymentProbe, vend, dup, refund time.Duration) {
	return 20 * time.Second, 30 * time.Second, 20 * time.Second, 5 * time.Minute, 2 * time.Minute
}

func (d ReconcilerDeps) beforeBoundary() time.Time {
	age := d.StableAge
	if age <= 0 {
		age = 2 * time.Minute
	}
	return time.Now().UTC().Add(-age)
}

// UnresolvedOrdersTick lists orders that still have aged non-terminal payments attached.
func UnresolvedOrdersTick(ctx context.Context, deps ReconcilerDeps) error {
	if deps.Reader == nil {
		return nil
	}
	lim := deps.limits()
	before := deps.beforeBoundary()
	orders, err := deps.Reader.ListOrdersWithUnresolvedPayment(ctx, before, lim)
	if err != nil {
		return err
	}
	for _, o := range orders {
		if deps.Log != nil {
			deps.Log.Info("reconcile_unresolved_order_payment",
				zap.String("order_id", o.ID.String()),
				zap.String("org_id", o.OrganizationID.String()),
				zap.String("order_status", o.Status),
			)
		}
	}
	atLimit := int32(len(orders)) >= lim
	if deps.Log != nil {
		deps.Log.Info("reconciler_job_summary",
			zap.String("job", "unresolved_orders"),
			zap.Int("selected", len(orders)),
			zap.Int("failed", 0),
			zap.Bool("at_batch_limit", atLimit),
			zap.Time("stable_before", before),
			zap.Int32("batch_limit", lim),
		)
		if atLimit {
			deps.Log.Warn("reconciler_batch_at_limit_may_lag",
				zap.String("job", "unresolved_orders"),
				zap.Int32("batch_limit", lim),
				zap.String("note", "more rows may exist beyond this batch; next tick continues"),
			)
		}
	}
	if deps.Telemetry != nil {
		deps.Telemetry.JobSummary("unresolved_orders", len(orders), 0, atLimit)
	}
	return nil
}

// PaymentProviderReconcileTick asks the PSP for snapshots on stuck payment rows when a gateway is configured.
func PaymentProviderReconcileTick(ctx context.Context, deps ReconcilerDeps) error {
	if deps.Reader == nil {
		return nil
	}
	if !deps.ActionsEnabled {
		return nil
	}
	if deps.Gateway == nil || deps.PaymentApplier == nil {
		return fmt.Errorf("reconciler: payment_provider_probe requires Gateway and PaymentApplier when RECONCILER_ACTIONS_ENABLED=true")
	}
	lim := deps.limits()
	before := deps.beforeBoundary()
	payments, err := deps.Reader.ListPaymentsPendingTimeout(ctx, before, lim)
	if err != nil {
		return err
	}
	var fetchOK, fetchFail, transitioned, noopClassify int
	for _, p := range payments {
		snap, err := deps.Gateway.FetchPaymentStatus(ctx, domaincommerce.PaymentProviderLookup{
			Provider:  p.Provider,
			PaymentID: p.ID,
			OrderID:   p.OrderID,
		})
		if err != nil {
			fetchFail++
			if deps.Log != nil {
				deps.Log.Warn("payment_provider_fetch_failed",
					zap.Error(err),
					zap.String("payment_id", p.ID.String()),
					zap.String("order_id", p.OrderID.String()),
					zap.String("provider", p.Provider),
				)
			}
			continue
		}
		fetchOK++
		toState, ok := classifyProviderNormalizedState(snap.NormalizedState)
		if !ok {
			noopClassify++
			if deps.Log != nil {
				deps.Log.Info("payment_provider_snapshot_no_transition",
					zap.String("payment_id", p.ID.String()),
					zap.String("order_id", p.OrderID.String()),
					zap.String("normalized_state", snap.NormalizedState),
					zap.Bool("reconciler_dry_run", deps.DryRun),
				)
			}
			continue
		}
		updated, aerr := deps.PaymentApplier.ApplyReconciledPaymentTransition(ctx, domaincommerce.ReconciledPaymentTransitionInput{
			PaymentID:    p.ID,
			ToState:      toState,
			Reason:       fmt.Sprintf("provider_probe:%s", strings.TrimSpace(snap.NormalizedState)),
			ProviderHint: snap.ProviderHint,
			DryRun:       deps.DryRun,
		})
		if aerr != nil {
			fetchFail++
			if deps.Log != nil {
				deps.Log.Warn("payment_reconcile_apply_failed",
					zap.Error(aerr),
					zap.String("payment_id", p.ID.String()),
					zap.String("order_id", p.OrderID.String()),
				)
			}
			continue
		}
		if !deps.DryRun && updated.State == toState && updated.State != p.State {
			transitioned++
		}
		if !deps.DryRun && toState == "captured" && deps.MarkOrderPaid != nil && deps.OrderRead != nil {
			o, oerr := deps.OrderRead.GetOrderByID(ctx, p.OrderID)
			if oerr != nil {
				if deps.Log != nil {
					deps.Log.Warn("reconcile_mark_paid_order_lookup_failed", zap.Error(oerr), zap.String("order_id", p.OrderID.String()))
				}
			} else {
				if _, merr := deps.MarkOrderPaid.MarkOrderPaidAfterPaymentCapture(ctx, o.OrganizationID, p.OrderID); merr != nil && deps.Log != nil {
					deps.Log.Warn("reconcile_mark_order_paid_failed", zap.Error(merr),
						zap.String("order_id", p.OrderID.String()),
						zap.String("payment_id", p.ID.String()),
					)
				}
			}
		}
		if deps.Log != nil {
			deps.Log.Info("payment_provider_reconcile_row",
				zap.String("payment_id", p.ID.String()),
				zap.String("order_id", p.OrderID.String()),
				zap.String("normalized_state", snap.NormalizedState),
				zap.String("applied_state", updated.State),
				zap.Bool("reconciler_dry_run", deps.DryRun),
			)
		}
	}
	atLimit := int32(len(payments)) >= lim
	if deps.Log != nil {
		deps.Log.Info("reconciler_job_summary",
			zap.String("job", "payment_provider_probe"),
			zap.Int("selected", len(payments)),
			zap.Int("processed_ok", fetchOK),
			zap.Int("transitioned", transitioned),
			zap.Int("no_transition_classify", noopClassify),
			zap.Int("failed", fetchFail),
			zap.Bool("at_batch_limit", atLimit),
			zap.Time("stable_before", before),
			zap.Int32("batch_limit", lim),
			zap.Bool("reconciler_dry_run", deps.DryRun),
		)
		if atLimit {
			deps.Log.Warn("reconciler_batch_at_limit_may_lag",
				zap.String("job", "payment_provider_probe"),
				zap.Int32("batch_limit", lim),
				zap.String("note", "more payments may exist beyond this batch; next tick continues"),
			)
		}
		if fetchFail > 0 && fetchFail == len(payments) {
			deps.Log.Warn("reconciler_job_all_rows_failed",
				zap.String("job", "payment_provider_probe"),
				zap.Int("failed", fetchFail),
			)
		}
	}
	if deps.Telemetry != nil {
		deps.Telemetry.JobSummary("payment_provider_probe", len(payments), fetchFail, atLimit)
	}
	return nil
}

// VendTimeoutReconcileTick lists vend sessions that are still non-terminal while parent orders progressed.
func VendTimeoutReconcileTick(ctx context.Context, deps ReconcilerDeps) error {
	if deps.Reader == nil {
		return nil
	}
	lim := deps.limits()
	before := deps.beforeBoundary()
	rows, err := deps.Reader.ListVendSessionsStuckForReconciliation(ctx, before, lim)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if deps.Log != nil {
			deps.Log.Info("reconcile_vend_stuck",
				zap.String("vend_session_id", row.Session.ID.String()),
				zap.String("order_id", row.Session.OrderID.String()),
				zap.String("machine_id", row.Session.MachineID.String()),
				zap.String("vend_state", row.Session.State),
				zap.String("order_status", row.OrderStatus),
			)
		}
	}
	atLimit := int32(len(rows)) >= lim
	if deps.Log != nil {
		deps.Log.Info("reconciler_job_summary",
			zap.String("job", "vend_stuck"),
			zap.Int("selected", len(rows)),
			zap.Int("failed", 0),
			zap.Bool("at_batch_limit", atLimit),
			zap.Time("stable_before", before),
			zap.Int32("batch_limit", lim),
		)
		if atLimit {
			deps.Log.Warn("reconciler_batch_at_limit_may_lag",
				zap.String("job", "vend_stuck"),
				zap.Int32("batch_limit", lim),
				zap.String("note", "more vend rows may exist beyond this batch; next tick continues"),
			)
		}
	}
	if deps.Telemetry != nil {
		deps.Telemetry.JobSummary("vend_stuck", len(rows), 0, atLimit)
	}
	return nil
}

// DuplicatePaymentRecoveryTick surfaces same-order duplicate money rows for operator or PSP duplicate handling.
func DuplicatePaymentRecoveryTick(ctx context.Context, deps ReconcilerDeps) error {
	if deps.Reader == nil {
		return nil
	}
	lim := deps.limits()
	before := deps.beforeBoundary()
	payments, err := deps.Reader.ListPotentialDuplicatePayments(ctx, before, lim)
	if err != nil {
		return err
	}
	var enqueued, enqueueFail, scheduled int
	for _, p := range payments {
		if deps.Log != nil {
			deps.Log.Info("reconcile_potential_duplicate_payment",
				zap.String("payment_id", p.ID.String()),
				zap.String("order_id", p.OrderID.String()),
				zap.String("provider", p.Provider),
				zap.String("state", p.State),
			)
		}
		if deps.ScheduleManualReviewEscalation && !deps.DryRun && deps.OrderRead != nil && deps.WorkflowOrchestration != nil && deps.WorkflowOrchestration.Enabled() {
			o, oerr := deps.OrderRead.GetOrderByID(ctx, p.OrderID)
			if oerr == nil {
				err := deps.WorkflowOrchestration.Start(ctx, workfloworch.StartManualReviewEscalation(workfloworch.ManualReviewEscalationInput{
					OrganizationID: o.OrganizationID,
					OrderID:        p.OrderID,
					PaymentID:      p.ID,
					Reason:         "manual_review:potential_duplicate_payment",
					RequestedAt:    time.Now().UTC(),
				}))
				if err == nil {
					scheduled++
					continue
				}
				if deps.Log != nil {
					deps.Log.Warn("duplicate_payment_workflow_schedule_failed", zap.Error(err), zap.String("payment_id", p.ID.String()))
				}
			}
		}
		if deps.ActionsEnabled && deps.RefundSink != nil && deps.OrderRead != nil {
			if deps.DryRun {
				if deps.Log != nil {
					deps.Log.Info("reconcile_duplicate_dry_run_skip_enqueue",
						zap.String("payment_id", p.ID.String()),
						zap.String("order_id", p.OrderID.String()),
					)
				}
				continue
			}
			o, oerr := deps.OrderRead.GetOrderByID(ctx, p.OrderID)
			if oerr != nil {
				enqueueFail++
				if deps.Log != nil {
					deps.Log.Warn("duplicate_payment_order_lookup_failed", zap.Error(oerr), zap.String("order_id", p.OrderID.String()))
				}
				continue
			}
			ticket := domaincommerce.RefundReviewTicket{
				OrganizationID: o.OrganizationID,
				OrderID:        p.OrderID,
				PaymentID:      p.ID,
				Reason:         "potential_duplicate_payment",
			}
			if err := deps.RefundSink.EnqueueRefundReview(ctx, ticket); err != nil {
				enqueueFail++
				if deps.Log != nil {
					deps.Log.Warn("duplicate_payment_enqueue_failed", zap.Error(err), zap.String("payment_id", p.ID.String()))
				}
				continue
			}
			enqueued++
		}
	}
	atLimit := int32(len(payments)) >= lim
	if deps.Log != nil {
		deps.Log.Info("reconciler_job_summary",
			zap.String("job", "duplicate_payments"),
			zap.Int("selected", len(payments)),
			zap.Int("enqueued", enqueued),
			zap.Int("scheduled_workflows", scheduled),
			zap.Int("failed", enqueueFail),
			zap.Bool("at_batch_limit", atLimit),
			zap.Time("stable_before", before),
			zap.Int32("batch_limit", lim),
			zap.Bool("reconciler_dry_run", deps.DryRun),
		)
		if atLimit {
			deps.Log.Warn("reconciler_batch_at_limit_may_lag",
				zap.String("job", "duplicate_payments"),
				zap.Int32("batch_limit", lim),
				zap.String("note", "more duplicate candidates may exist beyond this batch; next tick continues"),
			)
		}
	}
	if deps.Telemetry != nil {
		deps.Telemetry.JobSummary("duplicate_payments", len(payments), enqueueFail, atLimit)
	}
	return nil
}

// RefundReviewDecisionTick hands captured-on-failed-order payments to RefundReviewSink when configured.
func RefundReviewDecisionTick(ctx context.Context, deps ReconcilerDeps) error {
	if deps.Reader == nil {
		return nil
	}
	if deps.ActionsEnabled {
		if deps.RefundSink == nil || deps.OrderRead == nil {
			return fmt.Errorf("reconciler: refund_review requires RefundSink and OrderRead when RECONCILER_ACTIONS_ENABLED=true")
		}
	}
	lim := deps.limits()
	before := deps.beforeBoundary()
	payments, err := deps.Reader.ListPaymentsForRefundReview(ctx, before, lim)
	if err != nil {
		return err
	}
	var enqueued, skippedSink, orderFail, enqueueFail, scheduled int
	for _, p := range payments {
		if deps.ScheduleRefundOrchestration && !deps.DryRun && deps.OrderRead != nil && deps.WorkflowOrchestration != nil && deps.WorkflowOrchestration.Enabled() {
			o, oerr := deps.OrderRead.GetOrderByID(ctx, p.OrderID)
			if oerr == nil {
				err := deps.WorkflowOrchestration.Start(ctx, workfloworch.StartRefundOrchestration(workfloworch.RefundOrchestrationInput{
					OrganizationID: o.OrganizationID,
					OrderID:        p.OrderID,
					PaymentID:      p.ID,
					Reason:         "captured_payment_failed_order",
					RequestedAt:    time.Now().UTC(),
				}))
				if err == nil {
					scheduled++
					if deps.Log != nil {
						deps.Log.Info("refund_review_workflow_scheduled", zap.String("payment_id", p.ID.String()), zap.String("order_id", p.OrderID.String()))
					}
					continue
				}
				if deps.Log != nil {
					deps.Log.Warn("refund_review_workflow_schedule_failed", zap.Error(err), zap.String("payment_id", p.ID.String()))
				}
			}
		}
		if !deps.ActionsEnabled || deps.RefundSink == nil || deps.OrderRead == nil {
			skippedSink++
			if deps.Log != nil {
				deps.Log.Info("refund_review_candidate",
					zap.String("payment_id", p.ID.String()),
					zap.String("order_id", p.OrderID.String()),
					zap.String("note", "actions disabled or refund_sink/order_reader not configured; ticket not enqueued"),
				)
			}
			continue
		}
		if deps.DryRun {
			if deps.Log != nil {
				deps.Log.Info("refund_review_dry_run_skip_enqueue",
					zap.String("payment_id", p.ID.String()),
					zap.String("order_id", p.OrderID.String()),
				)
			}
			continue
		}
		o, err := deps.OrderRead.GetOrderByID(ctx, p.OrderID)
		if err != nil {
			orderFail++
			if deps.Log != nil {
				deps.Log.Warn("refund_review_order_lookup_failed", zap.Error(err), zap.String("order_id", p.OrderID.String()))
			}
			continue
		}
		ticket := domaincommerce.RefundReviewTicket{
			OrganizationID: o.OrganizationID,
			OrderID:        p.OrderID,
			PaymentID:      p.ID,
			Reason:         "captured_payment_failed_order",
		}
		if err := deps.RefundSink.EnqueueRefundReview(ctx, ticket); err != nil {
			enqueueFail++
			if deps.Log != nil {
				deps.Log.Warn("refund_review_enqueue_failed", zap.Error(err), zap.String("payment_id", p.ID.String()))
			}
			continue
		}
		enqueued++
		if deps.Log != nil {
			deps.Log.Info("refund_review_enqueued", zap.String("payment_id", p.ID.String()), zap.String("order_id", p.OrderID.String()))
		}
	}
	atLimit := int32(len(payments)) >= lim
	failed := orderFail + enqueueFail
	if deps.Log != nil {
		deps.Log.Info("reconciler_job_summary",
			zap.String("job", "refund_review"),
			zap.Int("selected", len(payments)),
			zap.Int("enqueued", enqueued),
			zap.Int("scheduled_workflows", scheduled),
			zap.Int("skipped_sink", skippedSink),
			zap.Int("failed", failed),
			zap.Bool("at_batch_limit", atLimit),
			zap.Time("stable_before", before),
			zap.Int32("batch_limit", lim),
			zap.Bool("reconciler_dry_run", deps.DryRun),
		)
		if atLimit {
			deps.Log.Warn("reconciler_batch_at_limit_may_lag",
				zap.String("job", "refund_review"),
				zap.Int32("batch_limit", lim),
				zap.String("note", "more refund candidates may exist beyond this batch; next tick continues"),
			)
		}
	}
	if deps.Telemetry != nil {
		deps.Telemetry.JobSummary("refund_review", len(payments), failed, atLimit)
	}
	return nil
}

func (d ReconcilerDeps) limits() int32 {
	if d.Limits <= 0 {
		return 100
	}
	return d.Limits
}

// RunReconciler starts five explicit commerce reconciliation tickers and blocks until ctx is cancelled.
//
// Ops: reconciler_job_summary, reconciler_batch_at_limit_may_lag, background_cycle_* per job name.
// When deps.ActionsEnabled is true, cmd/reconciler must wire Gateway, RefundSink, PaymentApplier, and MarkOrderPaid
// (validated at process startup). RECONCILER_DRY_RUN logs reconciler_dry_run=true and skips payment mutations plus refund/duplicate NATS enqueues.
func RunReconciler(ctx context.Context, deps ReconcilerDeps) error {
	if deps.Log == nil {
		deps.Log = zap.NewNop()
	}
	u, pp, v, d, r := deps.UnresolvedOrdersTick, deps.PaymentProbeTick, deps.VendStuckTick, deps.DuplicatePaymentTick, deps.RefundReviewTick
	if u <= 0 || pp <= 0 || v <= 0 || d <= 0 || r <= 0 {
		du, dpp, dv, dd, dr := DefaultReconcilerTickSchedule()
		if u <= 0 {
			u = du
		}
		if pp <= 0 {
			pp = dpp
		}
		if v <= 0 {
			v = dv
		}
		if d <= 0 {
			d = dd
		}
		if r <= 0 {
			r = dr
		}
	}

	stableAge := deps.StableAge
	if stableAge <= 0 {
		stableAge = 2 * time.Minute
	}
	deps.Log.Info("reconciler_startup",
		zap.Duration("stable_age", stableAge),
		zap.Int32("batch_limit", deps.limits()),
		zap.Duration("sample_default_cycle_timeout_unresolved_job", EffectivePeriodicCycleTimeout(u, deps.CycleTimeout)),
		zap.Duration("tick_unresolved_orders", u),
		zap.Duration("tick_payment_provider_probe", pp),
		zap.Duration("tick_vend_stuck", v),
		zap.Duration("tick_duplicate_payments", d),
		zap.Duration("tick_refund_review", r),
		zap.Bool("reader_configured", deps.Reader != nil),
		zap.Bool("payment_gateway_configured", deps.Gateway != nil),
		zap.Bool("refund_pipeline_configured", deps.RefundSink != nil && deps.OrderRead != nil),
		zap.String("note", "list queries use stable_before=now-UTC-stable_age; rows newer than that are intentionally skipped to avoid racing in-flight API writes"),
	)

	var wg sync.WaitGroup
	cto := deps.CycleTimeout
	var cycleHook CycleEndMetricsHook
	if deps.Telemetry != nil {
		tel := deps.Telemetry
		cycleHook = func(job string, duration time.Duration, err error, result string) {
			tel.CycleEnd(job, duration, err, result)
		}
	}
	startTickerGoroutine(&wg, ctx, deps.Log, "unresolved_orders", u, cto, cycleHook, func(c context.Context) error { return UnresolvedOrdersTick(c, deps) })
	startTickerGoroutine(&wg, ctx, deps.Log, "payment_provider_probe", pp, cto, cycleHook, func(c context.Context) error { return PaymentProviderReconcileTick(c, deps) })
	startTickerGoroutine(&wg, ctx, deps.Log, "vend_stuck", v, cto, cycleHook, func(c context.Context) error { return VendTimeoutReconcileTick(c, deps) })
	startTickerGoroutine(&wg, ctx, deps.Log, "duplicate_payments", d, cto, cycleHook, func(c context.Context) error { return DuplicatePaymentRecoveryTick(c, deps) })
	startTickerGoroutine(&wg, ctx, deps.Log, "refund_review", r, cto, cycleHook, func(c context.Context) error { return RefundReviewDecisionTick(c, deps) })

	<-ctx.Done()
	deps.Log.Info("reconciler_shutdown_wait", zap.String("note", "waiting for in-flight job cycles to finish (bounded by per-cycle timeout)"))
	wg.Wait()
	deps.Log.Info("reconciler_shutdown_complete")
	return ctx.Err()
}
