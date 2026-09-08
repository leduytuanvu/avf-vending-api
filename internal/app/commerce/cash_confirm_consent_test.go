package commerce

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestValidateCashConfirmConsent_walletAutoSettlement_requiresEvents(t *testing.T) {
	t.Parallel()
	err := validateCashConfirmConsent("wallet_auto_settlement", ConfirmCashPaymentInput{
		PreOrderCreditMinor:    15000,
		PostOrderInsertedMinor: 0,
		AcceptanceEvents:       nil,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidArgument)
}

func TestValidateCashConfirmConsent_implicitPostOrderWithPreOrderCredit_rejected(t *testing.T) {
	t.Parallel()
	err := validateCashConfirmConsent("implicit_post_order", ConfirmCashPaymentInput{
		PreOrderCreditMinor:    15000,
		PostOrderInsertedMinor: 0,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidArgument)
}

func TestConfirmCashPayment_walletAutoSettlement_acceptsWithEvents(t *testing.T) {
	t.Parallel()
	orderID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	machineID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	life := &cashConfirmLifecycle{
		order: domainOrder(orderID, machineID, 15000, PricingSourceMachineLocalVerified),
	}
	payments := &cashConfirmPayments{}
	svc := NewService(Deps{
		OrderVend:     stubOrderVendWorkflow{},
		PaymentOutbox: payments,
		Lifecycle:     life,
		SaleLines:     stubSaleLineResolver{},
	})
	life.payments = payments
	res, err := svc.ConfirmCashPayment(t.Context(), ConfirmCashPaymentInput{
		OrderID:                orderID,
		MachineID:              machineID,
		IdempotencyKey:         "cash-wallet-auto",
		AllocatedMinor:         15000,
		GrossAcceptedMinor:     15000,
		PreOrderCreditMinor:    15000,
		PostOrderInsertedMinor: 0,
		ConsentSource:          "wallet_auto_settlement",
		Currency:               "VND",
		AcceptanceEvents: []CashAcceptanceEventInput{
			{DeviceEventID: "bill-evt-1", DenominationMinor: 20000, CreditSource: "stacked_cashbox"},
		},
		OutboxTopic:         "commerce",
		OutboxEventType:     "cash",
		OutboxAggregateType: "order",
	})
	require.NoError(t, err)
	require.Equal(t, int64(15000), res.Payment.AmountMinor)
	require.Equal(t, "captured", res.Payment.State)
}

func TestConfirmCashPayment_implicitPostOrderWithPreOrderCredit_rejected(t *testing.T) {
	t.Parallel()
	orderID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	machineID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	life := &cashConfirmLifecycle{
		order: domainOrder(orderID, machineID, 15000, PricingSourceMachineLocalVerified),
	}
	payments := &cashConfirmPayments{}
	svc := NewService(Deps{
		OrderVend:     stubOrderVendWorkflow{},
		PaymentOutbox: payments,
		Lifecycle:     life,
		SaleLines:     stubSaleLineResolver{},
	})
	_, err := svc.ConfirmCashPayment(t.Context(), ConfirmCashPaymentInput{
		OrderID:                orderID,
		MachineID:              machineID,
		IdempotencyKey:         "cash-implicit-bad",
		AllocatedMinor:         15000,
		GrossAcceptedMinor:     15000,
		PreOrderCreditMinor:    15000,
		PostOrderInsertedMinor: 0,
		ConsentSource:          "implicit_post_order",
		Currency:               "VND",
		OutboxTopic:            "commerce",
		OutboxEventType:        "cash",
		OutboxAggregateType:    "order",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidArgument)
}

func TestNormalizeConsentSource_walletAutoSettlement(t *testing.T) {
	t.Parallel()
	require.Equal(t, "wallet_auto_settlement", normalizeConsentSource("wallet_auto_settlement"))
}
