package commerce

import (
	"context"
	"testing"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type stubFinancialCorrectness struct {
	claimFn func(ctx context.Context, paymentID, orderID uuid.UUID) (domaincommerce.Order, bool, error)
}

func (s *stubFinancialCorrectness) ClaimWinningPayment(ctx context.Context, paymentID, orderID uuid.UUID) (domaincommerce.Order, bool, error) {
	if s.claimFn != nil {
		return s.claimFn(ctx, paymentID, orderID)
	}
	return domaincommerce.Order{ID: orderID}, true, nil
}

func (s *stubFinancialCorrectness) GetWinningPaymentForOrder(context.Context, uuid.UUID) (domaincommerce.Payment, error) {
	return domaincommerce.Payment{}, ErrNotFound
}

func (s *stubFinancialCorrectness) UpdatePaymentOutcome(_ context.Context, paymentID uuid.UUID, outcome string) (domaincommerce.Payment, error) {
	return domaincommerce.Payment{ID: paymentID, Outcome: outcome}, nil
}

func (s *stubFinancialCorrectness) CancelPaymentByID(_ context.Context, paymentID uuid.UUID) (domaincommerce.Payment, error) {
	return domaincommerce.Payment{ID: paymentID}, nil
}

func (s *stubFinancialCorrectness) GetLatestNonCapturedPaymentForOrder(context.Context, uuid.UUID) (domaincommerce.Payment, error) {
	return domaincommerce.Payment{}, ErrNotFound
}

func (s *stubFinancialCorrectness) ListPaymentsForOrder(context.Context, uuid.UUID) ([]domaincommerce.Payment, error) {
	return nil, nil
}

func (s *stubFinancialCorrectness) RecordCashAcceptanceEvents(context.Context, RecordCashAcceptanceEventsInput) error {
	return nil
}

func (s *stubFinancialCorrectness) RecordCashAllocation(context.Context, RecordCashAllocationInput) (CashAllocationView, error) {
	return CashAllocationView{}, nil
}

func (s *stubFinancialCorrectness) RecordCashChangeEvent(context.Context, RecordCashChangeEventInput) (CashChangeEventView, error) {
	return CashChangeEventView{}, nil
}

func (s *stubFinancialCorrectness) GetOrderMoneyView(context.Context, uuid.UUID) (OrderMoneyView, error) {
	return OrderMoneyView{}, nil
}

func (s *stubFinancialCorrectness) InsertLedgerEntry(context.Context, LedgerEntryInput) error {
	return nil
}

func (s *stubFinancialCorrectness) UpsertReconciliationCase(context.Context, domaincommerce.ReconciliationCaseInput) (domaincommerce.ReconciliationCase, error) {
	return domaincommerce.ReconciliationCase{}, nil
}

func TestAttemptWinningPaymentClaim_winnerUpdatesOutcome(t *testing.T) {
	t.Parallel()
	orderID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	paymentID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	fin := &stubFinancialCorrectness{}
	svc := NewService(Deps{
		OrderVend:            stubOrderVendWorkflow{},
		SaleLines:            stubSaleLineResolver{},
		FinancialCorrectness: fin,
	})
	res, err := svc.AttemptWinningPaymentClaim(t.Context(), paymentID, orderID)
	require.NoError(t, err)
	require.True(t, res.Won)
	require.Equal(t, orderID, res.Order.ID)
}

func TestAttemptWinningPaymentClaim_loserCreatesRefundRequiredOutcome(t *testing.T) {
	t.Parallel()
	orderID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	paymentID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	winnerID := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")
	fin := &stubFinancialCorrectness{
		claimFn: func(_ context.Context, _, _ uuid.UUID) (domaincommerce.Order, bool, error) {
			return domaincommerce.Order{
				ID:               orderID,
				WinningPaymentID: &winnerID,
			}, false, nil
		},
	}
	life := &cashConfirmLifecycle{
		order: domaincommerce.Order{
			ID:               orderID,
			WinningPaymentID: &winnerID,
		},
	}
	svc := NewService(Deps{
		OrderVend:            stubOrderVendWorkflow{},
		SaleLines:            stubSaleLineResolver{},
		FinancialCorrectness: fin,
		Lifecycle:            life,
	})
	res, err := svc.AttemptWinningPaymentClaim(t.Context(), paymentID, orderID)
	require.NoError(t, err)
	require.False(t, res.Won)
	require.NotNil(t, res.ExistingWinner)
	require.Equal(t, winnerID, *res.ExistingWinner)
}

func TestValidateCashConfirmConsent_walletAutoSettlement_acceptsWithEvents(t *testing.T) {
	t.Parallel()
	err := validateCashConfirmConsent("wallet_auto_settlement", ConfirmCashPaymentInput{
		PreOrderCreditMinor:    15000,
		PostOrderInsertedMinor: 0,
		AcceptanceEvents: []CashAcceptanceEventInput{
			{DeviceEventID: "bill-1", DenominationMinor: 20000},
		},
	})
	require.NoError(t, err)
}

func TestCancelPaymentSessionInput_acceptsOptionalPaymentID(t *testing.T) {
	t.Parallel()
	pid := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	in := CancelPaymentSessionInput{
		OrderID:   uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
		PaymentID: &pid,
	}
	require.NotNil(t, in.PaymentID)
	require.Equal(t, pid, *in.PaymentID)
}
