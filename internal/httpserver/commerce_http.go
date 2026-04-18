package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/api"
	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func mountCommerceRoutes(r chi.Router, app *api.HTTPApplication, cfg *config.Config) {
	if app == nil || app.Commerce == nil || cfg == nil {
		return
	}
	svc := app.Commerce

	r.Route("/commerce", func(r chi.Router) {
		r.Use(auth.RequireOrganizationScope)

		r.Post("/orders", func(w http.ResponseWriter, r *http.Request) {
			idem, err := requireWriteIdempotencyKey(r)
			if err != nil {
				writeAPIError(w, http.StatusBadRequest, "missing_idempotency_key", err.Error())
				return
			}
			var body commerceCreateOrderRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeAPIError(w, http.StatusBadRequest, "invalid_json", "request body must be JSON")
				return
			}
			org := tenantOrgID(r)
			if org == uuid.Nil {
				writeAPIError(w, http.StatusForbidden, "organization_required", "organization scope required")
				return
			}
			in := appcommerce.CreateOrderInput{
				OrganizationID: org,
				MachineID:      body.MachineID,
				ProductID:      body.ProductID,
				SlotIndex:      body.SlotIndex,
				Currency:       body.Currency,
				SubtotalMinor:  body.SubtotalMinor,
				TaxMinor:       body.TaxMinor,
				TotalMinor:     body.TotalMinor,
				IdempotencyKey: idem,
			}
			out, err := svc.CreateOrder(r.Context(), in)
			if err != nil {
				writeCommerceServiceError(w, err)
				return
			}
			writeJSON(w, http.StatusCreated, commerceCreateOrderResponse{
				OrderID:       out.Order.ID.String(),
				VendSessionID: out.Vend.ID.String(),
				Replay:        out.Replay,
				OrderStatus:   out.Order.Status,
				VendState:     out.Vend.State,
			})
		})

		r.Post("/orders/{orderId}/payment-session", func(w http.ResponseWriter, r *http.Request) {
			idem, err := requireWriteIdempotencyKey(r)
			if err != nil {
				writeAPIError(w, http.StatusBadRequest, "missing_idempotency_key", err.Error())
				return
			}
			orderID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "orderId")))
			if err != nil {
				writeAPIError(w, http.StatusBadRequest, "invalid_order_id", "invalid orderId")
				return
			}
			var body commercePaymentSessionRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeAPIError(w, http.StatusBadRequest, "invalid_json", "request body must be JSON")
				return
			}
			org := tenantOrgID(r)
			if err := svc.EnsureOrderOrganization(r.Context(), org, orderID); err != nil {
				writeCommerceServiceError(w, err)
				return
			}
			topic := cfg.Commerce.PaymentOutboxTopic
			evType := cfg.Commerce.PaymentOutboxEventType
			aggType := cfg.Commerce.PaymentOutboxAggregateType
			if strings.TrimSpace(topic) == "" || strings.TrimSpace(evType) == "" || strings.TrimSpace(aggType) == "" {
				writeCapabilityNotConfigured(w, "v1.commerce.payment_session.outbox", "commerce outbox defaults are not configured")
				return
			}
			payload := body.OutboxPayloadJSON
			if len(payload) == 0 {
				payload = []byte(`{"source":"http_api"}`)
			}
			if !json.Valid(payload) {
				writeAPIError(w, http.StatusBadRequest, "invalid_json", "outbox_payload_json must be valid JSON")
				return
			}
			outboxIdem := idem + ":outbox:" + orderID.String()
			in := appcommerce.StartPaymentInput{
				OrganizationID:       org,
				OrderID:              orderID,
				Provider:             body.Provider,
				PaymentState:         firstNonEmpty(body.PaymentState, "created"),
				AmountMinor:          body.AmountMinor,
				Currency:             body.Currency,
				IdempotencyKey:       idem,
				OutboxTopic:          topic,
				OutboxEventType:      evType,
				OutboxPayload:        payload,
				OutboxAggregateType:  aggType,
				OutboxAggregateID:    orderID,
				OutboxIdempotencyKey: outboxIdem,
			}
			res, err := svc.StartPaymentWithOutbox(r.Context(), in)
			if err != nil {
				writeCommerceServiceError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, commercePaymentSessionResponse{
				PaymentID:     res.Payment.ID.String(),
				PaymentState:  res.Payment.State,
				OutboxEventID: res.Outbox.ID,
				Replay:        res.Replay,
			})
		})

		r.Get("/orders/{orderId}", func(w http.ResponseWriter, r *http.Request) {
			orderID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "orderId")))
			if err != nil {
				writeAPIError(w, http.StatusBadRequest, "invalid_order_id", "invalid orderId")
				return
			}
			slot := int32(parseSlotQuery(r, 0))
			org := tenantOrgID(r)
			st, err := svc.GetCheckoutStatus(r.Context(), org, orderID, slot)
			if err != nil {
				writeCommerceServiceError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, checkoutStatusToJSON(st))
		})

		r.Get("/orders/{orderId}/reconciliation", func(w http.ResponseWriter, r *http.Request) {
			orderID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "orderId")))
			if err != nil {
				writeAPIError(w, http.StatusBadRequest, "invalid_order_id", "invalid orderId")
				return
			}
			slot := int32(parseSlotQuery(r, 0))
			org := tenantOrgID(r)
			st, err := svc.GetCheckoutStatus(r.Context(), org, orderID, slot)
			if err != nil {
				writeCommerceServiceError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"kind":   "commerce.reconciliation_snapshot",
				"status": checkoutStatusToJSON(st),
			})
		})

		r.Post("/orders/{orderId}/payments/{paymentId}/webhooks", func(w http.ResponseWriter, r *http.Request) {
			if _, err := requireWriteIdempotencyKey(r); err != nil {
				writeAPIError(w, http.StatusBadRequest, "missing_idempotency_key", err.Error())
				return
			}
			orderID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "orderId")))
			if err != nil {
				writeAPIError(w, http.StatusBadRequest, "invalid_order_id", "invalid orderId")
				return
			}
			paymentID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "paymentId")))
			if err != nil {
				writeAPIError(w, http.StatusBadRequest, "invalid_payment_id", "invalid paymentId")
				return
			}
			var body commerceWebhookRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeAPIError(w, http.StatusBadRequest, "invalid_json", "request body must be JSON")
				return
			}
			org := tenantOrgID(r)
			in := appcommerce.ApplyPaymentProviderWebhookInput{
				OrganizationID:         org,
				OrderID:                orderID,
				PaymentID:              paymentID,
				Provider:               body.Provider,
				ProviderReference:      body.ProviderReference,
				EventType:              body.EventType,
				NormalizedPaymentState: body.NormalizedPaymentState,
				Payload:                body.PayloadJSON,
				ProviderAmountMinor:    body.ProviderAmountMinor,
				Currency:               body.Currency,
			}
			res, err := svc.ApplyPaymentProviderWebhook(r.Context(), in)
			if err != nil {
				writeCommerceServiceError(w, err)
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
		})

		r.Post("/orders/{orderId}/vend/start", func(w http.ResponseWriter, r *http.Request) {
			// Require Idempotency-Key on mutating routes for a uniform client contract; AdvanceVend
			// persistence does not yet key off this value (duplicate POSTs are rejected by state machine).
			if _, err := requireWriteIdempotencyKey(r); err != nil {
				writeAPIError(w, http.StatusBadRequest, "missing_idempotency_key", err.Error())
				return
			}
			orderID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "orderId")))
			if err != nil {
				writeAPIError(w, http.StatusBadRequest, "invalid_order_id", "invalid orderId")
				return
			}
			var body commerceVendStartRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeAPIError(w, http.StatusBadRequest, "invalid_json", "request body must be JSON")
				return
			}
			org := tenantOrgID(r)
			v, err := svc.AdvanceVend(r.Context(), appcommerce.AdvanceVendInput{
				OrganizationID: org,
				OrderID:        orderID,
				SlotIndex:      body.SlotIndex,
				ToState:        "in_progress",
			})
			if err != nil {
				writeCommerceServiceError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, commerceVendStateResponse{VendState: v.State, SlotIndex: v.SlotIndex})
		})

		r.Post("/orders/{orderId}/vend/success", func(w http.ResponseWriter, r *http.Request) {
			if _, err := requireWriteIdempotencyKey(r); err != nil {
				writeAPIError(w, http.StatusBadRequest, "missing_idempotency_key", err.Error())
				return
			}
			orderID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "orderId")))
			if err != nil {
				writeAPIError(w, http.StatusBadRequest, "invalid_order_id", "invalid orderId")
				return
			}
			var body commerceVendFinalizeRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeAPIError(w, http.StatusBadRequest, "invalid_json", "request body must be JSON")
				return
			}
			org := tenantOrgID(r)
			out, err := svc.FinalizeOrderAfterVend(r.Context(), appcommerce.FinalizeAfterVendInput{
				OrganizationID:    org,
				OrderID:           orderID,
				SlotIndex:         body.SlotIndex,
				TerminalVendState: "success",
				FailureReason:     nil,
			})
			if err != nil {
				writeCommerceServiceError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, commerceVendFinalizeResponse{
				OrderID:     out.Order.ID.String(),
				OrderStatus: out.Order.Status,
				VendState:   out.Vend.State,
			})
		})

		r.Post("/orders/{orderId}/vend/failure", func(w http.ResponseWriter, r *http.Request) {
			if _, err := requireWriteIdempotencyKey(r); err != nil {
				writeAPIError(w, http.StatusBadRequest, "missing_idempotency_key", err.Error())
				return
			}
			orderID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "orderId")))
			if err != nil {
				writeAPIError(w, http.StatusBadRequest, "invalid_order_id", "invalid orderId")
				return
			}
			var body commerceVendFinalizeRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeAPIError(w, http.StatusBadRequest, "invalid_json", "request body must be JSON")
				return
			}
			org := tenantOrgID(r)
			out, err := svc.FinalizeOrderAfterVend(r.Context(), appcommerce.FinalizeAfterVendInput{
				OrganizationID:    org,
				OrderID:           orderID,
				SlotIndex:         body.SlotIndex,
				TerminalVendState: "failed",
				FailureReason:     body.FailureReason,
			})
			if err != nil {
				writeCommerceServiceError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, commerceVendFinalizeResponse{
				OrderID:     out.Order.ID.String(),
				OrderStatus: out.Order.Status,
				VendState:   out.Vend.State,
			})
		})
	})
}

func tenantOrgID(r *http.Request) uuid.UUID {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return uuid.Nil
	}
	return p.OrganizationID
}

func parseSlotQuery(r *http.Request, def int) int {
	raw := strings.TrimSpace(r.URL.Query().Get("slot_index"))
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return def
	}
	return n
}

func checkoutStatusToJSON(st appcommerce.CheckoutStatusView) map[string]any {
	out := map[string]any{
		"order": map[string]any{
			"id":              st.Order.ID.String(),
			"organization_id": st.Order.OrganizationID.String(),
			"machine_id":      st.Order.MachineID.String(),
			"status":          st.Order.Status,
			"currency":        st.Order.Currency,
			"subtotal_minor":  st.Order.SubtotalMinor,
			"tax_minor":       st.Order.TaxMinor,
			"total_minor":     st.Order.TotalMinor,
			"created_at":      st.Order.CreatedAt,
			"updated_at":      st.Order.UpdatedAt,
		},
		"vend": map[string]any{
			"id":         st.Vend.ID.String(),
			"order_id":   st.Vend.OrderID.String(),
			"machine_id": st.Vend.MachineID.String(),
			"slot_index": st.Vend.SlotIndex,
			"product_id": st.Vend.ProductID.String(),
			"state":      st.Vend.State,
			"created_at": st.Vend.CreatedAt,
		},
	}
	if st.PaymentPresent {
		out["payment"] = map[string]any{
			"id":           st.Payment.ID.String(),
			"order_id":     st.Payment.OrderID.String(),
			"provider":     st.Payment.Provider,
			"state":        st.Payment.State,
			"amount_minor": st.Payment.AmountMinor,
			"currency":     st.Payment.Currency,
			"created_at":   st.Payment.CreatedAt,
		}
	} else {
		out["payment"] = nil
	}
	return out
}

func writeCommerceServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, appcommerce.ErrNotConfigured):
		writeCapabilityNotConfigured(w, "v1.commerce.persistence", "commerce persistence or webhook pipeline is not configured for this process")
	case errors.Is(err, appcommerce.ErrNotFound):
		writeAPIError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, appcommerce.ErrOrgMismatch):
		writeAPIError(w, http.StatusForbidden, "forbidden", "organization scope does not match this resource")
	case errors.Is(err, appcommerce.ErrIllegalTransition):
		writeAPIError(w, http.StatusConflict, "illegal_transition", err.Error())
	case errors.Is(err, appcommerce.ErrPaymentNotSettled):
		writeAPIError(w, http.StatusConflict, "payment_not_settled", err.Error())
	default:
		if errors.Is(err, appcommerce.ErrInvalidArgument) {
			writeAPIError(w, http.StatusBadRequest, "invalid_argument", err.Error())
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "internal", err.Error())
	}
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

// --- HTTP DTOs (request/response shapes only) ---

type commerceCreateOrderRequest struct {
	MachineID     uuid.UUID `json:"machine_id"`
	ProductID     uuid.UUID `json:"product_id"`
	SlotIndex     int32     `json:"slot_index"`
	Currency      string    `json:"currency"`
	SubtotalMinor int64     `json:"subtotal_minor"`
	TaxMinor      int64     `json:"tax_minor"`
	TotalMinor    int64     `json:"total_minor"`
}

type commerceCreateOrderResponse struct {
	OrderID       string `json:"order_id"`
	VendSessionID string `json:"vend_session_id"`
	Replay        bool   `json:"replay"`
	OrderStatus   string `json:"order_status"`
	VendState     string `json:"vend_state"`
}

type commercePaymentSessionRequest struct {
	Provider          string          `json:"provider"`
	PaymentState      string          `json:"payment_state"`
	AmountMinor       int64           `json:"amount_minor"`
	Currency          string          `json:"currency"`
	OutboxPayloadJSON json.RawMessage `json:"outbox_payload_json"`
}

type commercePaymentSessionResponse struct {
	PaymentID     string `json:"payment_id"`
	PaymentState  string `json:"payment_state"`
	OutboxEventID int64  `json:"outbox_event_id"`
	Replay        bool   `json:"replay"`
}

type commerceWebhookRequest struct {
	Provider               string          `json:"provider"`
	ProviderReference      string          `json:"provider_reference"`
	EventType              string          `json:"event_type"`
	NormalizedPaymentState string          `json:"normalized_payment_state"`
	PayloadJSON            json.RawMessage `json:"payload_json"`
	ProviderAmountMinor    *int64          `json:"provider_amount_minor"`
	Currency               *string         `json:"currency"`
}

type commerceWebhookResponse struct {
	Replay          bool   `json:"replay"`
	OrderID         string `json:"order_id"`
	OrderStatus     string `json:"order_status"`
	PaymentID       string `json:"payment_id"`
	PaymentState    string `json:"payment_state"`
	AttemptID       string `json:"attempt_id"`
	ProviderEventID int64  `json:"provider_event_id"`
}

type commerceVendStartRequest struct {
	SlotIndex int32 `json:"slot_index"`
}

type commerceVendFinalizeRequest struct {
	SlotIndex     int32   `json:"slot_index"`
	FailureReason *string `json:"failure_reason"`
}

type commerceVendStateResponse struct {
	VendState string `json:"vend_state"`
	SlotIndex int32  `json:"slot_index"`
}

type commerceVendFinalizeResponse struct {
	OrderID     string `json:"order_id"`
	OrderStatus string `json:"order_status"`
	VendState   string `json:"vend_state"`
}
