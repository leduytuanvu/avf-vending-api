package commerceadmin

import (
	"time"

	"github.com/avf/avf-vending-api/internal/app/listscope"
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
