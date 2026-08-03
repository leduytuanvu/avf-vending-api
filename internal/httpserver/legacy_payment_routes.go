package httpserver

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/api"
	applegacypayment "github.com/avf/avf-vending-api/internal/app/legacypayment"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/observability"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// mountLegacyPaymentRoutes registers /payment-service/payment/* on the root router when enabled.
func mountLegacyPaymentRoutes(r chi.Router, app *api.HTTPApplication, cfg *config.Config, svc *applegacypayment.Service, abuse *AbuseProtection, writeRL func(http.Handler) http.Handler) {
	if r == nil || app == nil || cfg == nil || !cfg.TransportBoundary.LegacyPaymentHTTPEnabled {
		return
	}
	if writeRL == nil {
		writeRL = func(next http.Handler) http.Handler { return next }
	}
	if abuse == nil {
		abuse = &AbuseProtection{}
	}

	r.Route("/payment-service/payment", func(r chi.Router) {
		r.With(writeRL).Post("/create", legacyPaymentCreateHandler(svc))
		r.With(writeRL).Post("/query", legacyPaymentQueryHandler(svc))
		r.With(writeRL, abuse.WebhookPOST()).Post("/momo/callback", MoMoNativeIPNHandler(app, cfg))
		r.With(writeRL, abuse.WebhookPOST()).Post("/callback", ZaloPayNativeCallbackHandler(app, cfg))
		r.With(writeRL, abuse.WebhookPOST()).Post("/shopeepay/callback", ShopeePayNativeCallbackHandler(app, cfg))
		r.With(abuse.WebhookPOST()).Get("/vnpay_return", VNPayNativeReturnHandler(app, cfg))
		r.With(writeRL).Post("/refund", legacyPaymentNotImplemented("refund"))
		r.With(writeRL).Post("/query_refund", legacyPaymentNotImplemented("query_refund"))
		r.With(writeRL).Post("/update_delivery_status", legacyPaymentNoOp("update_delivery_status"))
		r.With(writeRL).Post("/synchronize", legacyPaymentNoOp("synchronize"))
	})
}

func legacyPaymentCreateHandler(svc *applegacypayment.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"code": 503, "message": "legacy payment not configured"})
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "invalid body"})
			return
		}
		var req applegacypayment.CreateSessionRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "invalid json"})
			return
		}
		res, err := svc.CreateSession(r.Context(), req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, applegacypayment.CreateSuccessEnvelope(res))
	}
}

func legacyPaymentQueryHandler(svc *applegacypayment.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"return_code": -1, "return_message": "not configured"})
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"return_code": -1, "return_message": "invalid body"})
			return
		}
		var req applegacypayment.QueryStatusRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"return_code": -1, "return_message": "invalid json"})
			return
		}
		code, msg, err := svc.QueryStatus(r.Context(), req)
		if err != nil && strings.Contains(msg, "not supported") {
			writeJSON(w, http.StatusNotImplemented, map[string]any{"return_code": -1, "return_message": msg})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"return_code": code, "return_message": msg})
	}
}

func legacyPaymentNotImplemented(op string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotImplemented, map[string]any{
			"code":    501,
			"message": op + " not implemented on legacy payment facade",
		})
	}
}

func legacyPaymentNoOp(op string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		observability.LoggerFromContext(r.Context(), zap.NewNop()).Info("legacy payment no-op",
			zap.String("op", op),
			zap.String("path", r.URL.Path),
		)
		writeJSON(w, http.StatusOK, map[string]any{
			"code":    200,
			"message": "Thành công",
			"data":    map[string]any{},
		})
	}
}
