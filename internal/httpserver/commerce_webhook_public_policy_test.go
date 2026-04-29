package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/config"
)

func TestCommercePublicPaymentWebhook_production_rejectsMissingSecretWithoutUnsafe(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		AppEnv:   config.AppEnvProduction,
		Commerce: config.CommerceHTTPConfig{},
	}
	app := &api.HTTPApplication{}

	req := httptest.NewRequest(http.MethodPost, "/v1/commerce/orders/11111111-1111-1111-1111-111111111111/payments/22222222-2222-2222-2222-222222222222/webhooks", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	commercePublicPaymentWebhookHandler(app, cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "webhook_hmac_required") {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestCommercePublicPaymentWebhook_staging_rejectsUnsignedWithoutUnsafe(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		AppEnv:   config.AppEnvStaging,
		Commerce: config.CommerceHTTPConfig{},
	}
	app := &api.HTTPApplication{}

	req := httptest.NewRequest(http.MethodPost, "/v1/commerce/orders/11111111-1111-1111-1111-111111111111/payments/22222222-2222-2222-2222-222222222222/webhooks", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	commercePublicPaymentWebhookHandler(app, cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "webhook_hmac_required") {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestCommercePublicPaymentWebhook_development_capabilityWhenUnsignedNotAllowed(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		AppEnv: config.AppEnvDevelopment,
		Commerce: config.CommerceHTTPConfig{
			PaymentWebhookAllowUnsigned: false,
		},
	}
	app := &api.HTTPApplication{}

	req := httptest.NewRequest(http.MethodPost, "/v1/commerce/orders/11111111-1111-1111-1111-111111111111/payments/22222222-2222-2222-2222-222222222222/webhooks", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	commercePublicPaymentWebhookHandler(app, cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "capability_not_configured") {
		t.Fatalf("body=%s", rec.Body.String())
	}
}
