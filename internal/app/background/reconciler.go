package background

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/avf/avf-vending-api/internal/app/workfloworch"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	domainreliability "github.com/avf/avf-vending-api/internal/domain/reliability"
	"github.com/avf/avf-vending-api/internal/platform/observability/productionmetrics"
	platformredis "github.com/avf/avf-vending-api/internal/platform/redis"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

var commerceReconciliationCasesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "avf",
	Subsystem: "commerce",
	Name:      "reconciliation_cases_total",
	Help:      "Commerce payment/vend/refund reconciliation cases detected by background jobs.",
}, []string{"case_type"})

var paymentPaidVendFailedTotal = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: "avf",
	Subsystem: "commerce",
	Name:      "payment_paid_vend_failed_total",
	Help:      "Paid payments attached to vend sessions that failed or did not complete.",
})

var refundPendingTooLongTotal = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: "avf",
	Subsystem: "commerce",
	Name:      "refund_pending_too_long_total",
	Help:      "Refund review cases where a refund remained pending longer than the reconciler threshold.",
})

func recordCommerceReconciliationCase(caseType string) {
	caseType = strings.TrimSpace(strings.ToLower(caseType))
	if caseType == "" {
		caseType = "unknown"
	}
	commerceReconciliationCasesTotal.WithLabelValues(caseType).Inc()
	switch caseType {
	case "payment_paid_vend_failed", "payment_paid_vend_not_started":
		paymentPaidVendFailedTotal.Inc()
	case "refund_pending_too_long":
		refundPendingTooLongTotal.Inc()
	}
}

func reconciliationCaseMetadata(v map[string]any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(`{}`)
	}
	return b
}

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
	CaseWriter            domaincommerce.ReconciliationCaseWriter
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

	// PaymentOutboxTopic / AggregateType enable same-transaction outbox emission
	// for reconciler-applied payment terminal transitions.
	PaymentOutboxTopic         string
	PaymentOutboxAggregateType string

	// StableAge avoids selecting rows still being mutated in the same request lifecycle.
	StableAge time.Duration
	Limits    int32

	// CycleTimeout caps one pass (queries + per-row side effects). Zero uses EffectivePeriodicCycleTimeout(tick).
	// Duplicate/overlap protection: each job runs in a single goroutine; ticks do not overlap, but if a pass
	// exceeds the tick interval the next tick is deferred until the current pass returns (Go ticker behavior).
	CycleTimeout time.Duration

	// ObservabilityPool optional Postgres pool used for reconciliation_cases_open_total gauge snapshots.
	ObservabilityPool *pgxpool.Pool

	UnresolvedOrdersTick           time.Duration
	PaymentProbeTick               time.Duration
	VendStuckTick                  time.Duration
	DuplicatePaymentTick           time.Duration
	RefundReviewTick               time.Duration
	ScheduleRefundOrchestration    bool
	ScheduleManualReviewEscalation bool

	// DistributedLocker optional Redis lock so multi-replica reconcilers serialize commerce reconciliation ticks.
	DistributedLocker platformredis.Locker
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
	if deps.CaseWriter != nil {
		paidNoVend, err := deps.Reader.ListPaidOrdersWithoutVendStart(ctx, before, lim)
		if err != nil {
			return err
		}
		for _, row := range paidNoVend {
			orderID, paymentID, vendID := row.OrderID, row.PaymentID, row.VendSessionID
			mid := row.MachineID
			provider := strings.TrimSpace(row.Provider)
			_, cerr := deps.CaseWriter.UpsertReconciliationCase(ctx, domaincommerce.ReconciliationCaseInput{
				OrganizationID: row.OrganizationID,
				CaseType:       "payment_paid_vend_not_started",
				Severity:       "critical",
				OrderID:        &orderID,
				PaymentID:      &paymentID,
				VendSessionID:  &vendID,
				MachineID:      &mid,
				Provider:       &provider,
				Reason:         "captured payment has an order still paid while vend session is pending",
				Metadata: reconciliationCaseMetadata(map[string]any{
					"payment_state": row.PaymentState,
					"vend_state":    row.VendState,
					"machine_id":    row.MachineID.String(),
				}),
			})
			if cerr != nil {
				return cerr
			}
			recordCommerceReconciliationCase("payment_paid_vend_not_started")
		}
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
		productionmetrics.SetPaymentProviderProbeStalePendingQueue(0)
		return err
	}
	productionmetrics.SetPaymentProviderProbeStalePendingQueue(len(payments))
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
			PaymentID:            p.ID,
			ToState:              toState,
			Reason:               fmt.Sprintf("provider_probe:%s", strings.TrimSpace(snap.NormalizedState)),
			ProviderHint:         snap.ProviderHint,
			DryRun:               deps.DryRun,
			OutboxTopic:          strings.TrimSpace(deps.PaymentOutboxTopic),
			OutboxEventType:      reconciledPaymentOutboxEventType(toState),
			OutboxPayload:        reconciledPaymentOutboxPayload(p, toState, snap),
			OutboxAggregateType:  strings.TrimSpace(deps.PaymentOutboxAggregateType),
			OutboxAggregateID:    p.ID,
			OutboxIdempotencyKey: reconciledPaymentOutboxIdempotencyKey(p.ID, toState),
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

func reconciledPaymentOutboxEventType(toState string) string {
	switch strings.TrimSpace(strings.ToLower(toState)) {
	case "captured":
		return domainreliability.OutboxEventPaymentConfirmed
	case "failed":
		return domainreliability.OutboxEventPaymentFailed
	default:
		return ""
	}
}

func reconciledPaymentOutboxIdempotencyKey(paymentID uuid.UUID, toState string) string {
	eventType := reconciledPaymentOutboxEventType(toState)
	if paymentID == uuid.Nil || eventType == "" {
		return ""
	}
	return "payment_reconcile:" + paymentID.String() + ":" + eventType
}

func reconciledPaymentOutboxPayload(p domaincommerce.Payment, toState string, snap domaincommerce.PaymentStatusSnapshot) []byte {
	b, err := json.Marshal(map[string]any{
		"source":          "reconciler.provider_probe",
		"order_id":        p.OrderID.String(),
		"payment_id":      p.ID.String(),
		"provider":        strings.TrimSpace(p.Provider),
		"payment_state":   strings.TrimSpace(strings.ToLower(toState)),
		"provider_status": strings.TrimSpace(snap.NormalizedState),
	})
	if err != nil {
		return []byte(`{}`)
	}
	return b
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
		if deps.CaseWriter != nil && strings.EqualFold(row.Session.State, "in_progress") {
			orderID, vendID := row.Session.OrderID, row.Session.ID
			mid := row.Session.MachineID
			_, cerr := deps.CaseWriter.UpsertReconciliationCase(ctx, domaincommerce.ReconciliationCaseInput{
				OrganizationID: row.OrganizationID,
				CaseType:       "vend_started_no_terminal_ack",
				Severity:       "warning",
				OrderID:        &orderID,
				VendSessionID:  &vendID,
				MachineID:      &mid,
				Reason:         "vend session remains non-terminal after timeout",
				Metadata: reconciliationCaseMetadata(map[string]any{
					"vend_state":   row.Session.State,
					"order_status": row.OrderStatus,
					"machine_id":   row.Session.MachineID.String(),
				}),
			})
			if cerr != nil && deps.Log != nil {
				deps.Log.Warn("vend_stuck_reconciliation_case_failed", zap.Error(cerr), zap.String("vend_session_id", row.Session.ID.String()))
			}
		}
	}
	if deps.CaseWriter != nil {
		failures, err := deps.Reader.ListPaidVendFailuresForReview(ctx, before, lim)
		if err != nil {
			return err
		}
		for _, row := range failures {
			orderID, paymentID, vendID := row.OrderID, row.PaymentID, row.VendSessionID
			mid := row.MachineID
			provider := strings.TrimSpace(row.Provider)
			if _, err := deps.CaseWriter.UpsertReconciliationCase(ctx, domaincommerce.ReconciliationCaseInput{
				OrganizationID: row.OrganizationID,
				CaseType:       "payment_paid_vend_failed",
				Severity:       "critical",
				OrderID:        &orderID,
				PaymentID:      &paymentID,
				VendSessionID:  &vendID,
				MachineID:      &mid,
				Provider:       &provider,
				Reason:         "captured payment is attached to a failed vend",
				Metadata: reconciliationCaseMetadata(map[string]any{
					"payment_state": row.PaymentState,
					"vend_state":    row.VendState,
					"machine_id":    row.MachineID.String(),
				}),
			}); err != nil {
				return err
			}
			recordCommerceReconciliationCase("payment_paid_vend_failed")
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
		if deps.CaseWriter != nil && deps.OrderRead != nil {
			if o, oerr := deps.OrderRead.GetOrderByID(ctx, p.OrderID); oerr == nil {
				orderID, paymentID := p.OrderID, p.ID
				mid := o.MachineID
				provider := strings.TrimSpace(p.Provider)
				if _, cerr := deps.CaseWriter.UpsertReconciliationCase(ctx, domaincommerce.ReconciliationCaseInput{
					OrganizationID: o.OrganizationID,
					CaseType:       "duplicate_payment",
					Severity:       "critical",
					OrderID:        &orderID,
					PaymentID:      &paymentID,
					MachineID:      &mid,
					Provider:       &provider,
					Reason:         "multiple payments with matching amount/currency exist for the same order",
					Metadata: reconciliationCaseMetadata(map[string]any{
						"payment_state": p.State,
						"amount_minor":  p.AmountMinor,
						"currency":      p.Currency,
					}),
				}); cerr == nil {
					recordCommerceReconciliationCase("duplicate_payment")
				}
			}
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
	if deps.CaseWriter != nil {
		refunds, err := deps.Reader.ListRefundsPendingTooLong(ctx, before, lim)
		if err != nil {
			return err
		}
		for _, row := range refunds {
			orderID, paymentID, refundID := row.OrderID, row.PaymentID, row.RefundID
			provider := strings.TrimSpace(row.Provider)
			var mid *uuid.UUID
			if deps.OrderRead != nil {
				if o, oerr := deps.OrderRead.GetOrderByID(ctx, row.OrderID); oerr == nil {
					m := o.MachineID
					mid = &m
				}
			}
			if _, err := deps.CaseWriter.UpsertReconciliationCase(ctx, domaincommerce.ReconciliationCaseInput{
				OrganizationID: row.OrganizationID,
				CaseType:       "refund_pending_too_long",
				Severity:       "warning",
				OrderID:        &orderID,
				PaymentID:      &paymentID,
				RefundID:       &refundID,
				MachineID:      mid,
				Provider:       &provider,
				Reason:         "refund remains requested/processing after reconciliation timeout",
				Metadata: reconciliationCaseMetadata(map[string]any{
					"refund_state": row.RefundState,
					"amount_minor": row.AmountMinor,
					"currency":     row.Currency,
				}),
			}); err != nil {
				return err
			}
			recordCommerceReconciliationCase("refund_pending_too_long")
		}
	}
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

func reconcilerLockTTL(interval time.Duration) time.Duration {
	if interval <= 0 {
		return 3 * time.Minute
	}
	d := interval * 3
	if d > 15*time.Minute {
		return 15 * time.Minute
	}
	return d
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
		zap.Bool("distributed_redis_locks", deps.DistributedLocker != nil),
		zap.String("note", "list queries use stable_before=now-UTC-stable_age; rows newer than that are intentionally skipped to avoid racing in-flight API writes"),
	)

	var wg sync.WaitGroup
	if deps.ObservabilityPool != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			updateOpenCasesGauge := func(c context.Context) {
				qctx, cancel := context.WithTimeout(c, 5*time.Second)
				defer cancel()
				var n int64
				err := deps.ObservabilityPool.QueryRow(qctx, `SELECT count(*)::bigint FROM commerce_reconciliation_cases WHERE status IN ('open', 'reviewing', 'escalated')`).Scan(&n)
				if err != nil {
					deps.Log.Warn("reconciliation_cases_open_gauge_query_failed", zap.Error(err))
					return
				}
				productionmetrics.SetReconciliationCasesOpen(float64(n))
			}
			updateOpenCasesGauge(ctx)
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					updateOpenCasesGauge(ctx)
				}
			}
		}()
	}
	cto := deps.CycleTimeout
	var cycleHook CycleEndMetricsHook
	if deps.Telemetry != nil {
		tel := deps.Telemetry
		cycleHook = func(job string, duration time.Duration, err error, result string) {
			tel.CycleEnd(job, duration, err, result)
		}
	}
	startTickerGoroutine(&wg, ctx, deps.Log, "unresolved_orders", u, cto, 0, cycleHook, func(c context.Context) error {
		return platformredis.RunExclusive(c, deps.DistributedLocker, "reconciler_unresolved_orders", reconcilerLockTTL(u), func(c context.Context) error {
			return UnresolvedOrdersTick(c, deps)
		})
	})
	startTickerGoroutine(&wg, ctx, deps.Log, "payment_provider_probe", pp, cto, 0, cycleHook, func(c context.Context) error {
		return platformredis.RunExclusive(c, deps.DistributedLocker, "reconciler_payment_provider_probe", reconcilerLockTTL(pp), func(c context.Context) error {
			return PaymentProviderReconcileTick(c, deps)
		})
	})
	startTickerGoroutine(&wg, ctx, deps.Log, "vend_stuck", v, cto, 0, cycleHook, func(c context.Context) error {
		return platformredis.RunExclusive(c, deps.DistributedLocker, "reconciler_vend_stuck", reconcilerLockTTL(v), func(c context.Context) error {
			return VendTimeoutReconcileTick(c, deps)
		})
	})
	startTickerGoroutine(&wg, ctx, deps.Log, "duplicate_payments", d, cto, 0, cycleHook, func(c context.Context) error {
		return platformredis.RunExclusive(c, deps.DistributedLocker, "reconciler_duplicate_payments", reconcilerLockTTL(d), func(c context.Context) error {
			return DuplicatePaymentRecoveryTick(c, deps)
		})
	})
	startTickerGoroutine(&wg, ctx, deps.Log, "refund_review", r, cto, 0, cycleHook, func(c context.Context) error {
		return platformredis.RunExclusive(c, deps.DistributedLocker, "reconciler_refund_review", reconcilerLockTTL(r), func(c context.Context) error {
			return RefundReviewDecisionTick(c, deps)
		})
	})

	<-ctx.Done()
	deps.Log.Info("reconciler_shutdown_wait", zap.String("note", "waiting for in-flight job cycles to finish (bounded by per-cycle timeout)"))
	wg.Wait()
	deps.Log.Info("reconciler_shutdown_complete")
	return ctx.Err()
}
