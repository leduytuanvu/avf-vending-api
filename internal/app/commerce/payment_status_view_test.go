package commerce

import (
	"context"
	"testing"
	"time"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestGetPaymentStatusView_pendingDoesNotQueryProvider(t *testing.T) {
	t.Parallel()
	orderID := uuid.New()
	paymentID := uuid.New()
	provider := &refreshQueryProvider{
		snapshot: domaincommerce.PaymentStatusSnapshot{NormalizedState: "captured"},
	}
	life := &refreshLifecycleStub{
		orderStatusLifecycleStub: orderStatusLifecycleStub{
			order: domaincommerce.Order{ID: orderID, Status: "created"},
			payment: domaincommerce.Payment{
				ID:        paymentID,
				OrderID:   orderID,
				Provider:  "momo",
				State:     "pending",
				CreatedAt: time.Now().Add(-10 * time.Second),
			},
		},
		providerRef: "momo-ref-status",
	}
	svc := NewService(Deps{
		OrderVend:              stubOrderVend{},
		PaymentOutbox:          stubPaymentOutbox{},
		Lifecycle:              life,
		WebhookPersist:         &captureWebhookPersistence{},
		SaleLines:              stubSaleLineResolver{},
		PaymentSessionRegistry: refreshProviderRegistry{provider: provider},
	})

	got, err := svc.GetPaymentStatusView(context.Background(), uuid.Nil, orderID, paymentID, "AVF-01")
	require.NoError(t, err)
	require.Equal(t, "pending", got.Payment.State)
	require.Equal(t, "awaiting_callback", got.PaymentDiagnostic)
	require.Equal(t, uuid.Nil, provider.lastLookup.PaymentID)
}

func TestOldestPendingPaymentCreatedAt_usesOldestCreated(t *testing.T) {
	t.Parallel()
	orderID := uuid.New()
	now := time.Now()
	older := domaincommerce.Payment{
		ID:        uuid.New(),
		OrderID:   orderID,
		State:     "created",
		CreatedAt: now.Add(-8 * time.Second),
	}
	newer := domaincommerce.Payment{
		ID:        uuid.New(),
		OrderID:   orderID,
		State:     "pending",
		CreatedAt: now.Add(-1 * time.Second),
	}
	svc := NewService(Deps{
		OrderVend: stubOrderVendWorkflow{},
		SaleLines: stubSaleLineResolver{},
		Lifecycle: &orderStatusLifecycleStub{order: domaincommerce.Order{ID: orderID}, payment: newer},
		FinancialCorrectness: &refreshPaymentsFinancialStub{
			payments: []domaincommerce.Payment{newer, older},
		},
	})

	got, ok, err := svc.OldestPendingPaymentCreatedAt(context.Background(), orderID)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, older.CreatedAt, got)
}

func TestOldestPendingPaymentCreatedAt_noPending(t *testing.T) {
	t.Parallel()
	orderID := uuid.New()
	captured := domaincommerce.Payment{
		ID:        uuid.New(),
		OrderID:   orderID,
		State:     "captured",
		CreatedAt: time.Now(),
	}
	svc := NewService(Deps{
		OrderVend: stubOrderVendWorkflow{},
		SaleLines: stubSaleLineResolver{},
		Lifecycle: &orderStatusLifecycleStub{order: domaincommerce.Order{ID: orderID}, payment: captured},
		FinancialCorrectness: &refreshPaymentsFinancialStub{
			payments: []domaincommerce.Payment{captured},
		},
	})

	_, ok, err := svc.OldestPendingPaymentCreatedAt(context.Background(), orderID)
	require.NoError(t, err)
	require.False(t, ok)
}
