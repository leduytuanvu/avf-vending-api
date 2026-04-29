package commerceadmin

import (
	"encoding/json"
	"time"

	"github.com/avf/avf-vending-api/internal/app/listscope"
	"github.com/google/uuid"
)

// OrderListItem is a normalized order row for operations dashboards.
type OrderListItem struct {
	OrderID        string    `json:"orderId"`
	OrganizationID string    `json:"organizationId"`
	MachineID      string    `json:"machineId"`
	Status         string    `json:"status"`
	Currency       string    `json:"currency"`
	SubtotalMinor  int64     `json:"subtotalMinor"`
	TaxMinor       int64     `json:"taxMinor"`
	TotalMinor     int64     `json:"totalMinor"`
	IdempotencyKey *string   `json:"idempotencyKey,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// OrdersListResponse is returned by GET /v1/orders.
type OrdersListResponse struct {
	Items []OrderListItem          `json:"items"`
	Meta  listscope.CollectionMeta `json:"meta"`
}

// PaymentListItem is a normalized payment row joined to its parent order context.
type PaymentListItem struct {
	PaymentID            string    `json:"paymentId"`
	OrderID              string    `json:"orderId"`
	OrganizationID       string    `json:"organizationId"`
	MachineID            string    `json:"machineId"`
	Provider             string    `json:"provider"`
	PaymentState         string    `json:"paymentState"`
	OrderStatus          string    `json:"orderStatus"`
	AmountMinor          int64     `json:"amountMinor"`
	Currency             string    `json:"currency"`
	ReconciliationStatus string    `json:"reconciliationStatus"`
	SettlementStatus     string    `json:"settlementStatus"`
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

// PaymentsListResponse is returned by GET /v1/payments.
type PaymentsListResponse struct {
	Items []PaymentListItem        `json:"items"`
	Meta  listscope.CollectionMeta `json:"meta"`
}

type ReconciliationCaseItem struct {
	ID              string     `json:"id"`
	OrganizationID  string     `json:"organizationId"`
	CaseType        string     `json:"caseType"`
	Status          string     `json:"status"`
	Severity        string     `json:"severity"`
	OrderID         *string    `json:"orderId,omitempty"`
	PaymentID       *string    `json:"paymentId,omitempty"`
	VendSessionID   *string    `json:"vendSessionId,omitempty"`
	MachineID       *string    `json:"machineId,omitempty"`
	RefundID        *string    `json:"refundId,omitempty"`
	Provider        *string    `json:"provider,omitempty"`
	ProviderEventID *int64     `json:"providerEventId,omitempty"`
	CorrelationKey  string     `json:"correlationKey,omitempty"`
	Reason          string     `json:"reason"`
	Metadata        []byte     `json:"metadata"`
	FirstDetectedAt time.Time  `json:"firstDetectedAt"`
	LastDetectedAt  time.Time  `json:"lastDetectedAt"`
	ResolvedAt      *time.Time `json:"resolvedAt,omitempty"`
	ResolvedBy      *string    `json:"resolvedBy,omitempty"`
	ResolutionNote  *string    `json:"resolutionNote,omitempty"`
}

type ReconciliationListResponse struct {
	Items []ReconciliationCaseItem `json:"items"`
	Meta  listscope.CollectionMeta `json:"meta"`
}

type ResolveReconciliationInput struct {
	OrganizationID uuid.UUID
	CaseID         uuid.UUID
	ResolvedBy     uuid.UUID
	Status         string
	Note           string
}

// OrderTimelineEventItem is one append-only commerce order lifecycle row.
type OrderTimelineEventItem struct {
	ID         string          `json:"id"`
	EventType  string          `json:"eventType"`
	ActorType  string          `json:"actorType"`
	ActorID    *string         `json:"actorId,omitempty"`
	Payload    json.RawMessage `json:"payload"`
	OccurredAt time.Time       `json:"occurredAt"`
	CreatedAt  time.Time       `json:"createdAt"`
}

// OrderTimelineResponse is GET .../orders/{orderId}/timeline.
type OrderTimelineResponse struct {
	Items []OrderTimelineEventItem `json:"items"`
	Meta  listscope.CollectionMeta `json:"meta"`
}

// RefundRequestItem is the durable refund review record (links to refunds.refunds).
type RefundRequestItem struct {
	ID               string     `json:"id"`
	OrganizationID   string     `json:"organizationId"`
	OrderID          string     `json:"orderId"`
	PaymentID        *string    `json:"paymentId,omitempty"`
	RefundID         *string    `json:"refundId,omitempty"`
	AmountMinor      int64      `json:"amountMinor"`
	Currency         string     `json:"currency"`
	Status           string     `json:"status"`
	Reason           *string    `json:"reason,omitempty"`
	ProviderRefundID *string    `json:"providerRefundId,omitempty"`
	RequestedBy      *string    `json:"requestedBy,omitempty"`
	ApprovedBy       *string    `json:"approvedBy,omitempty"`
	IdempotencyKey   *string    `json:"idempotencyKey,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	CompletedAt      *time.Time `json:"completedAt,omitempty"`
}

// RefundRequestsListResponse is GET .../refunds.
type RefundRequestsListResponse struct {
	Items []RefundRequestItem      `json:"items"`
	Meta  listscope.CollectionMeta `json:"meta"`
}

// CreateOrderRefundInput is POST .../orders/{orderId}/refunds.
type CreateOrderRefundInput struct {
	OrganizationID uuid.UUID
	OrderID        uuid.UUID
	AmountMinor    *int64
	Currency       string
	Reason         string
	RequestedBy    uuid.UUID
	IdempotencyKey string
}

// CreateOrderRefundResult joins refund_requests with the ledger refund projection.
type CreateOrderRefundResult struct {
	RefundRequest     RefundRequestItem `json:"refundRequest"`
	LedgerRefundID    string            `json:"ledgerRefundId"`
	LedgerState       string            `json:"ledgerState"`
	LedgerAmountMinor int64             `json:"ledgerAmountMinor"`
	LedgerCurrency    string            `json:"ledgerCurrency"`
}

// RefundFromReconciliationCaseInput drives POST .../reconciliation/{caseId}/request-refund.
type RefundFromReconciliationCaseInput struct {
	OrganizationID uuid.UUID
	CaseID         uuid.UUID
	AmountMinor    *int64
	Reason         string
	RequestedBy    uuid.UUID
}
