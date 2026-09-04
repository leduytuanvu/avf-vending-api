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
