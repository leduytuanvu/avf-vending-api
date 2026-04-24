package httpserver

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// mountCommercePublicWebhookPost registers PSP callbacks on /v1 without Bearer JWT (HMAC-only when configured).
func mountCommercePublicWebhookPost(r chi.Router, app *api.HTTPApplication, cfg *config.Config) {
	if app == nil || app.Commerce == nil || cfg == nil || app.TelemetryStore == nil {
		return
	}
	r.Post("/commerce/orders/{orderId}/payments/{paymentId}/webhooks", commercePublicPaymentWebhookHandler(app, cfg))
}

func verifyCommerceWebhookHMAC(secret, tsHeader, sigHeader string, body []byte, skew time.Duration) error {
	sigHeader = strings.TrimSpace(sigHeader)
	tsHeader = strings.TrimSpace(tsHeader)
	if sigHeader == "" || tsHeader == "" {
		return errors.New("missing X-AVF-Webhook-Timestamp or X-AVF-Webhook-Signature")
	}
	ts, err := strconv.ParseInt(tsHeader, 10, 64)
	if err != nil {
		return errors.New("invalid X-AVF-Webhook-Timestamp")
	}
	skewSec := int64(skew / time.Second)
	if skewSec < 1 {
		skewSec = 300
	}
	now := time.Now().Unix()
	if ts > now+skewSec || ts < now-skewSec {
		return errors.New("webhook timestamp outside allowed skew")
	}
	tsStr := strconv.FormatInt(ts, 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(tsStr))
	mac.Write([]byte{'.'})
	mac.Write(body)
	sum := mac.Sum(nil)

	sigHex := strings.TrimPrefix(sigHeader, "sha256=")
	sigHex = strings.TrimSpace(sigHex)
	want, err := hex.DecodeString(sigHex)
	if err != nil || len(want) != len(sum) || !hmac.Equal(sum, want) {
		return errors.New("invalid webhook signature")
	}
	return nil
}

func commercePublicPaymentWebhookHandler(app *api.HTTPApplication, cfg *config.Config) http.HandlerFunc {
	svc := app.Commerce
	return func(w http.ResponseWriter, r *http.Request) {
		secret := strings.TrimSpace(cfg.Commerce.PaymentWebhookHMACSecret)
		if secret == "" {
			switch cfg.AppEnv {
			case config.AppEnvProduction:
				if !cfg.Commerce.PaymentWebhookUnsafeAllowUnsignedProduction {
					writeAPIError(w, r.Context(), http.StatusForbidden, "webhook_hmac_required",
						"payment webhooks require COMMERCE_PAYMENT_WEBHOOK_HMAC_SECRET in production; unsigned delivery is rejected unless COMMERCE_PAYMENT_WEBHOOK_UNSAFE_ALLOW_UNSIGNED_PRODUCTION=true (documented unsafe)")
					return
				}
			default:
				if !cfg.Commerce.PaymentWebhookAllowUnsigned {
					writeCapabilityNotConfigured(w, r.Context(), "v1.commerce.payment_webhook.hmac",
						"set COMMERCE_PAYMENT_WEBHOOK_HMAC_SECRET or COMMERCE_PAYMENT_WEBHOOK_ALLOW_UNSIGNED=true (non-production only)")
					return
				}
			}
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_body", "could not read body")
			return
		}
		if secret != "" {
			if err := verifyCommerceWebhookHMAC(secret, r.Header.Get("X-AVF-Webhook-Timestamp"), r.Header.Get("X-AVF-Webhook-Signature"), body, cfg.Commerce.PaymentWebhookTimestampSkew); err != nil {
				if strings.Contains(strings.ToLower(err.Error()), "timestamp") {
					writeAPIError(w, r.Context(), http.StatusBadRequest, "webhook_timestamp_skew", err.Error())
					return
				}
				writeAPIError(w, r.Context(), http.StatusUnauthorized, "webhook_auth_failed", err.Error())
				return
			}
		}

		orderID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "orderId")))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_order_id", "invalid orderId")
			return
		}
		paymentID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "paymentId")))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_payment_id", "invalid paymentId")
			return
		}

		q := db.New(app.TelemetryStore.Pool())
		pay, err := q.GetPaymentByID(r.Context(), paymentID)
		if err != nil {
			writeCommerceStoreError(w, r, err)
			return
		}
		if pay.OrderID != orderID {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "payment_order_mismatch", "payment does not belong to order")
			return
		}
		ord, err := q.GetOrderByID(r.Context(), orderID)
		if err != nil {
			writeCommerceStoreError(w, r, err)
			return
		}

		var wh commerceWebhookRequest
		if err := json.Unmarshal(body, &wh); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		in := appcommerce.ApplyPaymentProviderWebhookInput{
			OrganizationID:         ord.OrganizationID,
			OrderID:                orderID,
			PaymentID:              paymentID,
			Provider:               wh.Provider,
			ProviderReference:      wh.ProviderReference,
			WebhookEventID:         wh.WebhookEventID,
			EventType:              wh.EventType,
			NormalizedPaymentState: wh.NormalizedPaymentState,
			Payload:                wh.PayloadJSON,
			ProviderAmountMinor:    wh.ProviderAmountMinor,
			Currency:               wh.Currency,
		}
		res, err := svc.ApplyPaymentProviderWebhook(r.Context(), in)
		if err != nil {
			writeCommerceServiceError(w, r.Context(), err)
			return
		}
		resp := commerceWebhookResponse{
			Replay:          res.Replay,
			OrderID:         res.Order.ID.String(),
			OrderStatus:     res.Order.Status,
			PaymentID:       res.Payment.ID.String(),
			PaymentState:    res.Payment.State,
			ProviderEventID: res.ProviderRowID,
		}
		if res.Attempt.ID != uuid.Nil {
			resp.AttemptID = res.Attempt.ID.String()
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func writeCommerceStoreError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "resource not found")
		return
	}
	writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
}
