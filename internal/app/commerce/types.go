package commerce

import (
	"time"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/google/uuid"
)

// CreateOrderInput is the checkout surface for provisioning an order and its first vend session.
type CreateOrderInput struct {
	OrganizationID uuid.UUID
	MachineID      uuid.UUID
	ProductID      uuid.UUID
	SlotID         *uuid.UUID
	CabinetCode    string
	SlotCode       string
	// SlotIndex is deprecated; prefer SlotID or CabinetCode+SlotCode.
	SlotIndex *int32
	Currency  string
	// SubtotalMinor, TaxMinor, TotalMinor are deprecated and must be zero (pricing is server-authoritative).
	SubtotalMinor  int64
	TaxMinor       int64
	TotalMinor     int64
	IdempotencyKey string
}

// StartPaymentInput binds a payment row and optional outbox fan-out; provider is an opaque label from the caller.
type StartPaymentInput struct {
	OrganizationID uuid.UUID
	OrderID        uuid.UUID
	Provider       string
	PaymentState   string
	AmountMinor    int64
	Currency       string
	IdempotencyKey string

	OutboxTopic          string
	OutboxEventType      string
	OutboxPayload        []byte
	OutboxAggregateType  string
	OutboxAggregateID    uuid.UUID
	OutboxIdempotencyKey string
}

// AdvanceVendInput requests a vend_session state change for one slot on an order.
type AdvanceVendInput struct {
	OrganizationID uuid.UUID
	OrderID        uuid.UUID
	SlotIndex      int32
	ToState        string
	FailureReason  *string
}

// FinalizeAfterVendInput applies a terminal vend outcome and reconciles order status with payment reality.
type FinalizeAfterVendInput struct {
	OrganizationID    uuid.UUID
	OrderID           uuid.UUID
	SlotIndex         int32
	TerminalVendState string
	FailureReason     *string
	// InventoryDedupeKey optionally overrides computed inventory suppression for successful dispense.
	InventoryDedupeKey string
	// ClientWriteIdempotencyKey is the mutating client's idempotency (HTTP Idempotency-Key or gRPC mutation idempotency).
	// When InventoryDedupeKey is empty on success paths, Fulfill derives "{ClientWriteIdempotencyKey}:vend_sale_inventory"
	// for legacy parity with `/v1/device/.../vend-results` and machine gRPC.
	ClientWriteIdempotencyKey string
	CorrelationID             *uuid.UUID
}

// FulfillSuccessfulVendInput binds one atomic DB transaction that completes the order after a successful vend and applies deduplicated inventory.
type FulfillSuccessfulVendInput struct {
	OrganizationID     uuid.UUID
	OrderID            uuid.UUID
	SlotIndex          int32
	InventoryDedupeKey string
	CorrelationID      *uuid.UUID
}

// FulfillSuccessfulVendResult is the outcome of FulfillSuccessfulVendAtomically.
type FulfillSuccessfulVendResult struct {
	Order           domaincommerce.Order
	Vend            domaincommerce.VendSession
	InventoryReplay bool
	OrderVendReplay bool
}

// FulfillFailedVendInput binds one atomic DB transaction that records a failed vend alongside a failed order.
type FulfillFailedVendInput struct {
	OrganizationID uuid.UUID
	OrderID        uuid.UUID
	SlotIndex      int32
	FailureReason  *string
}

// FulfillFailedVendResult is the outcome of FulfillFailedVendAtomically.
type FulfillFailedVendResult struct {
	Order  domaincommerce.Order
	Vend   domaincommerce.VendSession
	Replay bool
}

// UpdateVendSessionParams is passed to persistence for partial vend updates.
type UpdateVendSessionParams struct {
	OrderID       uuid.UUID
	SlotIndex     int32
	ToState       string
	FailureReason *string
}

// InsertPaymentAttemptParams records a provider-agnostic attempt row for a payment aggregate.
type InsertPaymentAttemptParams struct {
	PaymentID         uuid.UUID
	State             string
	ProviderReference *string
	Payload           []byte
}

// PaymentAttemptView is a minimal read model for a payment_attempt row.
type PaymentAttemptView struct {
	ID        uuid.UUID
	PaymentID uuid.UUID
	State     string
	CreatedAt time.Time
}

// FinalizeOutcome captures persisted order and vend rows after a terminal vend step.
type FinalizeOutcome struct {
	Order domaincommerce.Order
	Vend  domaincommerce.VendSession
	// OrderVendReplay is true when the order and vend were already terminal (success or failed replay) before substantive mutation.
	OrderVendReplay bool
	// InventoryReplay is true when inventory decrement was skipped because the idempotency key already applied (success path).
	InventoryReplay bool
}

// RefundEligibilityAssessment is an advisory decision for operator or payment-adapter follow-up.
type RefundEligibilityAssessment struct {
	Eligible     bool
	Reason       string
	PaymentState string
	VendState    string
}

// InsertRefundRowInput persists a refund aggregate row.
type InsertRefundRowInput struct {
	PaymentID      uuid.UUID
	OrderID        uuid.UUID
	AmountMinor    int64
	Currency       string
	State          string
	Reason         string
	IdempotencyKey string
	Metadata       []byte
}

// CreateRefundInput requests a new refund against the latest captured payment.
type CreateRefundInput struct {
	OrganizationID uuid.UUID
	OrderID        uuid.UUID
	AmountMinor    int64
	Currency       string
	Reason         string
	IdempotencyKey string
	Metadata       []byte
}

// RefundRowView is a minimal refunds table projection.
type RefundRowView struct {
	ID             uuid.UUID
	PaymentID      uuid.UUID
	OrderID        uuid.UUID
	AmountMinor    int64
	Currency       string
	State          string
	Reason         *string
	IdempotencyKey *string
	Metadata       []byte
	CreatedAt      time.Time
}

// CreateOrderResult is the transactional create outcome plus resolved sale line metadata for clients.
type CreateOrderResult struct {
	domaincommerce.CreateOrderVendResult
	SaleLine ResolvedSaleLine
}
