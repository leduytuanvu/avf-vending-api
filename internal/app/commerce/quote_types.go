package commerce

import (
	"time"

	"github.com/google/uuid"
)

// QuoteLineInput is one cart line for server-side quote pricing.
type QuoteLineInput struct {
	ProductID   uuid.UUID
	SlotID      *uuid.UUID
	CabinetCode string
	SlotCode    string
	SlotIndex   *int32
	Quantity    int32
}

// CreateQuoteInput provisions a priced checkout quote for a multi-line cart.
type CreateQuoteInput struct {
	MachineID      uuid.UUID
	Currency       string
	PaymentMethod  string
	Lines          []QuoteLineInput
	IdempotencyKey string
	QuoteTTL       time.Duration
}

// QuoteLineView is one priced line returned to clients.
type QuoteLineView struct {
	LineSequence       int32
	ProductID          uuid.UUID
	SlotConfigID       uuid.UUID
	CabinetCode        string
	SlotCode           string
	SlotIndex          int32
	Quantity           int32
	UnitPriceMinor     int64
	LineSubtotalMinor  int64
	PricingFingerprint string
	PromotionLabel     string
}

// CreateQuoteResult is the quote snapshot for checkout UI.
type CreateQuoteResult struct {
	QuoteID       uuid.UUID
	MachineID     uuid.UUID
	Currency      string
	PaymentMethod string
	SubtotalMinor int64
	DiscountMinor int64
	PayableMinor  int64
	ExpiresAt     time.Time
	Lines         []QuoteLineView
	Replay        bool
}

// CreateOrderFromQuoteInput binds a quote to a new order with N vend sessions.
type CreateOrderFromQuoteInput struct {
	MachineID          uuid.UUID
	QuoteID            uuid.UUID
	PaymentMethod      string
	IdempotencyKey     string
	Simulated          bool
	SimulationRunID    string
	SimulationScenario string
	FakeBill           bool
	FakeBoard          bool
}

// OrderVendLineView is one vend session row created from a quote.
type OrderVendLineView struct {
	VendSessionID uuid.UUID
	LineSequence  int32
	SlotIndex     int32
	ProductID     uuid.UUID
	CabinetCode   string
	SlotCode      string
	VendState     string
}

// CreateOrderFromQuoteResult is the multi-line order create outcome.
type CreateOrderFromQuoteResult struct {
	OrderID       uuid.UUID
	OrderStatus   string
	Currency      string
	SubtotalMinor int64
	TaxMinor      int64
	TotalMinor    int64
	Lines         []OrderVendLineView
	Replay        bool
}
