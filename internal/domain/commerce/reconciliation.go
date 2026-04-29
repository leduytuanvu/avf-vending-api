package commerce

import (
	"time"

	"github.com/google/uuid"
)

// ReconciledPaymentTransitionInput is a reconciler-driven terminal transition from created|authorized.
// ProviderHint is persisted on payment_attempts (JSON); Reason is human/diagnostic text for audit.
type ReconciledPaymentTransitionInput struct {
	PaymentID    uuid.UUID
	ToState      string // captured | failed | expired | canceled
	Reason       string
	ProviderHint []byte
	DryRun       bool

	// Optional transactional outbox emission for reconciler-applied terminal
	// transitions. Persistence inserts this in the same DB transaction.
	OutboxTopic          string
	OutboxEventType      string
	OutboxPayload        []byte
	OutboxAggregateType  string
	OutboxAggregateID    uuid.UUID
	OutboxIdempotencyKey string
}

type ReconciliationCaseInput struct {
	OrganizationID  uuid.UUID
	CaseType        string
	Severity        string
	OrderID         *uuid.UUID
	PaymentID       *uuid.UUID
	VendSessionID   *uuid.UUID
	RefundID        *uuid.UUID
	MachineID       *uuid.UUID
	Provider        *string
	ProviderEventID *int64
	CorrelationKey  string
	Reason          string
	Metadata        []byte
}

type ReconciliationCase struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	CaseType        string
	Status          string
	Severity        string
	OrderID         *uuid.UUID
	PaymentID       *uuid.UUID
	VendSessionID   *uuid.UUID
	RefundID        *uuid.UUID
	MachineID       *uuid.UUID
	Provider        *string
	ProviderEventID *int64
	CorrelationKey  string
	Reason          string
	Metadata        []byte
	FirstDetectedAt time.Time
	LastDetectedAt  time.Time
	ResolvedAt      *time.Time
	ResolvedBy      *uuid.UUID
	ResolutionNote  *string
}

type PaidOrderVendStartCandidate struct {
	OrderID        uuid.UUID
	OrganizationID uuid.UUID
	MachineID      uuid.UUID
	PaymentID      uuid.UUID
	Provider       string
	PaymentState   string
	VendSessionID  uuid.UUID
	VendState      string
	UpdatedAt      time.Time
}

type PaidVendFailureCandidate struct {
	OrderID        uuid.UUID
	OrganizationID uuid.UUID
	MachineID      uuid.UUID
	PaymentID      uuid.UUID
	Provider       string
	PaymentState   string
	VendSessionID  uuid.UUID
	VendState      string
	CompletedAt    time.Time
}

type RefundPendingCandidate struct {
	RefundID       uuid.UUID
	PaymentID      uuid.UUID
	OrderID        uuid.UUID
	OrganizationID uuid.UUID
	Provider       string
	RefundState    string
	AmountMinor    int64
	Currency       string
	CreatedAt      time.Time
}
