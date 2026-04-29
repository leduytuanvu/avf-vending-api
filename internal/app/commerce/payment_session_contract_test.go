package commerce

import (
	"testing"
	"time"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPaymentLifecycleClientStatus(t *testing.T) {
	t.Parallel()
	require.Equal(t, "pending", paymentLifecycleClientStatus("created"))
	require.Equal(t, "paid", paymentLifecycleClientStatus("captured"))
	require.Equal(t, "refunded", paymentLifecycleClientStatus("partially_refunded"))
}

func TestBuildPaymentSessionKioskView_extractsURLsAndExpiry(t *testing.T) {
	t.Parallel()
	oid := uuid.New()
	pid := uuid.New()
	vid := uuid.New()
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	payload := []byte(`{
	  "checkout_url":"https://pay.example/c",
	  "qr_url":"https://pay.example/q",
	  "expires_at":"2026-01-03T00:00:00Z",
	  "expires_in": 120
	}`)
	view := BuildPaymentSessionKioskView(CheckoutStatusView{
		Order: domaincommerce.Order{ID: oid},
		Vend:  domaincommerce.VendSession{ID: vid},
	}, domaincommerce.PaymentOutboxResult{
		Payment: domaincommerce.Payment{
			ID:          pid,
			Provider:    "psp_fixture",
			State:       "created",
			AmountMinor: 500,
			Currency:    "usd",
			CreatedAt:   ts,
		},
		Outbox: domaincommerce.OutboxEvent{ID: 42},
	}, payload, "idem-key-1")
	require.Equal(t, oid.String(), view.SaleID)
	require.Equal(t, vid.String(), view.SessionID)
	require.Equal(t, pid.String(), view.PaymentID)
	require.NotNil(t, view.CheckoutURL)
	require.Equal(t, "https://pay.example/c", *view.CheckoutURL)
	require.NotNil(t, view.QrURL)
	require.NotNil(t, view.ExpiresAt)
	require.True(t, view.ExpiresAt.Equal(time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)))
}
