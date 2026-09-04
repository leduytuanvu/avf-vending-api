package commerce

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// CreateOrderVendInput captures the transactional create of an order plus its first vend session.
type CreateOrderVendInput struct {
	MachineID      uuid.UUID
	ProductID      uuid.UUID
	SlotIndex      int32
	Currency       string
	SubtotalMinor  int64
	TaxMinor       int64
	TotalMinor     int64
	IdempotencyKey string
	OrderStatus    string
	VendState      string
	// Simulation metadata (additive; never counted as real revenue when simulated=true).
	Simulated          bool
	SimulationRunID    string
	SimulationScenario string
	FakeBill           bool
	FakeBoard          bool
	PricingSource            string
	MachinePricingRevision   *int64
	MachinePricingSnapshot   []byte
}

// CreateOrderVendResult is the outcome of CreateOrderWithVendSession.
type CreateOrderVendResult struct {
	Order  Order
	Vend   VendSession
	Replay bool
}

// OrderVendWorkflow coordinates order + vend_session persistence in a single database transaction.
type OrderVendWorkflow interface {
	CreateOrderWithVendSession(ctx context.Context, in CreateOrderVendInput) (CreateOrderVendResult, error)
	// TryReplayCreateOrderWithVend returns the existing order and its first vend session when the idempotency key was already used for the machine.
	TryReplayCreateOrderWithVend(ctx context.Context, machineID uuid.UUID, idempotencyKey string) (CreateOrderVendResult, bool, error)
}

// PaymentOutboxInput captures payment + outbox emission atomically.
type PaymentOutboxInput struct {
	OrderID              uuid.UUID
	Provider             string
	PaymentState         string
	AmountMinor          int64
	Currency             string
	IdempotencyKey       string
	OutboxTopic          string
	OutboxEventType      string
	OutboxPayload        []byte
	OutboxAggregateType  string
	OutboxAggregateID    uuid.UUID
	OutboxIdempotencyKey string
	Simulated            bool
	SimulationRunID      string
	SimulationScenario   string
	FakeBill             bool
	FakeBoard            bool
}

// PaymentOutboxResult is the outcome of CreatePaymentWithOutbox.
type PaymentOutboxResult struct {
	Payment Payment
	Outbox  OutboxEvent
	Replay  bool
}

// PaymentOutboxWorkflow coordinates payment + outbox persistence in a single database transaction.
type PaymentOutboxWorkflow interface {
	CreatePaymentWithOutbox(ctx context.Context, in PaymentOutboxInput) (PaymentOutboxResult, error)
}

// CommandLedgerSummary is persisted command ledger metadata for background scans.
type CommandLedgerSummary struct {
	ID          uuid.UUID
	MachineID   uuid.UUID
	Sequence    int64
	CommandType string
	CreatedAt   time.Time
}

// PaymentProviderLookup identifies a payment for PSP-side status reconciliation.
type PaymentProviderLookup struct {
	Provider            string
	PaymentID           uuid.UUID
	OrderID             uuid.UUID
	ProviderReference   string // PSP order/ref code when known (payment_attempt.provider_reference)
	AmountMinor         int64  // optional; required by some PSPs (e.g. ShopeePay check)
	MachineExternalCode string // machine_code for AVF/TFO tenant selection (MoMo)
}

// PaymentStatusSnapshot is a provider-normalized payment state hint (adapters interpret ProviderHint).
type PaymentStatusSnapshot struct {
	NormalizedState string
	ProviderHint    []byte
}

// PaymentProviderGateway is implemented outside this module (Stripe, Adyen, etc.).
type PaymentProviderGateway interface {
	FetchPaymentStatus(ctx context.Context, lookup PaymentProviderLookup) (PaymentStatusSnapshot, error)
}

// RefundReviewTicket hands off captured-money / failed-fulfillment cases for operator or PSP workflows.
type RefundReviewTicket struct {
	OrderID   uuid.UUID
	PaymentID uuid.UUID
	Reason    string
}

// RefundReviewSink receives tickets from reconciler policy (queue, helpdesk, manual runbooks).
type RefundReviewSink interface {
	EnqueueRefundReview(ctx context.Context, ticket RefundReviewTicket) error
}

// OutboxPublisher delivers durable outbox payloads to external transports.
type OutboxPublisher interface {
	Publish(ctx context.Context, event OutboxEvent) error
}

// VendReconciliationCandidate is a stuck vend row plus the parent order status for policy decisions.
type VendReconciliationCandidate struct {
	Session     VendSession
	OrderStatus string
}

// ReconciliationReader lists reconciliation candidates from Postgres (no provider I/O).
type ReconciliationReader interface {
	ListPaymentsPendingTimeout(ctx context.Context, before time.Time, limit int32) ([]Payment, error)
	ListOrdersWithUnresolvedPayment(ctx context.Context, before time.Time, limit int32) ([]Order, error)
	ListVendSessionsStuckForReconciliation(ctx context.Context, before time.Time, limit int32) ([]VendReconciliationCandidate, error)
	ListPotentialDuplicatePayments(ctx context.Context, before time.Time, limit int32) ([]Payment, error)
	ListPaymentsForRefundReview(ctx context.Context, before time.Time, limit int32) ([]Payment, error)
	ListStaleCommandLedgerEntries(ctx context.Context, before time.Time, limit int32) ([]CommandLedgerSummary, error)
	ListPaidOrdersWithoutVendStart(ctx context.Context, before time.Time, limit int32) ([]PaidOrderVendStartCandidate, error)
	ListPaidVendFailuresForReview(ctx context.Context, before time.Time, limit int32) ([]PaidVendFailureCandidate, error)
	ListRefundsPendingTooLong(ctx context.Context, before time.Time, limit int32) ([]RefundPendingCandidate, error)
}

type ReconciliationCaseWriter interface {
	UpsertReconciliationCase(ctx context.Context, in ReconciliationCaseInput) (ReconciliationCase, error)
}

// OutboxMarkPublishedWriter marks outbox rows published after transport acknowledgement.
// marked is false when no row was updated (already published or unknown id). That is expected
// after a duplicate mark following a successful JetStream publish (dedupe via Nats-Msg-Id) or
// concurrent worker completion.
type OutboxMarkPublishedWriter interface {
	MarkOutboxPublished(ctx context.Context, outboxID int64) (marked bool, err error)
}
