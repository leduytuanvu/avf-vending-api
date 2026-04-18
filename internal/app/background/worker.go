package background

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/avf/avf-vending-api/internal/app/background/outboxmetrics"
	appreliability "github.com/avf/avf-vending-api/internal/app/reliability"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	domainreliability "github.com/avf/avf-vending-api/internal/domain/reliability"
	"go.uber.org/zap"
)

// OutboxDeadLetterPublisher emits one terminal copy per quarantined outbox row (e.g. JetStream DLQ)
// after Postgres sets dead_lettered_at. Optional; transport failures are logged only.
type OutboxDeadLetterPublisher interface {
	PublishOutboxDeadLettered(ctx context.Context, ev domaincommerce.OutboxEvent, lastPublishError string) error
}

// WorkerDeps wires worker jobs to Postgres-backed repositories and optional outbox transport.
type WorkerDeps struct {
	Log *zap.Logger

	Reliability *appreliability.Service
	Policy      appreliability.RecoveryPolicy
	Limits      appreliability.ScanLimits

	OutboxList domainreliability.OutboxRepository
	OutboxMark domaincommerce.OutboxMarkPublishedWriter
	OutboxPub  domaincommerce.OutboxPublisher
	// OutboxDeadLetter publishes to external messaging after Postgres quarantine (JetStream DLQ when wired).
	OutboxDeadLetter OutboxDeadLetterPublisher

	// CycleTimeout caps one pass of a periodic job. Zero uses EffectivePeriodicCycleTimeout(tick).
	CycleTimeout time.Duration

	OutboxTick         time.Duration
	PaymentTimeoutTick time.Duration
	StuckCommandTick   time.Duration
	// RetentionTick schedules telemetry / projection retention when > 0 (requires TelemetryRetention).
	RetentionTick time.Duration
	// TelemetryRetention prunes machine_state_transitions, incidents, rollups, diagnostic manifests (not financial OLTP).
	TelemetryRetention func(ctx context.Context) error

	// MQTTCommandAckTimeouts optionally marks MQTT dispatch attempts as ack_timeout once ack_deadline_at passes.
	MQTTCommandAckTimeouts func(ctx context.Context, before time.Time) error

	// OnOutboxPublishedMirror optional cold-path hook after Postgres marks a row published (non-blocking;
	// used for ClickHouse analytics mirror). Must never block or fail the outbox dispatch tick.
	OnOutboxPublishedMirror func(ev domaincommerce.OutboxEvent)
}

// DefaultWorkerTickSchedule returns conservative polling defaults for non-API processes.
// The retention duration is always 0: no retention ticker runs until a real pruning job exists (see RunWorker).
func DefaultWorkerTickSchedule() (outbox, payment, command, retention time.Duration) {
	return 3 * time.Second, 10 * time.Second, 15 * time.Second, 0
}

func commerceOutboxFromReliability(ev domainreliability.OutboxEvent) domaincommerce.OutboxEvent {
	return domaincommerce.OutboxEvent{
		ID:                   ev.ID,
		OrganizationID:       ev.OrganizationID,
		Topic:                ev.Topic,
		EventType:            ev.EventType,
		Payload:              ev.Payload,
		AggregateType:        ev.AggregateType,
		AggregateID:          ev.AggregateID,
		IdempotencyKey:       ev.IdempotencyKey,
		CreatedAt:            ev.CreatedAt,
		PublishedAt:          ev.PublishedAt,
		PublishAttemptCount:  ev.PublishAttemptCount,
		LastPublishError:     ev.LastPublishError,
		LastPublishAttemptAt: ev.LastPublishAttemptAt,
		NextPublishAfter:     ev.NextPublishAfter,
		DeadLetteredAt:       ev.DeadLetteredAt,
	}
}

// OutboxDispatchTick plans unpublished rows, publishes through the configured gateway, then marks published.
//
// Ordering invariant: JetStream publish happens before Postgres marks published_at. If the process crashes
// after a successful publish and before Mark, the row stays unpublished and a later tick retries publish;
// JetStream deduplication (Nats-Msg-Id / outbox id) limits duplicate side effects on consumers.
//
// Ops: zap fields outbox_pending_*, worker_job_summary job=outbox_dispatch, outbox publish failed —
// see ops/RUNBOOK.md (stuck outbox, broker failures).
func OutboxDispatchTick(ctx context.Context, deps WorkerDeps) error {
	if deps.Reliability == nil || deps.OutboxList == nil || deps.OutboxMark == nil {
		return nil
	}
	run := appreliability.ScanRunContext{Now: time.Now().UTC()}
	plan, err := deps.Reliability.PlanOutboxRepublishBatch(ctx, run, deps.Policy, deps.Limits)
	if err != nil {
		return err
	}
	outboxmetrics.ObservePipelineGauges(run.Now, plan.Pipeline)
	if deps.Log != nil {
		pl := plan.Pipeline
		fields := []zap.Field{
			zap.Int64("outbox_pending_total", pl.PendingTotal),
			zap.Int64("outbox_pending_due_now", pl.PendingDueNow),
			zap.Int64("outbox_dead_lettered_total", pl.DeadLetteredTotal),
			zap.Int64("outbox_max_pending_attempts", pl.MaxPendingAttempts),
		}
		if pl.OldestPendingCreatedAt != nil {
			fields = append(fields,
				zap.Time("outbox_oldest_pending_created_at", *pl.OldestPendingCreatedAt),
				zap.Duration("outbox_oldest_pending_age", run.Now.Sub(*pl.OldestPendingCreatedAt)),
				zap.Float64("outbox_oldest_pending_age_seconds", run.Now.Sub(*pl.OldestPendingCreatedAt).Seconds()),
			)
		}
		if pl.DeadLetteredTotal > 0 || pl.MaxPendingAttempts >= 6 {
			deps.Log.Warn("outbox_pipeline_unhealthy", fields...)
		} else {
			deps.Log.Debug("outbox_pipeline_snapshot", fields...)
		}
	}
	var (
		decisionsTotal     = len(plan.Decisions)
		eligibleRepublish  int
		skippedNoPublisher int
		publishFailed      int
		publishedMarked    int
		publishOkMarkNoop  int
	)
	for _, d := range plan.Decisions {
		if !d.ShouldRepublish {
			continue
		}
		eligibleRepublish++
		if deps.OutboxPub == nil {
			skippedNoPublisher++
			if deps.Log != nil {
				deps.Log.Debug("outbox row eligible but publisher not configured",
					zap.Int64("outbox_id", d.Event.ID),
					zap.String("topic", d.Event.Topic),
				)
			}
			continue
		}
		ev := commerceOutboxFromReliability(d.Event)
		if err := deps.OutboxPub.Publish(ctx, ev); err != nil {
			publishFailed++
			outboxmetrics.IncDispatchPublishFailed()
			nextAttempt := d.Event.PublishAttemptCount + 1
			dead := appreliability.OutboxWillDeadLetterThisFailure(d.Event.PublishAttemptCount, deps.Policy.OutboxMaxPublishAttempts)
			var nextAfter *time.Time
			if !dead {
				bo := appreliability.OutboxPublishBackoffAfterFailure(nextAttempt, deps.Policy.OutboxPublishBackoffBase, deps.Policy.OutboxPublishBackoffMax)
				t := run.Now.Add(bo)
				nextAfter = &t
			}
			rec := domainreliability.OutboxPublishFailureRecord{
				EventID:          d.Event.ID,
				ErrorMessage:     err.Error(),
				NextPublishAfter: nextAfter,
				DeadLettered:     dead,
			}
			if recErr := deps.OutboxList.RecordOutboxPublishFailure(ctx, rec); recErr != nil {
				return recErr
			}
			if dead {
				outboxmetrics.IncDispatchDeadLettered()
				if deps.OutboxDeadLetter != nil {
					if derr := deps.OutboxDeadLetter.PublishOutboxDeadLettered(ctx, ev, err.Error()); derr != nil {
						outboxmetrics.IncDispatchDLQPublishFailed()
						if deps.Log != nil {
							deps.Log.Error("outbox_dead_letter_dlq_publish_failed",
								zap.Error(derr),
								zap.Int64("outbox_id", d.Event.ID),
								zap.String("topic", d.Event.Topic),
								zap.String("note", "row is quarantined in Postgres; repair DLQ transport or replay manually"),
							)
						}
					} else if deps.Log != nil {
						deps.Log.Warn("outbox_dead_lettered",
							zap.Int64("outbox_id", d.Event.ID),
							zap.String("topic", d.Event.Topic),
							zap.Int32("publish_attempt_count_after", nextAttempt),
							zap.String("note", "postgres quarantine + DLQ copy emitted"),
						)
					}
				} else if deps.Log != nil {
					deps.Log.Warn("outbox_dead_lettered",
						zap.Int64("outbox_id", d.Event.ID),
						zap.String("topic", d.Event.Topic),
						zap.Int32("publish_attempt_count_after", nextAttempt),
						zap.String("note", "postgres quarantine only; no OutboxDeadLetter hook configured"),
					)
				}
			} else if deps.Log != nil {
				deps.Log.Warn("outbox publish failed",
					zap.Error(err),
					zap.Int64("outbox_id", d.Event.ID),
					zap.Int32("publish_attempt_after", nextAttempt),
					zap.Bool("dead_lettered", false),
				)
			}
			continue
		}
		marked, err := deps.OutboxMark.MarkOutboxPublished(ctx, d.Event.ID)
		if err != nil {
			if deps.Log != nil {
				deps.Log.Error("outbox mark published failed after successful publish (retry tick will republish; JetStream dedupe should apply)",
					zap.Error(err),
					zap.Int64("outbox_id", d.Event.ID),
					zap.String("topic", d.Event.Topic),
				)
			}
			return err
		}
		if marked {
			publishedMarked++
			outboxmetrics.IncDispatchPublished()
			lagSec := run.Now.Sub(ev.CreatedAt).Seconds()
			outboxmetrics.ObservePublishSuccessLag(lagSec)
			if deps.Log != nil {
				deps.Log.Info("outbox event published",
					zap.Int64("outbox_id", d.Event.ID),
					zap.String("topic", d.Event.Topic),
					zap.Int32("prior_publish_attempts", d.Event.PublishAttemptCount),
					zap.Float64("outbox_publish_lag_seconds", lagSec),
				)
			}
			if deps.OnOutboxPublishedMirror != nil {
				pubAt := run.Now
				ev.PublishedAt = &pubAt
				deps.OnOutboxPublishedMirror(ev)
			}
		} else {
			publishOkMarkNoop++
			if deps.Log != nil {
				deps.Log.Warn("outbox publish succeeded but mark updated no row (already published or race)",
					zap.Int64("outbox_id", d.Event.ID),
					zap.String("topic", d.Event.Topic),
				)
			}
		}
	}
	if deps.Log != nil {
		atLimit := deps.Limits.MaxItems > 0 && int32(decisionsTotal) >= deps.Limits.MaxItems
		deps.Log.Info("worker_job_summary",
			zap.String("job", "outbox_dispatch"),
			zap.Int("decisions_total", decisionsTotal),
			zap.Int("eligible_republish", eligibleRepublish),
			zap.Int("skipped_no_publisher", skippedNoPublisher),
			zap.Int("publish_failed", publishFailed),
			zap.Int("published_marked", publishedMarked),
			zap.Int("publish_ok_mark_noop", publishOkMarkNoop),
			zap.Int32("batch_limit", deps.Limits.MaxItems),
			zap.Bool("at_batch_limit", atLimit),
		)
		if atLimit {
			deps.Log.Warn("worker_batch_at_limit_may_lag",
				zap.String("job", "outbox_dispatch"),
				zap.Int32("batch_limit", deps.Limits.MaxItems),
				zap.String("note", "more unpublished outbox rows may exist beyond this plan; next tick continues"),
			)
		}
	}
	return nil
}

// PaymentTimeoutScanTick classifies non-terminal payments using recovery policy (no PSP calls).
func PaymentTimeoutScanTick(ctx context.Context, deps WorkerDeps) error {
	if deps.Reliability == nil {
		return nil
	}
	run := appreliability.ScanRunContext{Now: time.Now().UTC()}
	scan, err := deps.Reliability.ScanStuckPayments(ctx, run, deps.Policy, deps.Limits)
	if err != nil {
		return err
	}
	var noop, actionable int
	for _, d := range scan.Decisions {
		if d.Action == appreliability.ActionNoop {
			noop++
			continue
		}
		actionable++
		if deps.Log != nil {
			deps.Log.Info("payment_timeout_scan",
				zap.String("action", string(d.Action)),
				zap.String("reason", d.ReasonCode),
				zap.String("payment_id", d.Candidate.PaymentID.String()),
				zap.String("order_id", d.Candidate.OrderID.String()),
				zap.String("trace", d.TraceNote),
			)
		}
	}
	if deps.Log != nil {
		atLimit := int32(len(scan.Decisions)) >= deps.Limits.MaxItems && len(scan.Decisions) > 0
		deps.Log.Info("worker_job_summary",
			zap.String("job", "payment_timeout_scan"),
			zap.Int("candidates", len(scan.Decisions)),
			zap.Int("noop", noop),
			zap.Int("actionable", actionable),
			zap.Int32("batch_limit", deps.Limits.MaxItems),
			zap.Bool("at_batch_limit", atLimit),
		)
		if atLimit {
			deps.Log.Warn("worker_batch_at_limit_may_lag",
				zap.String("job", "payment_timeout_scan"),
				zap.Int32("batch_limit", deps.Limits.MaxItems),
			)
		}
	}
	return nil
}

// StuckCommandScanTick surfaces stale command ledger rows (publish success is not device action success).
func StuckCommandScanTick(ctx context.Context, deps WorkerDeps) error {
	if deps.MQTTCommandAckTimeouts != nil {
		if err := deps.MQTTCommandAckTimeouts(ctx, time.Now()); err != nil {
			return err
		}
	}
	if deps.Reliability == nil {
		return nil
	}
	run := appreliability.ScanRunContext{Now: time.Now().UTC()}
	scan, err := deps.Reliability.ScanStuckCommands(ctx, run, deps.Policy, deps.Limits)
	if err != nil {
		return err
	}
	var noop, actionable int
	for _, d := range scan.Decisions {
		if d.Action == appreliability.ActionNoop {
			noop++
			continue
		}
		actionable++
		if deps.Log != nil {
			deps.Log.Info("stuck_command_scan",
				zap.String("action", string(d.Action)),
				zap.String("reason", d.ReasonCode),
				zap.String("command_id", d.Candidate.CommandID.String()),
				zap.String("machine_id", d.Candidate.MachineID.String()),
				zap.Int64("sequence", d.Candidate.Sequence),
				zap.String("trace", d.TraceNote),
			)
		}
	}
	if deps.Log != nil {
		atLimit := int32(len(scan.Decisions)) >= deps.Limits.MaxItems && len(scan.Decisions) > 0
		deps.Log.Info("worker_job_summary",
			zap.String("job", "stuck_command_scan"),
			zap.Int("candidates", len(scan.Decisions)),
			zap.Int("noop", noop),
			zap.Int("actionable", actionable),
			zap.Int32("batch_limit", deps.Limits.MaxItems),
			zap.Bool("at_batch_limit", atLimit),
		)
		if atLimit {
			deps.Log.Warn("worker_batch_at_limit_may_lag",
				zap.String("job", "stuck_command_scan"),
				zap.Int32("batch_limit", deps.Limits.MaxItems),
			)
		}
	}
	return nil
}

// RunWorker starts explicit ticker-driven jobs and blocks until ctx is cancelled.
func RunWorker(ctx context.Context, deps WorkerDeps) error {
	if deps.Log == nil {
		deps.Log = zap.NewNop()
	}
	ob, pay, cmd, ret := deps.OutboxTick, deps.PaymentTimeoutTick, deps.StuckCommandTick, deps.RetentionTick
	if ob <= 0 || pay <= 0 || cmd <= 0 || ret <= 0 {
		dob, dpay, dcmd, dret := DefaultWorkerTickSchedule()
		if ob <= 0 {
			ob = dob
		}
		if pay <= 0 {
			pay = dpay
		}
		if cmd <= 0 {
			cmd = dcmd
		}
		if ret <= 0 {
			ret = dret
		}
	}
	if ret > 0 && deps.TelemetryRetention == nil {
		return fmt.Errorf("background: RetentionTick>0 requires WorkerDeps.TelemetryRetention (set both or set RetentionTick<=0)")
	}

	retentionNote := "disabled"
	if ret > 0 {
		retentionNote = ret.String()
	}
	deps.Log.Info("worker_startup",
		zap.Int32("outbox_batch_limit", deps.Limits.MaxItems),
		zap.Duration("tick_outbox_dispatch", ob),
		zap.Duration("tick_payment_timeout_scan", pay),
		zap.Duration("tick_stuck_command_scan", cmd),
		zap.String("tick_retention_hook", retentionNote),
		zap.Duration("example_cycle_timeout_outbox", EffectivePeriodicCycleTimeout(ob, deps.CycleTimeout)),
		zap.Bool("outbox_publisher_configured", deps.OutboxPub != nil),
		zap.String("note", "recovery scans are policy-bounded; outbox uses PlanOutboxRepublishBatch limits"),
	)

	var wg sync.WaitGroup
	cto := deps.CycleTimeout
	startTickerGoroutine(&wg, ctx, deps.Log, "outbox_dispatch", ob, cto, nil, func(c context.Context) error { return OutboxDispatchTick(c, deps) })
	startTickerGoroutine(&wg, ctx, deps.Log, "payment_timeout_scan", pay, cto, nil, func(c context.Context) error { return PaymentTimeoutScanTick(c, deps) })
	startTickerGoroutine(&wg, ctx, deps.Log, "stuck_command_scan", cmd, cto, nil, func(c context.Context) error { return StuckCommandScanTick(c, deps) })
	if ret > 0 && deps.TelemetryRetention != nil {
		tr := deps.TelemetryRetention
		startTickerGoroutine(&wg, ctx, deps.Log, "telemetry_retention", ret, cto, nil, func(c context.Context) error { return tr(c) })
	}

	<-ctx.Done()
	deps.Log.Info("worker_shutdown_wait", zap.String("note", "waiting for in-flight job cycles to finish (bounded by per-cycle timeout)"))
	wg.Wait()

	// One final bounded outbox pass after tickers stop so stranded publishes can mark without racing tick loops.
	drainCtx, cancelDrain := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelDrain()
	if err := OutboxDispatchTick(drainCtx, deps); err != nil && !errors.Is(err, context.Canceled) {
		deps.Log.Warn("worker_shutdown_outbox_drain_error", zap.Error(err))
	} else {
		deps.Log.Info("worker_shutdown_outbox_drain_complete")
	}

	deps.Log.Info("worker_shutdown_complete")
	return ctx.Err()
}
