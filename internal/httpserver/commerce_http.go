package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	platformpayments "github.com/avf/avf-vending-api/internal/platform/payments"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func mountCommerceRoutes(r chi.Router, app *api.HTTPApplication, cfg *config.Config) {
	// Legacy machine REST commerce (order/payment/vend HTTP). Production: disabled unless ENABLE_LEGACY_MACHINE_HTTP / MACHINE_REST_LEGACY_ENABLED; vending apps should use avf.machine.v1 gRPC.
	if app == nil || app.Commerce == nil || cfg == nil {
		return
	}
	svc := app.Commerce

	r.Route("/commerce", func(r chi.Router) {
		r.Use(auth.RequireOrganizationScope)

		r.With(auth.RequireInteractivePermissionOrMachinePrincipal(auth.PermCommerceRead)).Post("/orders", func(w http.ResponseWriter, r *http.Request) {
			idem, err := requireWriteIdempotencyKey(r)
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
				return
			}
			var body commerceCreateOrderRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
				return
			}
			org := tenantOrgID(r)
			if org == uuid.Nil {
				writeAPIError(w, r.Context(), http.StatusForbidden, "organization_required", "organization scope required")
				return
			}
			p, ok := auth.PrincipalFromContext(r.Context())
			if ok && p.HasRole(auth.RoleMachine) && !p.AllowsMachine(body.MachineID) {
				writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", "machine scope does not match order machine_id")
				return
			}
			in := appcommerce.CreateOrderInput{
				OrganizationID: org,
				MachineID:      body.MachineID,
				ProductID:      body.ProductID,
				SlotID:         body.SlotID,
				CabinetCode:    body.CabinetCode,
				SlotCode:       body.SlotCode,
				SlotIndex:      body.SlotIndex,
				Currency:       body.Currency,
				SubtotalMinor:  body.SubtotalMinor,
				TaxMinor:       body.TaxMinor,
				TotalMinor:     body.TotalMinor,
				IdempotencyKey: idem,
			}
			out, err := svc.CreateOrder(r.Context(), in)
			if err != nil {
				writeCommerceServiceError(w, r.Context(), err)
				return
			}
			slotID := ""
			if out.SaleLine.SlotConfigID != uuid.Nil {
				slotID = out.SaleLine.SlotConfigID.String()
			}
			writeJSON(w, http.StatusCreated, commerceCreateOrderResponse{
				OrderID:       out.Order.ID.String(),
				VendSessionID: out.Vend.ID.String(),
				Replay:        out.Replay,
				OrderStatus:   out.Order.Status,
				VendState:     out.Vend.State,
				SlotID:        slotID,
				CabinetCode:   out.SaleLine.CabinetCode,
				SlotCode:      out.SaleLine.SlotCode,
				SlotIndex:     out.SaleLine.SlotIndex,
				SubtotalMinor: out.Order.SubtotalMinor,
				TaxMinor:      out.Order.TaxMinor,
				TotalMinor:    out.Order.TotalMinor,
				PriceMinor:    out.SaleLine.PriceMinor,
			})
		})

		r.With(auth.RequireInteractivePermissionOrMachinePrincipal(auth.PermCommerceRead)).Post("/cash-checkout", func(w http.ResponseWriter, r *http.Request) {
			idem, err := requireWriteIdempotencyKey(r)
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
				return
			}
			var body commerceCashCheckoutRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
				return
			}
			org := tenantOrgID(r)
			if org == uuid.Nil {
				writeAPIError(w, r.Context(), http.StatusForbidden, "organization_required", "organization scope required")
				return
			}
			p, ok := auth.PrincipalFromContext(r.Context())
			if ok && p.HasRole(auth.RoleMachine) && !p.AllowsMachine(body.MachineID) {
				writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", "machine scope does not match order machine_id")
				return
			}
			topic := cfg.Commerce.PaymentOutboxTopic
			evType := cfg.Commerce.PaymentOutboxEventType
			aggType := cfg.Commerce.PaymentOutboxAggregateType
			if strings.TrimSpace(topic) == "" || strings.TrimSpace(evType) == "" || strings.TrimSpace(aggType) == "" {
				writeCapabilityNotConfigured(w, r.Context(), "v1.commerce.payment_session.outbox", "commerce outbox defaults are not configured")
				return
			}
			createOut, err := svc.CreateOrder(r.Context(), appcommerce.CreateOrderInput{
				OrganizationID: org,
				MachineID:      body.MachineID,
				ProductID:      body.ProductID,
				SlotID:         body.SlotID,
				CabinetCode:    body.CabinetCode,
				SlotCode:       body.SlotCode,
				SlotIndex:      body.SlotIndex,
				Currency:       body.Currency,
				SubtotalMinor:  body.SubtotalMinor,
				TaxMinor:       body.TaxMinor,
				TotalMinor:     body.TotalMinor,
				IdempotencyKey: idem,
			})
			if err != nil {
				writeCommerceServiceError(w, r.Context(), err)
				return
			}
			outboxIdem := idem + ":cash:payment:outbox:" + createOut.Order.ID.String()
			payRes, err := svc.StartPaymentWithOutbox(r.Context(), appcommerce.StartPaymentInput{
				OrganizationID:       org,
				OrderID:              createOut.Order.ID,
				Provider:             "cash",
				PaymentState:         "captured",
				AmountMinor:          createOut.Order.TotalMinor,
				Currency:             body.Currency,
				IdempotencyKey:       idem + ":cash:payment",
				OutboxTopic:          topic,
				OutboxEventType:      evType,
				OutboxPayload:        []byte(`{"source":"cash_checkout"}`),
				OutboxAggregateType:  aggType,
				OutboxAggregateID:    createOut.Order.ID,
				OutboxIdempotencyKey: outboxIdem,
			})
			if err != nil {
				writeCommerceServiceError(w, r.Context(), err)
				return
			}
			if _, err := svc.MarkOrderPaidAfterPaymentCapture(r.Context(), org, createOut.Order.ID); err != nil {
				writeCommerceServiceError(w, r.Context(), err)
				return
			}
			st, err := svc.GetCheckoutStatus(r.Context(), org, createOut.Order.ID, createOut.SaleLine.SlotIndex)
			if err != nil {
				writeCommerceServiceError(w, r.Context(), err)
				return
			}
			replay := createOut.Replay || payRes.Replay
			writeJSON(w, http.StatusOK, commerceCashCheckoutResponse{
				OrderID:       createOut.Order.ID.String(),
				VendSessionID: createOut.Vend.ID.String(),
				PaymentID:     payRes.Payment.ID.String(),
				OrderStatus:   st.Order.Status,
				PaymentState:  payRes.Payment.State,
				Replay:        replay,
			})
		})

		r.With(auth.RequireInteractivePermissionOrMachinePrincipal(auth.PermCommerceRead)).Post("/orders/{orderId}/payment-session", func(w http.ResponseWriter, r *http.Request) {
			idem, err := requireWriteIdempotencyKey(r)
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
				return
			}
			orderID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "orderId")))
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_order_id", "invalid orderId")
				return
			}
			var body commercePaymentSessionRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
				return
			}
			org := tenantOrgID(r)
			p, ok := auth.PrincipalFromContext(r.Context())
			if !ok {
				writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", "unauthenticated")
				return
			}
			if err := svc.EnsureCommerceCallerOrderAccess(r.Context(), org, orderID, p); err != nil {
				writeCommerceServiceError(w, r.Context(), err)
				return
			}
			topic := cfg.Commerce.PaymentOutboxTopic
			evType := cfg.Commerce.PaymentOutboxEventType
			aggType := cfg.Commerce.PaymentOutboxAggregateType
			if strings.TrimSpace(topic) == "" || strings.TrimSpace(evType) == "" || strings.TrimSpace(aggType) == "" {
				writeCapabilityNotConfigured(w, r.Context(), "v1.commerce.payment_session.outbox", "commerce outbox defaults are not configured")
				return
			}
			payload := body.OutboxPayloadJSON
			if len(payload) == 0 {
				payload = []byte(`{"source":"http_api"}`)
			}
			if !json.Valid(payload) {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "outbox_payload_json must be valid JSON")
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
				writeCommerceServiceError(w, r.Context(), err)
				return
			}
			slot := int32(parseSlotQuery(r, 0))
			st, err := svc.GetCheckoutStatus(r.Context(), org, orderID, slot)
			if err != nil {
				writeCommerceServiceError(w, r.Context(), err)
				return
			}
			view := appcommerce.BuildPaymentSessionKioskView(st, res, payload, idem)
			writeJSON(w, http.StatusOK, commercePaymentSessionResponseFromView(view))
		})

		r.With(auth.RequireInteractivePermissionOrMachinePrincipal(auth.PermCommerceRead)).Get("/orders/{orderId}", func(w http.ResponseWriter, r *http.Request) {
			orderID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "orderId")))
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_order_id", "invalid orderId")
				return
			}
			slot := int32(parseSlotQuery(r, 0))
			org := tenantOrgID(r)
			p, ok := auth.PrincipalFromContext(r.Context())
			if !ok {
				writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", "unauthenticated")
				return
			}
			if err := svc.EnsureCommerceCallerOrderAccess(r.Context(), org, orderID, p); err != nil {
				writeCommerceServiceError(w, r.Context(), err)
				return
			}
			st, err := svc.GetCheckoutStatus(r.Context(), org, orderID, slot)
			if err != nil {
				writeCommerceServiceError(w, r.Context(), err)
				return
			}
			writeJSON(w, http.StatusOK, checkoutStatusToJSON(st))
		})

		r.With(auth.RequireInteractivePermissionOrMachinePrincipal(auth.PermCommerceRead)).Get("/orders/{orderId}/reconciliation", func(w http.ResponseWriter, r *http.Request) {
			orderID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "orderId")))
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_order_id", "invalid orderId")
				return
			}
			slot := int32(parseSlotQuery(r, 0))
			org := tenantOrgID(r)
			p, ok := auth.PrincipalFromContext(r.Context())
			if !ok {
				writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", "unauthenticated")
				return
			}
			if err := svc.EnsureCommerceCallerOrderAccess(r.Context(), org, orderID, p); err != nil {
				writeCommerceServiceError(w, r.Context(), err)
				return
			}
			st, err := svc.GetCheckoutStatus(r.Context(), org, orderID, slot)
			if err != nil {
				writeCommerceServiceError(w, r.Context(), err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"kind":   "commerce.reconciliation_snapshot",
				"status": checkoutStatusToJSON(st),
			})
		})

		r.With(auth.RequireInteractivePermissionOrMachinePrincipal(auth.PermCommerceRead)).Post("/orders/{orderId}/vend/start", func(w http.ResponseWriter, r *http.Request) {
			// Require Idempotency-Key on mutating routes for a uniform client contract; AdvanceVend
			// persistence does not yet key off this value (duplicate POSTs are rejected by state machine).
			if _, err := requireWriteIdempotencyKey(r); err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
				return
			}
			orderID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "orderId")))
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_order_id", "invalid orderId")
				return
			}
			var body commerceVendStartRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
				return
			}
			org := tenantOrgID(r)
			p, ok := auth.PrincipalFromContext(r.Context())
			if !ok {
				writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", "unauthenticated")
				return
			}
			if err := svc.EnsureCommerceCallerOrderAccess(r.Context(), org, orderID, p); err != nil {
				writeCommerceServiceError(w, r.Context(), err)
				return
			}
			v, err := svc.AdvanceVend(r.Context(), appcommerce.AdvanceVendInput{
				OrganizationID: org,
				OrderID:        orderID,
				SlotIndex:      body.SlotIndex,
				ToState:        "in_progress",
			})
			if err != nil {
				writeCommerceServiceError(w, r.Context(), err)
				return
			}
			writeJSON(w, http.StatusOK, commerceVendStateResponse{VendState: v.State, SlotIndex: v.SlotIndex})
		})

		r.With(auth.RequireInteractivePermissionOrMachinePrincipal(auth.PermCommerceRead)).Post("/orders/{orderId}/vend/success", func(w http.ResponseWriter, r *http.Request) {
			idem, err := requireWriteIdempotencyKey(r)
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
				return
			}
			orderID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "orderId")))
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_order_id", "invalid orderId")
				return
			}
			var body commerceVendFinalizeRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
				return
			}
			org := tenantOrgID(r)
			p, ok := auth.PrincipalFromContext(r.Context())
			if !ok {
				writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", "unauthenticated")
				return
			}
			if err := svc.EnsureCommerceCallerOrderAccess(r.Context(), org, orderID, p); err != nil {
				writeCommerceServiceError(w, r.Context(), err)
				return
			}
			out, err := svc.FinalizeOrderAfterVend(r.Context(), appcommerce.FinalizeAfterVendInput{
				OrganizationID:            org,
				OrderID:                   orderID,
				SlotIndex:                 body.SlotIndex,
				TerminalVendState:         "success",
				FailureReason:             nil,
				ClientWriteIdempotencyKey: idem,
			})
			if err != nil {
				writeCommerceServiceError(w, r.Context(), err)
				return
			}
			writeJSON(w, http.StatusOK, commerceVendFinalizeResponse{
				OrderID:     out.Order.ID.String(),
				OrderStatus: out.Order.Status,
				VendState:   out.Vend.State,
			})
		})

		r.With(auth.RequireInteractivePermissionOrMachinePrincipal(auth.PermRefundsWrite, auth.PermInventoryAdjust)).Post("/orders/{orderId}/cancel", func(w http.ResponseWriter, r *http.Request) {
			idem, err := requireWriteIdempotencyKey(r)
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
				return
			}
			_ = idem
			orderID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "orderId")))
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_order_id", "invalid orderId")
				return
			}
			var body commerceCancelRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
				return
			}
			org := tenantOrgID(r)
			p, ok := auth.PrincipalFromContext(r.Context())
			if !ok {
				writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", "unauthenticated")
				return
			}
			if err := svc.EnsureCommerceCallerOrderAccess(r.Context(), org, orderID, p); err != nil {
				writeCommerceServiceError(w, r.Context(), err)
				return
			}
			o, err := svc.CancelOrder(r.Context(), org, orderID, body.Reason)
			if err != nil {
				writeCommerceServiceError(w, r.Context(), err)
				return
			}
			writeJSON(w, http.StatusOK, commerceCancelResponse{
				OrderID:      o.ID.String(),
				OrderStatus:  o.Status,
				PaymentState: "none",
				RefundState:  "not_required",
				Replay:       false,
			})
		})

		r.With(auth.RequireInteractivePermissionOrMachinePrincipal(auth.PermRefundsWrite)).Post("/orders/{orderId}/refunds", func(w http.ResponseWriter, r *http.Request) {
			idem, err := requireWriteIdempotencyKey(r)
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
				return
			}
			orderID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "orderId")))
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_order_id", "invalid orderId")
				return
			}
			var body commerceRefundRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
				return
			}
			org := tenantOrgID(r)
			p, ok := auth.PrincipalFromContext(r.Context())
			if !ok {
				writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", "unauthenticated")
				return
			}
			if err := svc.EnsureCommerceCallerOrderAccess(r.Context(), org, orderID, p); err != nil {
				writeCommerceServiceError(w, r.Context(), err)
				return
			}
			meta := map[string]any{}
			if body.Metadata != nil {
				meta = body.Metadata
			}
			metaJSON, _ := json.Marshal(meta)
			ref, err := svc.CreateRefund(r.Context(), appcommerce.CreateRefundInput{
				OrganizationID: org,
				OrderID:        orderID,
				AmountMinor:    body.AmountMinor,
				Currency:       body.Currency,
				Reason:         body.Reason,
				IdempotencyKey: idem,
				Metadata:       metaJSON,
			})
			if err != nil {
				writeCommerceServiceError(w, r.Context(), err)
				return
			}
			writeJSON(w, http.StatusOK, commerceRefundResponse{
				RefundID:    ref.ID.String(),
				OrderID:     orderID.String(),
				PaymentID:   ref.PaymentID.String(),
				RefundState: refundClientState(ref.State),
				AmountMinor: ref.AmountMinor,
				Currency:    ref.Currency,
				Replay:      false,
			})
		})

		r.With(auth.RequireInteractivePermissionOrMachinePrincipal(auth.PermCommerceRead)).Get("/orders/{orderId}/refunds", func(w http.ResponseWriter, r *http.Request) {
			orderID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "orderId")))
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_order_id", "invalid orderId")
				return
			}
			org := tenantOrgID(r)
			p, ok := auth.PrincipalFromContext(r.Context())
			if !ok {
				writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", "unauthenticated")
				return
			}
			if err := svc.EnsureCommerceCallerOrderAccess(r.Context(), org, orderID, p); err != nil {
				writeCommerceServiceError(w, r.Context(), err)
				return
			}
			rows, err := svc.ListRefundsForOrder(r.Context(), org, orderID)
			if err != nil {
				writeCommerceServiceError(w, r.Context(), err)
				return
			}
			items := make([]map[string]any, 0, len(rows))
			for _, row := range rows {
				items = append(items, map[string]any{
					"refund_id":    row.ID.String(),
					"order_id":     row.OrderID.String(),
					"payment_id":   row.PaymentID.String(),
					"refund_state": refundClientState(row.State),
					"amount_minor": row.AmountMinor,
					"currency":     row.Currency,
					"created_at":   row.CreatedAt,
				})
			}
			writeJSON(w, http.StatusOK, map[string]any{"items": items})
		})

		r.With(auth.RequireInteractivePermissionOrMachinePrincipal(auth.PermCommerceRead)).Get("/orders/{orderId}/refunds/{refundId}", func(w http.ResponseWriter, r *http.Request) {
			orderID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "orderId")))
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_order_id", "invalid orderId")
				return
			}
			refundID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "refundId")))
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_refund_id", "invalid refundId")
				return
			}
			org := tenantOrgID(r)
			p, ok := auth.PrincipalFromContext(r.Context())
			if !ok {
				writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", "unauthenticated")
				return
			}
			if err := svc.EnsureCommerceCallerOrderAccess(r.Context(), org, orderID, p); err != nil {
				writeCommerceServiceError(w, r.Context(), err)
				return
			}
			row, err := svc.GetRefundForOrder(r.Context(), org, orderID, refundID)
			if err != nil {
				writeCommerceServiceError(w, r.Context(), err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"refund_id":    row.ID.String(),
				"order_id":     row.OrderID.String(),
				"payment_id":   row.PaymentID.String(),
				"refund_state": refundClientState(row.State),
				"amount_minor": row.AmountMinor,
				"currency":     row.Currency,
				"created_at":   row.CreatedAt,
			})
		})

		r.With(auth.RequireInteractivePermissionOrMachinePrincipal(auth.PermCommerceRead)).Post("/orders/{orderId}/vend/failure", func(w http.ResponseWriter, r *http.Request) {
			if _, err := requireWriteIdempotencyKey(r); err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
				return
			}
			orderID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "orderId")))
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_order_id", "invalid orderId")
				return
			}
			var body commerceVendFinalizeRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
				return
			}
			org := tenantOrgID(r)
			p, ok := auth.PrincipalFromContext(r.Context())
			if !ok {
				writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", "unauthenticated")
				return
			}
			if err := svc.EnsureCommerceCallerOrderAccess(r.Context(), org, orderID, p); err != nil {
				writeCommerceServiceError(w, r.Context(), err)
				return
			}
			out, err := svc.FinalizeOrderAfterVend(r.Context(), appcommerce.FinalizeAfterVendInput{
				OrganizationID:    org,
				OrderID:           orderID,
				SlotIndex:         body.SlotIndex,
				TerminalVendState: "failed",
				FailureReason:     body.FailureReason,
			})
			if err != nil {
				writeCommerceServiceError(w, r.Context(), err)
				return
			}
			st, _ := svc.GetCheckoutStatus(r.Context(), org, orderID, body.SlotIndex)
			resp := map[string]any{
				"order_id":     out.Order.ID.String(),
				"order_status": out.Order.Status,
				"vend_state":   out.Vend.State,
			}
			if st.PaymentPresent && strings.EqualFold(st.Payment.Provider, "cash") && st.Payment.State == "captured" {
				resp["local_cash_refund_required"] = true
			} else if st.PaymentPresent && st.Payment.State == "captured" {
				resp["refund_required"] = true
			}
			writeJSON(w, http.StatusOK, resp)
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

func writeCommerceServiceError(w http.ResponseWriter, ctx context.Context, err error) {
	switch {
	case errors.Is(err, appcommerce.ErrNotConfigured):
		writeCapabilityNotConfigured(w, ctx, "v1.commerce.persistence", "commerce persistence or webhook pipeline is not configured for this process")
	case errors.Is(err, appcommerce.ErrNotFound):
		writeAPIError(w, ctx, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, appcommerce.ErrOrgMismatch):
		writeAPIError(w, ctx, http.StatusForbidden, "forbidden", "organization scope does not match this resource")
	case errors.Is(err, appcommerce.ErrWebhookAmountCurrencyMismatch):
		writeAPIError(w, ctx, http.StatusConflict, "webhook_amount_currency_mismatch", err.Error())
	case errors.Is(err, appcommerce.ErrWebhookAfterTerminalOrder):
		writeAPIError(w, ctx, http.StatusConflict, "webhook_after_terminal_order", err.Error())
	case errors.Is(err, appcommerce.ErrIllegalTransition):
		writeAPIError(w, ctx, http.StatusConflict, "illegal_transition", err.Error())
	case errors.Is(err, appcommerce.ErrWebhookIdempotencyConflict):
		writeAPIError(w, ctx, http.StatusConflict, "webhook_idempotency_conflict", "webhook replay does not match stored provider_reference or webhook_event_id")
	case errors.Is(err, appcommerce.ErrWebhookProviderMismatch):
		writeAPIError(w, ctx, http.StatusForbidden, "webhook_provider_mismatch", "webhook provider does not match payment provider")
	case errors.Is(err, appcommerce.ErrIdempotencyPayloadConflict):
		writeAPIError(w, ctx, http.StatusConflict, "idempotency_payload_conflict", err.Error())
	case errors.Is(err, appcommerce.ErrPaymentNotSettled):
		writeAPIError(w, ctx, http.StatusConflict, "payment_not_settled", err.Error())
	case errors.Is(err, appcommerce.ErrCancelNotAllowed):
		writeAPIError(w, ctx, http.StatusConflict, "cancel_not_allowed", err.Error())
	case errors.Is(err, appcommerce.ErrRefundNotAllowed):
		writeAPIError(w, ctx, http.StatusConflict, "refund_not_allowed", err.Error())
	default:
		if errors.Is(err, appcommerce.ErrInvalidArgument) {
			writeAPIError(w, ctx, http.StatusBadRequest, "invalid_argument", err.Error())
			return
		}
		writeAPIError(w, ctx, http.StatusInternalServerError, "internal", err.Error())
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
	MachineID   uuid.UUID  `json:"machine_id"`
	ProductID   uuid.UUID  `json:"product_id"`
	SlotID      *uuid.UUID `json:"slot_id,omitempty"`
	CabinetCode string     `json:"cabinet_code,omitempty"`
	SlotCode    string     `json:"slot_code,omitempty"`
	// SlotIndex is deprecated; prefer slot_id or cabinet_code+slot_code.
	SlotIndex     *int32 `json:"slot_index,omitempty"`
	Currency      string `json:"currency"`
	SubtotalMinor int64  `json:"subtotal_minor,omitempty"`
	TaxMinor      int64  `json:"tax_minor,omitempty"`
	TotalMinor    int64  `json:"total_minor,omitempty"`
}

// commerceCashCheckoutRequest mirrors create-order slot selection; records a captured cash payment and marks the order paid.
type commerceCashCheckoutRequest struct {
	MachineID     uuid.UUID  `json:"machine_id"`
	ProductID     uuid.UUID  `json:"product_id"`
	SlotID        *uuid.UUID `json:"slot_id,omitempty"`
	CabinetCode   string     `json:"cabinet_code,omitempty"`
	SlotCode      string     `json:"slot_code,omitempty"`
	SlotIndex     *int32     `json:"slot_index,omitempty"`
	Currency      string     `json:"currency"`
	SubtotalMinor int64      `json:"subtotal_minor,omitempty"`
	TaxMinor      int64      `json:"tax_minor,omitempty"`
	TotalMinor    int64      `json:"total_minor,omitempty"`
}

type commerceCashCheckoutResponse struct {
	OrderID       string `json:"order_id"`
	VendSessionID string `json:"vend_session_id"`
	PaymentID     string `json:"payment_id"`
	OrderStatus   string `json:"order_status"`
	PaymentState  string `json:"payment_state"`
	Replay        bool   `json:"replay"`
}

type commerceCreateOrderResponse struct {
	OrderID       string `json:"order_id"`
	VendSessionID string `json:"vend_session_id"`
	Replay        bool   `json:"replay"`
	OrderStatus   string `json:"order_status"`
	VendState     string `json:"vend_state"`
	SlotID        string `json:"slot_id"`
	CabinetCode   string `json:"cabinet_code"`
	SlotCode      string `json:"slot_code"`
	SlotIndex     int32  `json:"slot_index"`
	SubtotalMinor int64  `json:"subtotal_minor"`
	TaxMinor      int64  `json:"tax_minor"`
	TotalMinor    int64  `json:"total_minor"`
	PriceMinor    int64  `json:"price_minor"`
}

type commercePaymentSessionRequest struct {
	Provider          string          `json:"provider"`
	PaymentState      string          `json:"payment_state"`
	AmountMinor       int64           `json:"amount_minor"`
	Currency          string          `json:"currency"`
	OutboxPayloadJSON json.RawMessage `json:"outbox_payload_json"`
}

type commercePaymentSessionResponse struct {
	SaleID         string     `json:"sale_id"`
	SessionID      string     `json:"session_id"`
	PaymentID      string     `json:"payment_id"`
	AmountMinor    int64      `json:"amount_minor"`
	Currency       string     `json:"currency"`
	Provider       string     `json:"provider"`
	Status         string     `json:"status"`
	PaymentState   string     `json:"payment_state"`
	QrURL          *string    `json:"qr_url,omitempty"`
	PaymentURL     *string    `json:"payment_url,omitempty"`
	CheckoutURL    *string    `json:"checkout_url,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	IdempotencyKey string     `json:"idempotency_key"`
	RequestID      string     `json:"request_id"`
	OutboxEventID  int64      `json:"outbox_event_id"`
	Replay         bool       `json:"replay"`
}

func commercePaymentSessionResponseFromView(v appcommerce.PaymentSessionKioskView) commercePaymentSessionResponse {
	return commercePaymentSessionResponse{
		SaleID:         v.SaleID,
		SessionID:      v.SessionID,
		PaymentID:      v.PaymentID,
		AmountMinor:    v.AmountMinor,
		Currency:       v.Currency,
		Provider:       v.Provider,
		Status:         v.Status,
		PaymentState:   v.PaymentState,
		QrURL:          v.QrURL,
		PaymentURL:     v.PaymentURL,
		CheckoutURL:    v.CheckoutURL,
		ExpiresAt:      v.ExpiresAt,
		IdempotencyKey: v.IdempotencyKey,
		RequestID:      v.RequestID,
		OutboxEventID:  v.OutboxEventID,
		Replay:         v.Replay,
	}
}

type commerceWebhookRequest = platformpayments.CommerceWebhookEventJSON

type commerceWebhookResponse struct {
	Replay          bool   `json:"replay"`
	OrderID         string `json:"order_id"`
	OrderStatus     string `json:"order_status"`
	PaymentID       string `json:"payment_id"`
	PaymentState    string `json:"payment_state"`
	AttemptID       string `json:"attempt_id"`
	ProviderEventID int64  `json:"provider_event_id"`
}

type commerceCancelRequest struct {
	Reason    string `json:"reason"`
	SlotIndex *int32 `json:"slot_index,omitempty"`
}

type commerceCancelResponse struct {
	OrderID      string `json:"order_id"`
	OrderStatus  string `json:"order_status"`
	PaymentState string `json:"payment_state"`
	RefundState  string `json:"refund_state"`
	Replay       bool   `json:"replay"`
}

type commerceRefundRequest struct {
	Reason      string         `json:"reason"`
	AmountMinor int64          `json:"amount_minor"`
	Currency    string         `json:"currency"`
	Metadata    map[string]any `json:"metadata"`
}

type commerceRefundResponse struct {
	RefundID    string `json:"refund_id"`
	OrderID     string `json:"order_id"`
	PaymentID   string `json:"payment_id"`
	RefundState string `json:"refund_state"`
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
	Replay      bool   `json:"replay"`
}

func refundClientState(dbState string) string {
	if dbState == "requested" {
		return "pending"
	}
	return dbState
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
