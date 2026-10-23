package commerce

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	platformpayments "github.com/avf/avf-vending-api/internal/platform/payments"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type refreshProviderRegistry struct {
	provider platformpayments.PaymentProvider
}

func (r refreshProviderRegistry) ResolveForPaymentSession(config.AppEnvironment, string) (platformpayments.PaymentProvider, string, error) {
	return r.provider, "momo", nil
}

func (r refreshProviderRegistry) Get(string) platformpayments.PaymentProvider {
	return r.provider
}

type refreshQueryProvider struct {
	lastLookup domaincommerce.PaymentProviderLookup
	snapshot   domaincommerce.PaymentStatusSnapshot
}

func (p *refreshQueryProvider) Key() string { return "momo" }

func (p *refreshQueryProvider) VerifyWebhookSignature(string, string, string, []byte, time.Duration) error {
	return nil
}

func (p *refreshQueryProvider) ParseWebhookEvent([]byte) (platformpayments.CommerceWebhookEventJSON, error) {
	return platformpayments.CommerceWebhookEventJSON{}, nil
}

func (p *refreshQueryProvider) SupportsQueryPaymentStatus() bool { return true }

func (p *refreshQueryProvider) QueryPaymentStatus(_ context.Context, lookup domaincommerce.PaymentProviderLookup) (domaincommerce.PaymentStatusSnapshot, error) {
	p.lastLookup = lookup
	return p.snapshot, nil
}

func (p *refreshQueryProvider) CreatePaymentSession(context.Context, platformpayments.CreatePaymentSessionInput) (platformpayments.CreatePaymentSessionResult, error) {
	return platformpayments.CreatePaymentSessionResult{}, nil
}

func (p *refreshQueryProvider) CancelPayment(context.Context, platformpayments.CancelPaymentInput) error {
	return nil
}

func (p *refreshQueryProvider) RefundPayment(context.Context, platformpayments.RefundPaymentInput) error {
	return nil
}

type refreshLifecycleStub struct {
	orderStatusLifecycleStub
	providerRef string
}

func (s *refreshLifecycleStub) GetLatestPaymentAttemptProviderReference(context.Context, uuid.UUID) (string, error) {
	return s.providerRef, nil
}

func TestRefreshPendingPaymentFromProvider_usesProviderNativeVerifiedAndMachineCode(t *testing.T) {
	orderID := uuid.New()
	paymentID := uuid.New()
	provider := &refreshQueryProvider{
		snapshot: domaincommerce.PaymentStatusSnapshot{
			NormalizedState: "captured",
			ProviderHint:    []byte(`{"resultCode":0}`),
		},
	}
	life := &refreshLifecycleStub{
		orderStatusLifecycleStub: orderStatusLifecycleStub{
			order: domaincommerce.Order{ID: orderID, Status: "created"},
			vend: domaincommerce.VendSession{
				ID:      uuid.New(),
				OrderID: orderID,
				State:   "pending",
			},
			payment: domaincommerce.Payment{
				ID:          paymentID,
				OrderID:     orderID,
				Provider:    "momo",
				State:       "created",
				AmountMinor: 2000,
			},
		},
		providerRef: "momo-ref-123",
	}
	capture := &captureWebhookPersistence{}
	svc := NewService(Deps{
		OrderVend:              stubOrderVend{},
		PaymentOutbox:          stubPaymentOutbox{},
		Lifecycle:              life,
		WebhookPersist:         capture,
		SaleLines:              stubSaleLineResolver{},
		PaymentSessionRegistry: refreshProviderRegistry{provider: provider},
	})

	svc.RefreshPendingPaymentFromProvider(context.Background(), uuid.Nil, orderID, "TFO-MACHINE-01")

	require.Equal(t, "TFO-MACHINE-01", provider.lastLookup.MachineExternalCode)
	require.Equal(t, "momo-ref-123", provider.lastLookup.ProviderReference)
	require.Equal(t, "provider_native_verified", capture.got.WebhookValidationStatus)
	require.Equal(t, "captured", capture.got.NormalizedPaymentState)
	require.Equal(t, "provider.query_refresh", capture.got.EventType)
}

func TestRefreshPendingPaymentFromProvider_fallsBackToPaymentIDReference(t *testing.T) {
	orderID := uuid.New()
	paymentID := uuid.New()
	provider := &refreshQueryProvider{
		snapshot: domaincommerce.PaymentStatusSnapshot{NormalizedState: "pending"},
	}
	life := &refreshLifecycleStub{
		orderStatusLifecycleStub: orderStatusLifecycleStub{
			order: domaincommerce.Order{ID: orderID, Status: "created"},
			vend: domaincommerce.VendSession{
				ID:      uuid.New(),
				OrderID: orderID,
				State:   "pending",
			},
			payment: domaincommerce.Payment{
				ID:          paymentID,
				OrderID:     orderID,
				Provider:    "momo",
				State:       "created",
				AmountMinor: 2000,
			},
		},
		providerRef: "",
	}
	svc := NewService(Deps{
		OrderVend:              stubOrderVend{},
		PaymentOutbox:          stubPaymentOutbox{},
		Lifecycle:              life,
		WebhookPersist:         &captureWebhookPersistence{},
		SaleLines:              stubSaleLineResolver{},
		PaymentSessionRegistry: refreshProviderRegistry{provider: provider},
	})

	svc.RefreshPendingPaymentFromProvider(context.Background(), uuid.Nil, orderID, "AVF-01")
	require.NotEmpty(t, provider.lastLookup.ProviderReference)
}

func TestRefreshPendingPaymentFromProvider_skipsNonTerminalSnapshot(t *testing.T) {
	orderID := uuid.New()
	provider := &refreshQueryProvider{
		snapshot: domaincommerce.PaymentStatusSnapshot{NormalizedState: "pending"},
	}
	life := &refreshLifecycleStub{
		orderStatusLifecycleStub: orderStatusLifecycleStub{
			order: domaincommerce.Order{ID: orderID, Status: "created"},
			vend: domaincommerce.VendSession{
				ID:      uuid.New(),
				OrderID: orderID,
				State:   "pending",
			},
			payment: domaincommerce.Payment{
				ID:       uuid.New(),
				OrderID:  orderID,
				Provider: "momo",
				State:    "created",
			},
		},
		providerRef: "ref",
	}
	capture := &captureWebhookPersistence{}
	svc := NewService(Deps{
		OrderVend:              stubOrderVend{},
		PaymentOutbox:          stubPaymentOutbox{},
		Lifecycle:              life,
		WebhookPersist:         capture,
		SaleLines:              stubSaleLineResolver{},
		PaymentSessionRegistry: refreshProviderRegistry{provider: provider},
	})

	svc.RefreshPendingPaymentFromProvider(context.Background(), uuid.Nil, orderID, "AVF-01")
	require.Empty(t, capture.got.EventType)
}

func TestRefreshPendingPaymentFromProvider_appliesCapturedHintJSON(t *testing.T) {
	orderID := uuid.New()
	hint, err := json.Marshal(map[string]any{"resultCode": 0})
	require.NoError(t, err)
	provider := &refreshQueryProvider{
		snapshot: domaincommerce.PaymentStatusSnapshot{
			NormalizedState: "captured",
			ProviderHint:    hint,
		},
	}
	life := &refreshLifecycleStub{
		orderStatusLifecycleStub: orderStatusLifecycleStub{
			order: domaincommerce.Order{ID: orderID, Status: "created"},
			vend: domaincommerce.VendSession{
				ID:      uuid.New(),
				OrderID: orderID,
				State:   "pending",
			},
			payment: domaincommerce.Payment{
				ID:       uuid.New(),
				OrderID:  orderID,
				Provider: "momo",
				State:    "created",
			},
		},
		providerRef: "ref",
	}
	capture := &captureWebhookPersistence{}
	svc := NewService(Deps{
		OrderVend:              stubOrderVend{},
		PaymentOutbox:          stubPaymentOutbox{},
		Lifecycle:              life,
		WebhookPersist:         capture,
		SaleLines:              stubSaleLineResolver{},
		PaymentSessionRegistry: refreshProviderRegistry{provider: provider},
	})

	svc.RefreshPendingPaymentFromProvider(context.Background(), uuid.Nil, orderID, "AVF-01")
	require.JSONEq(t, string(hint), string(capture.got.Payload))
}

func TestRefreshPendingPaymentFromProvider_doesNotApplyFailedFromQuery(t *testing.T) {
	orderID := uuid.New()
	provider := &refreshQueryProvider{
		snapshot: domaincommerce.PaymentStatusSnapshot{NormalizedState: "failed"},
	}
	life := &refreshLifecycleStub{
		orderStatusLifecycleStub: orderStatusLifecycleStub{
			order: domaincommerce.Order{ID: orderID, Status: "created"},
			vend: domaincommerce.VendSession{
				ID:      uuid.New(),
				OrderID: orderID,
				State:   "pending",
			},
			payment: domaincommerce.Payment{
				ID:       uuid.New(),
				OrderID:  orderID,
				Provider: "momo",
				State:    "created",
			},
		},
		providerRef: "ref",
	}
	capture := &captureWebhookPersistence{}
	svc := NewService(Deps{
		OrderVend:              stubOrderVend{},
		PaymentOutbox:          stubPaymentOutbox{},
		Lifecycle:              life,
		WebhookPersist:         capture,
		SaleLines:              stubSaleLineResolver{},
		PaymentSessionRegistry: refreshProviderRegistry{provider: provider},
	})

	out := svc.RefreshPendingPaymentFromProvider(context.Background(), uuid.Nil, orderID, "AVF-01")
	require.Equal(t, "provider_reported_failure", out.Diagnostic)
	require.Empty(t, capture.got.EventType)
}

func TestRefreshPendingPaymentFromProvider_queriesWhenSlotIndexMisses(t *testing.T) {
	orderID := uuid.New()
	paymentID := uuid.New()
	provider := &refreshQueryProvider{
		snapshot: domaincommerce.PaymentStatusSnapshot{
			NormalizedState: "captured",
			ProviderHint:    []byte(`{"resultCode":0}`),
		},
	}
	life := &refreshLifecycleStub{
		orderStatusLifecycleStub: orderStatusLifecycleStub{
			order: domaincommerce.Order{ID: orderID, Status: "created"},
			vend: domaincommerce.VendSession{
				ID:        uuid.New(),
				OrderID:   orderID,
				SlotIndex: 11,
				State:     "pending",
			},
			payment: domaincommerce.Payment{
				ID:          paymentID,
				OrderID:     orderID,
				Provider:    "momo",
				State:       "created",
				AmountMinor: 2000,
			},
			slotMiss: true,
		},
		providerRef: "momo-ref-slot-miss",
	}
	capture := &captureWebhookPersistence{}
	svc := NewService(Deps{
		OrderVend:              stubOrderVend{},
		PaymentOutbox:          stubPaymentOutbox{},
		Lifecycle:              life,
		WebhookPersist:         capture,
		SaleLines:              stubSaleLineResolver{},
		PaymentSessionRegistry: refreshProviderRegistry{provider: provider},
	})

	out := svc.RefreshPendingPaymentFromProvider(context.Background(), uuid.Nil, orderID, "AVF-01")

	require.NotEqual(t, "checkout_status_error", out.Diagnostic)
	require.Equal(t, "momo-ref-slot-miss", provider.lastLookup.ProviderReference)
	require.Equal(t, "captured", capture.got.NormalizedPaymentState)
}

type refreshPaymentsFinancialStub struct {
	payments []domaincommerce.Payment
}

func (s *refreshPaymentsFinancialStub) ClaimWinningPayment(context.Context, uuid.UUID, uuid.UUID) (domaincommerce.Order, bool, error) {
	return domaincommerce.Order{}, false, ErrNotConfigured
}

func (s *refreshPaymentsFinancialStub) GetWinningPaymentForOrder(context.Context, uuid.UUID) (domaincommerce.Payment, error) {
	return domaincommerce.Payment{}, ErrNotFound
}

func (s *refreshPaymentsFinancialStub) UpdatePaymentOutcome(context.Context, uuid.UUID, string) (domaincommerce.Payment, error) {
	return domaincommerce.Payment{}, ErrNotConfigured
}

func (s *refreshPaymentsFinancialStub) CancelPaymentByID(context.Context, uuid.UUID) (domaincommerce.Payment, error) {
	return domaincommerce.Payment{}, ErrNotConfigured
}

func (s *refreshPaymentsFinancialStub) GetLatestNonCapturedPaymentForOrder(context.Context, uuid.UUID) (domaincommerce.Payment, error) {
	return domaincommerce.Payment{}, ErrNotFound
}

func (s *refreshPaymentsFinancialStub) ListPaymentsForOrder(context.Context, uuid.UUID) ([]domaincommerce.Payment, error) {
	return s.payments, nil
}

func (s *refreshPaymentsFinancialStub) RecordCashAcceptanceEvents(context.Context, RecordCashAcceptanceEventsInput) error {
	return nil
}

func (s *refreshPaymentsFinancialStub) RecordCashAllocation(context.Context, RecordCashAllocationInput) (CashAllocationView, error) {
	return CashAllocationView{}, ErrNotConfigured
}

func (s *refreshPaymentsFinancialStub) RecordCashChangeEvent(context.Context, RecordCashChangeEventInput) (CashChangeEventView, error) {
	return CashChangeEventView{}, ErrNotConfigured
}

func (s *refreshPaymentsFinancialStub) GetOrderMoneyView(context.Context, uuid.UUID) (OrderMoneyView, error) {
	return OrderMoneyView{}, ErrNotConfigured
}

func (s *refreshPaymentsFinancialStub) InsertLedgerEntry(context.Context, LedgerEntryInput) error {
	return nil
}

func (s *refreshPaymentsFinancialStub) UpsertReconciliationCase(context.Context, domaincommerce.ReconciliationCaseInput) (domaincommerce.ReconciliationCase, error) {
	return domaincommerce.ReconciliationCase{}, ErrNotConfigured
}

type refreshMultiPaymentLife struct {
	refreshLifecycleStub
	byID map[uuid.UUID]domaincommerce.Payment
}

func (s *refreshMultiPaymentLife) GetPaymentByID(_ context.Context, paymentID uuid.UUID) (domaincommerce.Payment, error) {
	if pay, ok := s.byID[paymentID]; ok {
		return pay, nil
	}
	return domaincommerce.Payment{}, ErrNotFound
}

func (s *refreshMultiPaymentLife) GetLatestPaymentForOrder(context.Context, uuid.UUID) (domaincommerce.Payment, error) {
	var latest domaincommerce.Payment
	for _, pay := range s.byID {
		if latest.ID == uuid.Nil || pay.CreatedAt.After(latest.CreatedAt) {
			latest = pay
		}
	}
	if latest.ID == uuid.Nil {
		return domaincommerce.Payment{}, ErrNotFound
	}
	return latest, nil
}

type selectiveCaptureRefreshProvider struct {
	capturePaymentID uuid.UUID
	refreshQueryProvider
}

func (p *selectiveCaptureRefreshProvider) QueryPaymentStatus(_ context.Context, lookup domaincommerce.PaymentProviderLookup) (domaincommerce.PaymentStatusSnapshot, error) {
	p.lastLookup = lookup
	if lookup.PaymentID == p.capturePaymentID {
		return domaincommerce.PaymentStatusSnapshot{
			NormalizedState: "captured",
			ProviderHint:    []byte(`{"resultCode":0}`),
		}, nil
	}
	return domaincommerce.PaymentStatusSnapshot{NormalizedState: "pending"}, nil
}

func TestRefreshPendingPaymentFromProvider_capturesOlderPendingWhenLatestUnpaid(t *testing.T) {
	orderID := uuid.New()
	momoID := uuid.New()
	shopeeID := uuid.New()
	now := time.Now()
	provider := &selectiveCaptureRefreshProvider{capturePaymentID: momoID}
	momoPay := domaincommerce.Payment{
		ID:          momoID,
		OrderID:     orderID,
		Provider:    "momo",
		State:       "created",
		AmountMinor: 2000,
		CreatedAt:   now.Add(-2 * time.Minute),
	}
	shopeePay := domaincommerce.Payment{
		ID:          shopeeID,
		OrderID:     orderID,
		Provider:    "shopeepay",
		State:       "created",
		AmountMinor: 2000,
		CreatedAt:   now,
	}
	life := &refreshMultiPaymentLife{
		refreshLifecycleStub: refreshLifecycleStub{
			orderStatusLifecycleStub: orderStatusLifecycleStub{
				order: domaincommerce.Order{ID: orderID, Status: "created"},
				vend: domaincommerce.VendSession{
					ID:      uuid.New(),
					OrderID: orderID,
					State:   "pending",
				},
				payment: shopeePay,
			},
			providerRef: "provider-ref",
		},
		byID: map[uuid.UUID]domaincommerce.Payment{
			momoID:   momoPay,
			shopeeID: shopeePay,
		},
	}
	capture := &captureWebhookPersistence{}
	fin := &refreshPaymentsFinancialStub{
		payments: []domaincommerce.Payment{momoPay, shopeePay},
	}
	svc := NewService(Deps{
		OrderVend:              stubOrderVend{},
		PaymentOutbox:          stubPaymentOutbox{},
		Lifecycle:              life,
		WebhookPersist:         capture,
		SaleLines:              stubSaleLineResolver{},
		PaymentSessionRegistry: refreshProviderRegistry{provider: provider},
		FinancialCorrectness:   fin,
	})

	out := svc.RefreshPendingPaymentFromProvider(context.Background(), uuid.Nil, orderID, "AVF-01")

	require.Equal(t, "captured", out.Diagnostic)
	require.Equal(t, momoID, capture.got.PaymentID)
	require.Equal(t, "captured", capture.got.NormalizedPaymentState)
}

func TestPaymentQueryMinInterval_acceleratedWithinTwoMinutes(t *testing.T) {
	t.Parallel()
	createdAt := time.Now().Add(-30 * time.Second)
	require.Equal(t, paymentQueryMinIntervalActiveQR, paymentQueryMinInterval(createdAt, time.Now()))
}

func TestPaymentQueryMinInterval_defaultAfterAcceleratedWindow(t *testing.T) {
	t.Parallel()
	createdAt := time.Now().Add(-3 * time.Minute)
	require.Equal(t, paymentQueryMinIntervalDefault, paymentQueryMinInterval(createdAt, time.Now()))
}
