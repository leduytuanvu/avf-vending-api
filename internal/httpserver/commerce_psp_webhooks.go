package httpserver

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/api"
	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/observability"
	platformpayments "github.com/avf/avf-vending-api/internal/platform/payments"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// mountCommercePSPWebhooks registers native PSP IPN/callback routes on /v1 (public, no Bearer).
func mountCommercePSPWebhooks(r chi.Router, app *api.HTTPApplication, cfg *config.Config, abuse *AbuseProtection, writeRL func(http.Handler) http.Handler) {
	if app == nil || app.Commerce == nil || cfg == nil || app.TelemetryStore == nil || app.PaymentProviders == nil {
		return
	}
	if abuse == nil {
		abuse = &AbuseProtection{}
	}
	if writeRL == nil {
		writeRL = func(next http.Handler) http.Handler { return next }
	}
	r.Route("/commerce/webhooks", func(r chi.Router) {
		r.With(writeRL, abuse.WebhookPOST()).Post("/momo", MoMoNativeIPNHandler(app, cfg))
		r.With(writeRL, abuse.WebhookPOST()).Post("/zalopay", ZaloPayNativeCallbackHandler(app, cfg))
		r.With(writeRL, abuse.WebhookPOST()).Post("/shopeepay", ShopeePayNativeCallbackHandler(app, cfg))
		r.With(abuse.WebhookPOST()).Get("/vnpay/return", VNPayNativeReturnHandler(app, cfg))
	})
}

// MoMoNativeIPNHandler handles POST /v1/commerce/webhooks/momo (exported for legacy alias mounts).
func MoMoNativeIPNHandler(app *api.HTTPApplication, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid body", "resultCode": 1001})
			return
		}
		prov, ok := app.PaymentProviders.Get("momo").(*platformpayments.MoMoProvider)
		if !ok || prov == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"message": "momo not configured", "resultCode": 1001})
			return
		}
		_, status, _, event, err := prov.VerifyAndParseIPN(body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": err.Error(), "resultCode": 1001})
			return
		}
		applied, applyErr := applyNativePSPWebhook(r.Context(), app, cfg, event, status)
		if applyErr != nil {
			if errors.Is(applyErr, pgx.ErrNoRows) {
				writeJSON(w, http.StatusOK, map[string]any{"message": "order not found", "resultCode": 1001})
				return
			}
			observability.LoggerFromContext(r.Context(), zap.NewNop()).Warn("momo native ipn apply failed", zap.Error(applyErr))
			writeJSON(w, http.StatusOK, map[string]any{"message": "apply failed", "resultCode": 1001})
			return
		}
		_ = applied
		writeJSON(w, http.StatusOK, map[string]any{"message": "Success", "resultCode": 0})
	}
}

// ZaloPayNativeCallbackHandler handles POST /v1/commerce/webhooks/zalopay.
func ZaloPayNativeCallbackHandler(app *api.HTTPApplication, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"return_code": -1, "return_message": "invalid body"})
			return
		}
		prov, ok := app.PaymentProviders.Get("zalopay").(*platformpayments.ZaloPayProvider)
		if !ok || prov == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"return_code": -1, "return_message": "zalopay not configured"})
			return
		}
		_, status, _, event, err := prov.VerifyAndParseCallback(body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"return_code": -1, "return_message": err.Error()})
			return
		}
		_, applyErr := applyNativePSPWebhook(r.Context(), app, cfg, event, status)
		if applyErr != nil {
			if errors.Is(applyErr, pgx.ErrNoRows) {
				writeJSON(w, http.StatusOK, map[string]any{"return_code": -1, "return_message": "order not found"})
				return
			}
			observability.LoggerFromContext(r.Context(), zap.NewNop()).Warn("zalopay native callback apply failed", zap.Error(applyErr))
			writeJSON(w, http.StatusOK, map[string]any{"return_code": -1, "return_message": "apply failed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"return_code": 1, "return_message": "success"})
	}
}

// ShopeePayNativeCallbackHandler handles POST /v1/commerce/webhooks/shopeepay.
func ShopeePayNativeCallbackHandler(app *api.HTTPApplication, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		softACK := func() {
			writeJSON(w, http.StatusOK, map[string]any{"errcode": 0, "debug_msg": "success"})
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			softACK()
			return
		}
		prov, ok := app.PaymentProviders.Get("shopeepay").(*platformpayments.ShopeePayProvider)
		if !ok || prov == nil {
			softACK()
			return
		}
		clientID := strings.TrimSpace(r.Header.Get("X-Airpay-ClientId"))
		signature := strings.TrimSpace(r.Header.Get("X-Airpay-Req-H"))
		_, status, _, event, err := prov.VerifyAndParseCallback(body, clientID, signature, clientIP(r))
		if err != nil {
			observability.LoggerFromContext(r.Context(), zap.NewNop()).Warn("shopeepay native callback verify failed", zap.Error(err))
			softACK()
			return
		}
		_, applyErr := applyNativePSPWebhook(r.Context(), app, cfg, event, status)
		if applyErr != nil && !errors.Is(applyErr, pgx.ErrNoRows) {
			observability.LoggerFromContext(r.Context(), zap.NewNop()).Warn("shopeepay native callback apply failed", zap.Error(applyErr))
		}
		softACK()
	}
}

// VNPayNativeReturnHandler handles GET /v1/commerce/webhooks/vnpay/return.
func VNPayNativeReturnHandler(app *api.HTTPApplication, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		prov, ok := app.PaymentProviders.Get("vnpay").(*platformpayments.VNPayProvider)
		if !ok || prov == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"RspCode": "99", "Message": "vnpay not configured"})
			return
		}
		event, err := prov.ParseReturnQuery(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"RspCode": "99", "Message": err.Error()})
			return
		}
		_, applyErr := applyNativePSPWebhook(r.Context(), app, cfg, event, event.NormalizedPaymentState)
		if applyErr != nil {
			if errors.Is(applyErr, pgx.ErrNoRows) {
				writeJSON(w, http.StatusOK, map[string]any{"RspCode": "01", "Message": "Order not found"})
				return
			}
			observability.LoggerFromContext(r.Context(), zap.NewNop()).Warn("vnpay return apply failed", zap.Error(applyErr))
			writeJSON(w, http.StatusOK, map[string]any{"RspCode": "99", "Message": "apply failed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"RspCode": "00", "Message": "Confirm Success"})
	}
}

func applyNativePSPWebhook(ctx context.Context, app *api.HTTPApplication, cfg *config.Config, event platformpayments.CommerceWebhookEventJSON, normalizedState string) (bool, error) {
	if app == nil || app.Commerce == nil || app.TelemetryStore == nil {
		return false, errors.New("commerce webhook not configured")
	}
	ref := strings.TrimSpace(event.ProviderReference)
	if ref == "" {
		return false, errors.New("empty provider_reference")
	}
	row, err := app.TelemetryStore.GetPaymentByProviderReference(ctx, ref)
	if err != nil {
		return false, err
	}
	state := strings.ToLower(strings.TrimSpace(normalizedState))
	if state == "" {
		state = strings.ToLower(strings.TrimSpace(event.NormalizedPaymentState))
	}
	switch state {
	case "captured", "failed":
	default:
		// Pending / unknown: acknowledge without mutating payment state.
		return false, nil
	}
	in := appcommerce.ApplyPaymentProviderWebhookInput{
		OrderID:                 row.OrderID,
		PaymentID:               row.PaymentID,
		Provider:                strings.TrimSpace(event.Provider),
		ProviderReference:       ref,
		WebhookEventID:          strings.TrimSpace(event.WebhookEventID),
		EventType:               strings.TrimSpace(event.EventType),
		NormalizedPaymentState:  state,
		Payload:                 event.PayloadJSON,
		ProviderAmountMinor:     event.ProviderAmountMinor,
		Currency:                event.Currency,
		WebhookValidationStatus: "provider_native_verified",
		ProviderMetadata:        []byte(`{"delivery":{"mode":"provider_native"}}`),
	}
	if in.Provider == "" {
		in.Provider = row.Provider
	}
	if in.WebhookEventID == "" {
		in.WebhookEventID = ref
	}
	if in.EventType == "" {
		in.EventType = "provider.native"
	}
	attachPaymentWebhookOutbox(&in, cfg)
	_, err = app.Commerce.ApplyPaymentProviderWebhook(ctx, in)
	if err != nil {
		return false, err
	}
	return true, nil
}

// LookupPaymentByProviderReference is a thin alias for tests/handlers that already hold the store.
func LookupPaymentByProviderReference(ctx context.Context, store *postgres.Store, ref string) (postgres.PaymentByProviderRef, error) {
	if store == nil {
		return postgres.PaymentByProviderRef{}, errors.New("store not configured")
	}
	return store.GetPaymentByProviderReference(ctx, ref)
}
