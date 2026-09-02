package commerce

import (
	"context"
	"errors"
	"strings"
	"time"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/google/uuid"
)

// FinancialCorrectnessStore persists winner arbitration and cash evidence.
type FinancialCorrectnessStore interface {
	ClaimWinningPayment(ctx context.Context, paymentID, orderID uuid.UUID) (domaincommerce.Order, bool, error)
	GetWinningPaymentForOrder(ctx context.Context, orderID uuid.UUID) (domaincommerce.Payment, error)
	UpdatePaymentOutcome(ctx context.Context, paymentID uuid.UUID, outcome string) (domaincommerce.Payment, error)
	CancelPaymentByID(ctx context.Context, paymentID uuid.UUID) (domaincommerce.Payment, error)
	GetLatestNonCapturedPaymentForOrder(ctx context.Context, orderID uuid.UUID) (domaincommerce.Payment, error)
	ListPaymentsForOrder(ctx context.Context, orderID uuid.UUID) ([]domaincommerce.Payment, error)

	RecordCashAcceptanceEvents(ctx context.Context, in RecordCashAcceptanceEventsInput) error
	RecordCashAllocation(ctx context.Context, in RecordCashAllocationInput) (CashAllocationView, error)
	RecordCashChangeEvent(ctx context.Context, in RecordCashChangeEventInput) (CashChangeEventView, error)
	GetOrderMoneyView(ctx context.Context, orderID uuid.UUID) (OrderMoneyView, error)
	InsertLedgerEntry(ctx context.Context, in LedgerEntryInput) error
	UpsertReconciliationCase(ctx context.Context, in domaincommerce.ReconciliationCaseInput) (domaincommerce.ReconciliationCase, error)
}

// RecordCashAcceptanceEventsInput batches hardware acceptance events for idempotent replay.
type RecordCashAcceptanceEventsInput struct {
	MachineID uuid.UUID
	OrderID   uuid.UUID
	Currency  string
	Events    []CashAcceptanceEventInput
}

type CashAcceptanceEventInput struct {
	DeviceEventID     string
	DenominationMinor int64
	CreditSource      string
	AcceptedAt        time.Time
	RawMetadata       []byte
}

type RecordCashAllocationInput struct {
	OrderID                uuid.UUID
	PaymentID              uuid.UUID
	MachineID              uuid.UUID
	AmountMinor            int64
	PreOrderCreditMinor    int64
	PostOrderInsertedMinor int64
	ConsentSource          string
	ConsentedAt            *time.Time
	Currency               string
	IdempotencyKey         string
}

type RecordCashChangeEventInput struct {
	OrderID             uuid.UUID
	PaymentID           uuid.UUID
	MachineID           uuid.UUID
	ChangeDueMinor      int64
	ChangeDispensedMinor int64
	Outcome             string
	LiabilityMinor      int64
	Currency            string
	IdempotencyKey      string
}

type CashAllocationView struct {
	ID                     uuid.UUID
	OrderID                uuid.UUID
	AmountMinor            int64
	PreOrderCreditMinor    int64
	PostOrderInsertedMinor int64
	ConsentSource          string
}

type CashChangeEventView struct {
	ID                   uuid.UUID
	ChangeDueMinor       int64
	ChangeDispensedMinor int64
	Outcome              string
	LiabilityMinor       int64
}

type LedgerEntryInput struct {
	MachineID         *uuid.UUID
	OrderID           *uuid.UUID
	PaymentID         *uuid.UUID
	EntryType         string
	SignedAmountMinor int64
	Currency          string
	OccurredAt        time.Time
	Metadata          []byte
}

// OrderMoneyView is the admin read model for all money on an order.
type OrderMoneyView struct {
	OrderID            uuid.UUID
	WinningPaymentID   *uuid.UUID
	Payments           []PaymentMoneyView
	CashAllocation     *CashAllocationView
	CashChange         *CashChangeEventView
	AcceptanceEvents   []CashAcceptanceEventView
	OutstandingLiability int64
}

type PaymentMoneyView struct {
	Payment       domaincommerce.Payment
	IsWinner      bool
	IsLosingCapture bool
}

type CashAcceptanceEventView struct {
	DeviceEventID     string
	DenominationMinor int64
	CreditSource      string
	AcceptedAt        time.Time
}

// ConfirmCashPaymentInput is the app-layer cash confirm with full evidence.
type ConfirmCashPaymentInput struct {
	OrderID                uuid.UUID
	MachineID              uuid.UUID
	IdempotencyKey         string
	GrossAcceptedMinor     int64
	AllocatedMinor         int64
	PreOrderCreditMinor    int64
	PostOrderInsertedMinor int64
	ChangeDueMinor         int64
	ChangeDispensedMinor   int64
	ChangeOutcome          string
	ConsentSource          string
	ConsentedAt            *time.Time
	Currency               string
	AcceptanceEvents       []CashAcceptanceEventInput
	Simulated              bool
	SimulationRunID        string
	SimulationScenario     string
	FakeBill               bool
	FakeBoard              bool
	OutboxTopic            string
	OutboxEventType        string
	OutboxAggregateType    string
}

type ConfirmCashPaymentResult struct {
	Replay       bool
	Payment      domaincommerce.Payment
	Order        domaincommerce.Order
	Allocation   CashAllocationView
	ChangeEvent  *CashChangeEventView
}

// CancelPaymentSessionInput cancels the latest non-captured payment on an order.
type CancelPaymentSessionInput struct {
	OrderID        uuid.UUID
	MachineID      uuid.UUID
	IdempotencyKey string
	Reason         string
}

type CancelPaymentSessionResult struct {
	Replay       bool
	Order        domaincommerce.Order
	Payment      domaincommerce.Payment
	PaymentFound bool
}

// ArbitrationResult describes winner claim outcome.
type ArbitrationResult struct {
	Won            bool
	Order          domaincommerce.Order
	ExistingWinner *uuid.UUID
}

func normalizeConsentSource(s string) string {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case "explicit_confirm", "implicit_post_order", "operator", "unknown":
		return strings.TrimSpace(strings.ToLower(s))
	default:
		return "unknown"
	}
}

func normalizeChangeOutcome(s string) string {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case "delivered", "delivered_after_fault", "not_delivered", "ambiguous", "none", "":
		if strings.TrimSpace(s) == "" {
			return "none"
		}
		return strings.TrimSpace(strings.ToLower(s))
	default:
		return "ambiguous"
	}
}

func (s *Service) resolveAuthoritativePayment(ctx context.Context, orderID uuid.UUID) (domaincommerce.Payment, error) {
	if s.financial == nil {
		if s.life == nil {
			return domaincommerce.Payment{}, ErrNotConfigured
		}
		return s.life.GetLatestPaymentForOrder(ctx, orderID)
	}
	pay, err := s.financial.GetWinningPaymentForOrder(ctx, orderID)
	if err == nil {
		return pay, nil
	}
	if errors.Is(err, ErrNotFound) && s.life != nil {
		return s.life.GetLatestPaymentForOrder(ctx, orderID)
	}
	return domaincommerce.Payment{}, err
}

// AttemptWinningPaymentClaim atomically claims order financial ownership for a captured payment.
func (s *Service) AttemptWinningPaymentClaim(ctx context.Context, paymentID, orderID uuid.UUID) (ArbitrationResult, error) {
	if s.financial == nil {
		return ArbitrationResult{}, ErrNotConfigured
	}
	if paymentID == uuid.Nil || orderID == uuid.Nil {
		return ArbitrationResult{}, errors.Join(ErrInvalidArgument, errors.New("payment_id and order_id required"))
	}
	ord, won, err := s.financial.ClaimWinningPayment(ctx, paymentID, orderID)
	if err != nil {
		return ArbitrationResult{}, err
	}
	if won {
		if _, err := s.financial.UpdatePaymentOutcome(ctx, paymentID, "winner"); err != nil {
			return ArbitrationResult{}, err
		}
		return ArbitrationResult{Won: true, Order: ord}, nil
	}
	var existing *uuid.UUID
	ord, gerr := s.life.GetOrderByID(ctx, orderID)
	if gerr == nil && ord.WinningPaymentID != nil {
		existing = ord.WinningPaymentID
	}
	if _, err := s.financial.UpdatePaymentOutcome(ctx, paymentID, "refund_required"); err != nil {
		return ArbitrationResult{}, err
	}
	orderIDCopy := orderID
	paymentIDCopy := paymentID
	_, _ = s.financial.UpsertReconciliationCase(ctx, domaincommerce.ReconciliationCaseInput{
		CaseType:      "late_capture_refund_required",
		Severity:      "critical",
		OrderID:       &orderIDCopy,
		PaymentID:     &paymentIDCopy,
		Reason:        "Payment captured but order already has a winning payment",
		CorrelationKey: "financial_correctness:losing_capture:" + paymentID.String(),
	})
	return ArbitrationResult{Won: false, Order: ord, ExistingWinner: existing}, nil
}
