// Package productionmetrics registers canonical Prometheus series for AVF vending production operations (P1.1).
// Metric names match ops dashboards and alerting; avoid registering duplicates elsewhere.
package productionmetrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "HTTP requests completed.",
	}, []string{"method", "route", "status"})
	httpRequestDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds.",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
	}, []string{"method", "route", "status"})
	httpErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_errors_total",
		Help: "HTTP responses with status >= 400.",
	}, []string{"method", "route", "status"})

	// gRPC
	grpcRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_requests_total",
		Help: "gRPC unary requests completed.",
	}, []string{"service", "method", "grpc_code"})
	grpcRequestDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "grpc_request_duration_seconds",
		Help:    "gRPC unary request duration in seconds.",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 18),
	}, []string{"service", "method", "grpc_code"})
	grpcErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_errors_total",
		Help: "gRPC unary requests completed with non-OK status.",
	}, []string{"service", "method", "grpc_code"})
	grpcAuthFailuresTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_auth_failures_total",
		Help: "gRPC requests rejected during authentication or credential validation.",
	}, []string{"reason"})
	grpcIdempotencyReplaysTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "grpc_idempotency_replays_total",
		Help: "Machine mutation idempotent replay responses served from PostgreSQL snapshots.",
	})
	grpcIdempotencyConflictsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "grpc_idempotency_conflicts_total",
		Help: "Machine mutation idempotency key reuse with incompatible payload.",
	})

	// Machine runtime
	machineCheckinsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "machine_checkins_total",
		Help: "Machine check-ins recorded (HTTP or gRPC bootstrap paths).",
	}, []string{"transport"})
	machineLastSeenAgeSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "machine_last_seen_age_seconds",
		Help:    "Observed age of machine last_seen_at relative to now.",
		Buckets: prometheus.ExponentialBuckets(1, 2, 20),
	})
	machineOfflineEventsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "machine_offline_events_total",
		Help: "Offline events persisted for replay.",
	}, []string{"result"})
	machineOfflineReplayFailuresTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "machine_offline_replay_failures_total",
		Help: "Offline event payloads rejected at dispatch (telemetry/commerce/inventory handlers).",
	}, []string{"reason"})
	machineSyncLagSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "machine_sync_lag_seconds",
		Help:    "Wall-clock lag between offline event occurred_at and server processing time.",
		Buckets: prometheus.ExponentialBuckets(0.05, 2, 18),
	})

	// Commerce / payment
	ordersCreatedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "orders_created_total",
		Help: "Orders created through commerce flows.",
	}, []string{"channel"})
	vendSuccessTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "vend_success_total",
		Help: "Terminal vend outcomes recorded as success.",
	})
	vendFailureTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "vend_failure_total",
		Help: "Terminal vend outcomes recorded as failed.",
	})
	paymentWebhooksTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "payment_webhooks_total",
		Help: "Payment provider webhook POSTs handled.",
	}, []string{"result"})
	paymentWebhookRejectionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "payment_webhook_rejections_total",
		Help: "Payment webhooks rejected (HMAC, validation, ordering, replay conflict).",
	}, []string{"reason"})
	paymentWebhookAmountCurrencyMismatchTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "payment_webhook_amount_currency_mismatch_total",
		Help: "Payment webhooks rejected: provider amount/currency does not match persisted payment row (reconciliation case opened).",
	})
	paymentProviderProbeStalePendingQueue = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "payment_provider_probe_stale_pending_queue",
		Help: "Count of payments selected by the reconciler payment_provider_probe as past pending-timeout (last tick snapshot; 0 when none).",
	})
	reconciliationCasesOpenTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "reconciliation_cases_open_total",
		Help: "Current count of commerce reconciliation cases in open status.",
	})
	refundsRequestedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "refunds_requested_total",
		Help: "Refund requests recorded.",
	}, []string{"channel"})
	refundsFailedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "refunds_failed_total",
		Help: "Refund terminal failures or irreversible errors.",
	}, []string{"reason"})

	// MQTT commands
	commandsCreatedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "commands_created_total",
		Help: "Rows inserted into command_ledger for machine commands.",
	})
	commandsDispatchedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "commands_dispatched_total",
		Help: "MQTT command attempts successfully published to the broker.",
	})
	commandsAckedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "commands_acked_total",
		Help: "Device command receipts completing an attempt (acked terminal good path).",
	})
	commandsFailedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "commands_failed_total",
		Help: "Command receipts rejected or terminal failures at persistence.",
	}, []string{"reason"})
	commandsExpiredTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "commands_expired_total",
		Help: "Command attempts expired by ledger timeout policy.",
	})
	commandAckLatencySeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "command_ack_latency_seconds",
		Help:    "Latency from attempt sent_at to ack receipt.",
		Buckets: prometheus.ExponentialBuckets(0.05, 2, 16),
	})
	commandRetryTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "command_retry_total",
		Help: "Command publish retries or dispatch refusal reasons implying retry pressure.",
	}, []string{"reason"})

	// Inventory
	inventoryAdjustmentsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "inventory_adjustments_total",
		Help: "Inventory adjustment batches accepted.",
	}, []string{"source"})
	inventoryNegativeStockAttemptsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "inventory_negative_stock_attempts_total",
		Help: "Attempts that would drive slot quantity negative.",
	})
	inventoryReconciliationCasesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "inventory_reconciliation_cases_total",
		Help: "Inventory-focused reconciliation cases opened.",
	})

	// Outbox / NATS
	outboxPendingTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "outbox_pending_total",
		Help: "Unpublished outbox rows awaiting JetStream publish.",
	})
	outboxPublishedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "outbox_published_total",
		Help: "Successful outbox publishes marking published_at.",
	})
	outboxFailedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "outbox_failed_total",
		Help: "JetStream publish failures recorded by worker dispatch.",
	})
	outboxDLQTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "outbox_dlq_total",
		Help: "Rows dead-lettered in Postgres after exhausting publish attempts.",
	})
	outboxLagSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "outbox_lag_seconds",
		Help:    "Lag from outbox created_at to successful publish.",
		Buckets: prometheus.ExponentialBuckets(0.05, 2, 18),
	})

	// Audit
	auditEventsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "audit_events_total",
		Help: "Enterprise audit_events rows inserted.",
	}, []string{"action"})
	auditWriteFailuresTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "audit_write_failures_total",
		Help: "Failures persisting audit_events.",
	}, []string{"operation"})
)

// RecordHTTPRequest observes HTTP duration and counters for successful routing completion.
func RecordHTTPRequest(method, route, status string, elapsed time.Duration) {
	if route == "" {
		route = "unknown"
	}
	httpRequestsTotal.WithLabelValues(method, route, status).Inc()
	httpRequestDurationSeconds.WithLabelValues(method, route, status).Observe(elapsed.Seconds())
	codeNum, err := strconv.Atoi(status)
	if err == nil && codeNum >= 400 {
		httpErrorsTotal.WithLabelValues(method, route, status).Inc()
	}
}

// RecordGRPCUnary records gRPC unary completion.
func RecordGRPCUnary(service, method, grpcCode string, elapsed time.Duration) {
	grpcRequestsTotal.WithLabelValues(service, method, grpcCode).Inc()
	grpcRequestDurationSeconds.WithLabelValues(service, method, grpcCode).Observe(elapsed.Seconds())
	if grpcCode != "" && grpcCode != "OK" {
		grpcErrorsTotal.WithLabelValues(service, method, grpcCode).Inc()
	}
}

// RecordGRPCAuthFailure increments grpc_auth_failures_total.
func RecordGRPCAuthFailure(reason string) {
	if reason == "" {
		reason = "unknown"
	}
	grpcAuthFailuresTotal.WithLabelValues(reason).Inc()
}

// RecordGRPCIdempotencyReplay increments grpc_idempotency_replays_total.
func RecordGRPCIdempotencyReplay() { grpcIdempotencyReplaysTotal.Inc() }

// RecordGRPCIdempotencyConflict increments grpc_idempotency_conflicts_total.
func RecordGRPCIdempotencyConflict() { grpcIdempotencyConflictsTotal.Inc() }

// RecordMachineCheckIn increments machine_checkins_total.
func RecordMachineCheckIn(transport string) {
	if transport == "" {
		transport = "unknown"
	}
	machineCheckinsTotal.WithLabelValues(transport).Inc()
}

// ObserveMachineLastSeenAge records machine_last_seen_age_seconds.
func ObserveMachineLastSeenAge(d time.Duration) {
	if d < 0 {
		d = 0
	}
	machineLastSeenAgeSeconds.Observe(d.Seconds())
}

// RecordOfflineEventResult records machine_offline_events_total{result}.
func RecordOfflineEventResult(result string) {
	if result == "" {
		result = "unknown"
	}
	machineOfflineEventsTotal.WithLabelValues(result).Inc()
}

// RecordOfflineReplayFailure records machine_offline_replay_failures_total{reason}.
func RecordOfflineReplayFailure(reason string) {
	if reason == "" {
		reason = "unknown"
	}
	machineOfflineReplayFailuresTotal.WithLabelValues(reason).Inc()
}

// ObserveMachineSyncLag records machine_sync_lag_seconds from payload occurred time to now.
func ObserveMachineSyncLag(lag time.Duration) {
	if lag < 0 {
		lag = 0
	}
	machineSyncLagSeconds.Observe(lag.Seconds())
}

// RecordOrderCreated increments orders_created_total{channel}.
func RecordOrderCreated(channel string) {
	if channel == "" {
		channel = "unknown"
	}
	ordersCreatedTotal.WithLabelValues(channel).Inc()
}

// RecordVendSuccess increments vend_success_total.
func RecordVendSuccess() { vendSuccessTotal.Inc() }

// RecordVendFailure increments vend_failure_total.
func RecordVendFailure() { vendFailureTotal.Inc() }

// RecordPaymentWebhook increments payment_webhooks_total{result}.
func RecordPaymentWebhook(result string) {
	if result == "" {
		result = "unknown"
	}
	paymentWebhooksTotal.WithLabelValues(result).Inc()
}

// RecordPaymentWebhookRejection increments payment_webhook_rejections_total{reason}.
func RecordPaymentWebhookRejection(reason string) {
	if reason == "" {
		reason = "unknown"
	}
	paymentWebhookRejectionsTotal.WithLabelValues(reason).Inc()
}

// RecordPaymentWebhookAmountCurrencyMismatch increments payment_webhook_amount_currency_mismatch_total.
func RecordPaymentWebhookAmountCurrencyMismatch() {
	paymentWebhookAmountCurrencyMismatchTotal.Inc()
}

// SetPaymentProviderProbeStalePendingQueue records payment_provider_probe_stale_pending_queue (reconciler last tick).
func SetPaymentProviderProbeStalePendingQueue(n int) {
	if n < 0 {
		n = 0
	}
	paymentProviderProbeStalePendingQueue.Set(float64(n))
}

// SetReconciliationCasesOpen sets reconciliation_cases_open_total gauge.
func SetReconciliationCasesOpen(n float64) {
	if n < 0 {
		n = 0
	}
	reconciliationCasesOpenTotal.Set(n)
}

// RecordRefundRequested increments refunds_requested_total{channel}.
func RecordRefundRequested(channel string) {
	if channel == "" {
		channel = "unknown"
	}
	refundsRequestedTotal.WithLabelValues(channel).Inc()
}

// RecordRefundFailed increments refunds_failed_total{reason}.
func RecordRefundFailed(reason string) {
	if reason == "" {
		reason = "unknown"
	}
	refundsFailedTotal.WithLabelValues(reason).Inc()
}

// RecordCommandCreated increments commands_created_total.
func RecordCommandCreated() { commandsCreatedTotal.Inc() }

// RecordCommandDispatched increments commands_dispatched_total.
func RecordCommandDispatched() { commandsDispatchedTotal.Inc() }

// RecordCommandAcked increments commands_acked_total.
func RecordCommandAcked() { commandsAckedTotal.Inc() }

// RecordCommandFailed increments commands_failed_total{reason}.
func RecordCommandFailed(reason string) {
	if reason == "" {
		reason = "unknown"
	}
	commandsFailedTotal.WithLabelValues(reason).Inc()
}

// AddCommandsFailed adds delta to commands_failed_total{reason} (batch timeouts, etc.).
func AddCommandsFailed(reason string, n int64) {
	if n <= 0 {
		return
	}
	if reason == "" {
		reason = "unknown"
	}
	commandsFailedTotal.WithLabelValues(reason).Add(float64(n))
}

// AddCommandsExpired adds to commands_expired_total.
func AddCommandsExpired(n int64) {
	if n <= 0 {
		return
	}
	commandsExpiredTotal.Add(float64(n))
}

// ObserveCommandAckLatency records command_ack_latency_seconds.
func ObserveCommandAckLatency(d time.Duration) {
	if d < 0 {
		d = 0
	}
	commandAckLatencySeconds.Observe(d.Seconds())
}

// RecordCommandRetry records command_retry_total{reason}.
func RecordCommandRetry(reason string) {
	if reason == "" {
		reason = "unknown"
	}
	commandRetryTotal.WithLabelValues(reason).Inc()
}

// RecordInventoryAdjustment increments inventory_adjustments_total{source}.
func RecordInventoryAdjustment(source string) {
	if source == "" {
		source = "unknown"
	}
	inventoryAdjustmentsTotal.WithLabelValues(source).Inc()
}

// RecordInventoryNegativeStockAttempt increments inventory_negative_stock_attempts_total.
func RecordInventoryNegativeStockAttempt() {
	inventoryNegativeStockAttemptsTotal.Inc()
}

// RecordInventoryReconciliationCase increments inventory_reconciliation_cases_total.
func RecordInventoryReconciliationCase() {
	inventoryReconciliationCasesTotal.Inc()
}

// SetOutboxPending sets outbox_pending_total.
func SetOutboxPending(v float64) {
	if v < 0 {
		v = 0
	}
	outboxPendingTotal.Set(v)
}

// RecordOutboxPublished increments outbox_published_total.
func RecordOutboxPublished() { outboxPublishedTotal.Inc() }

// RecordOutboxFailed increments outbox_failed_total.
func RecordOutboxFailed() { outboxFailedTotal.Inc() }

// RecordOutboxDLQ increments outbox_dlq_total.
func RecordOutboxDLQ() { outboxDLQTotal.Inc() }

// ObserveOutboxLag records outbox_lag_seconds.
func ObserveOutboxLag(seconds float64) { outboxLagSeconds.Observe(seconds) }

// RecordAuditEvent increments audit_events_total{action}.
func RecordAuditEvent(action string) {
	if action == "" {
		action = "unknown"
	}
	auditEventsTotal.WithLabelValues(action).Inc()
}

// RecordAuditWriteFailure increments audit_write_failures_total{operation}.
func RecordAuditWriteFailure(operation string) {
	if operation == "" {
		operation = "unknown"
	}
	auditWriteFailuresTotal.WithLabelValues(operation).Inc()
}
