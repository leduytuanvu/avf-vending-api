package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/config"
)

func TestCommercePublicPaymentWebhook_DoesNotRequireIdempotencyKey(t *testing.T) {
	t.Parallel()

	app := &api.HTTPApplication{}
	cfg := &config.Config{}
	cfg.Commerce.PaymentWebhookHMACSecret = "test-webhook-secret"

	req := httptest.NewRequest(http.MethodPost, "/v1/commerce/orders/11111111-1111-1111-1111-111111111111/payments/22222222-2222-2222-2222-222222222222/webhooks", strings.NewReader(`{"provider":"psp","provider_reference":"ref-1"}`))
	req.Header.Set("X-AVF-Webhook-Timestamp", strconv.FormatInt(time.Now().UTC().Unix(), 10))
	req.Header.Set("X-AVF-Webhook-Signature", "bad-signature")

	rec := httptest.NewRecorder()
	commercePublicPaymentWebhookHandler(app, cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "webhook_auth_failed") {
		t.Fatalf("expected webhook auth failure body, got %s", rec.Body.String())
	}
}
