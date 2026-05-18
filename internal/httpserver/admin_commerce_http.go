package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	appcommerceadmin "github.com/avf/avf-vending-api/internal/app/commerceadmin"
	"github.com/avf/avf-vending-api/internal/app/listscope"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func parseCompanyCommerceListScope(r *http.Request) (listscope.CompanyCommerce, error) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return listscope.CompanyCommerce{}, listscope.ErrInvalidListQuery
	}
	limit, offset, err := parseAdminLimitOffset(r)
	if err != nil {
		return listscope.CompanyCommerce{}, listscope.ErrInvalidListQuery
	}
	q := r.URL.Query()
	var machineID *uuid.UUID
	if raw := strings.TrimSpace(q.Get("machine_id")); raw != "" {
		mid, perr := uuid.Parse(raw)
		if perr != nil || mid == uuid.Nil {
			return listscope.CompanyCommerce{}, listscope.ErrInvalidListQuery
		}
		machineID = &mid
	}
	var from *time.Time
	if raw := strings.TrimSpace(q.Get("from")); raw != "" {
		t, perr := time.Parse(time.RFC3339Nano, raw)
		if perr != nil {
			t, perr = time.Parse(time.RFC3339, raw)
		}
		if perr != nil {
			return listscope.CompanyCommerce{}, listscope.ErrInvalidListQuery
		}
		u := t.UTC()
		from = &u
	}
	var to *time.Time
	if raw := strings.TrimSpace(q.Get("to")); raw != "" {
		t, perr := time.Parse(time.RFC3339Nano, raw)
		if perr != nil {
			t, perr = time.Parse(time.RFC3339, raw)
		}
		if perr != nil {
			return listscope.CompanyCommerce{}, listscope.ErrInvalidListQuery
		}
		u := t.UTC()
		to = &u
	}
	return listscope.CompanyCommerce{IsPlatformAdmin: p.HasRole(auth.RolePlatformAdmin), Limit: limit, Offset: offset, Status: strings.TrimSpace(q.Get("status")), MachineID: machineID, PaymentMethod: strings.TrimSpace(q.Get("payment_method")), Search: strings.TrimSpace(q.Get("search")), From: from, To: to}, nil
}

func mountAdminCommerceRoutes(r chi.Router, app *api.HTTPApplication, writeRL func(http.Handler) http.Handler) {
	if app == nil || app.Reconciliation == nil {
		return
	}
	if writeRL == nil {
		writeRL = func(next http.Handler) http.Handler { return next }
	}
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermCommerceRead))
		r.Get("/commerce/reconciliation", listAdminCommerceReconciliation(app))
		r.Get("/commerce/reconciliation/{caseId}", getAdminCommerceReconciliation(app))
	})
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermCommerceRead, auth.PermPaymentRead))
		r.Get("/orders/{orderId}/timeline", listAdminCommerceOrderTimeline(app))
		r.Get("/refunds", listAdminCommerceRefundRequests(app))
		r.Get("/refunds/{refundId}", getAdminCommerceRefundRequest(app))
	})
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermRefundsWrite))
		r.With(writeRL).Post("/commerce/reconciliation/{caseId}/resolve", resolveAdminCommerceReconciliation(app))
		r.With(writeRL).Post("/commerce/reconciliation/{caseId}/ignore", ignoreAdminCommerceReconciliation(app))
		r.With(writeRL).Post("/commerce/reconciliation/{caseId}/request-refund", requestRefundAdminCommerceReconciliation(app))
		r.With(writeRL).Post("/orders/{orderId}/refunds", createAdminCommerceOrderRefund(app))
	})
}

func adminCommerceRequirePrincipal(r *http.Request) error {
	if _, ok := auth.PrincipalFromContext(r.Context()); !ok {
		return listscope.ErrInvalidListQuery
	}
	return nil
}

func parseAdminCommerceReconciliationScope(r *http.Request) (listscope.CompanyCommerce, error) {
	if err := adminCommerceRequirePrincipal(r); err != nil {
		return listscope.CompanyCommerce{}, err
	}
	limit, offset, err := parseAdminLimitOffset(r)
	if err != nil {
		return listscope.CompanyCommerce{}, listscope.ErrInvalidListQuery
	}
	q := r.URL.Query()
	return listscope.CompanyCommerce{
		Limit:    limit,
		Offset:   offset,
		Status:   strings.TrimSpace(q.Get("status")),
		CaseType: strings.TrimSpace(q.Get("case_type")),
	}, nil
}

func parseAdminRefundListScope(r *http.Request) (listscope.CompanyCommerce, error) {
	if err := adminCommerceRequirePrincipal(r); err != nil {
		return listscope.CompanyCommerce{}, err
	}
	limit, offset, err := parseAdminLimitOffset(r)
	if err != nil {
		return listscope.CompanyCommerce{}, listscope.ErrInvalidListQuery
	}
	q := r.URL.Query()
	return listscope.CompanyCommerce{
		Limit:  limit,
		Offset: offset,
		Status: strings.TrimSpace(q.Get("status")),
	}, nil
}

func parseAdminOrderTimelineParams(r *http.Request) (orderID uuid.UUID, limit int32, offset int32, err error) {
	if err := adminCommerceRequirePrincipal(r); err != nil {
		return uuid.Nil, 0, 0, err
	}
	orderID, err = uuid.Parse(strings.TrimSpace(chi.URLParam(r, "orderId")))
	if err != nil || orderID == uuid.Nil {
		return uuid.Nil, 0, 0, listscope.ErrInvalidListQuery
	}
	limit, offset, err = parseAdminLimitOffset(r)
	if err != nil {
		return uuid.Nil, 0, 0, listscope.ErrInvalidListQuery
	}
	return orderID, limit, offset, nil
}

func listAdminCommerceReconciliation(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scope, err := parseAdminCommerceReconciliationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		out, err := app.Reconciliation.ListReconciliationCases(r.Context(), scope)
		writeV1Collection(w, r.Context(), out, err)
	}
}

func getAdminCommerceReconciliation(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := adminCommerceRequirePrincipal(r); err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		caseID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "caseId")))
		if err != nil || caseID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_case_id", "invalid reconciliation case id")
			return
		}
		out, err := app.Reconciliation.GetReconciliationCase(r.Context(), caseID)
		if errors.Is(err, pgx.ErrNoRows) {
			writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "reconciliation case not found")
			return
		}
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func listAdminCommerceOrderTimeline(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orderID, limit, offset, err := parseAdminOrderTimelineParams(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		out, err := app.Reconciliation.ListOrderTimeline(r.Context(), orderID, limit, offset)
		if errors.Is(err, pgx.ErrNoRows) {
			writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "order not found for company")
			return
		}
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func listAdminCommerceRefundRequests(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scope, err := parseAdminRefundListScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		out, err := app.Reconciliation.ListRefundRequests(r.Context(), scope)
		writeV1Collection(w, r.Context(), out, err)
	}
}

func getAdminCommerceRefundRequest(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := adminCommerceRequirePrincipal(r); err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		refundRequestID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "refundId")))
		if err != nil || refundRequestID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_refund_id", "invalid refund request id")
			return
		}
		out, err := app.Reconciliation.GetRefundRequest(r.Context(), refundRequestID)
		if errors.Is(err, pgx.ErrNoRows) {
			writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "refund request not found")
			return
		}
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func resolveAdminCommerceReconciliation(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := adminCommerceRequirePrincipal(r); err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		caseID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "caseId")))
		if err != nil || caseID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_case_id", "invalid reconciliation case id")
			return
		}
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthorized", "missing principal")
			return
		}
		var body struct {
			Status string `json:"status"`
			Note   string `json:"note"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		actorID, _ := uuid.Parse(strings.TrimSpace(p.Subject))
		out, err := app.Reconciliation.ResolveReconciliationCase(r.Context(), appcommerceadmin.ResolveReconciliationInput{
			CaseID:     caseID,
			ResolvedBy: actorID,
			Status:     body.Status,
			Note:       body.Note,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "open reconciliation case not found")
			return
		}
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_argument", err.Error())
			return
		}
		if app.EnterpriseAudit != nil {
			cid := caseID.String()
			_ = app.EnterpriseAudit.Record(r.Context(), compliance.EnterpriseAuditRecord{
				ActorType:    compliance.ActorUser,
				ActorID:      &p.Subject,
				Action:       compliance.ActionCommerceReconciliationCaseResolved,
				ResourceType: "commerce.reconciliation_case",
				ResourceID:   &cid,
				Metadata:     compliance.SanitizeJSONBytes([]byte(`{"source":"admin_commerce_reconciliation_resolve"}`)),
			})
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func ignoreAdminCommerceReconciliation(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := adminCommerceRequirePrincipal(r); err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		caseID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "caseId")))
		if err != nil || caseID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_case_id", "invalid reconciliation case id")
			return
		}
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthorized", "missing principal")
			return
		}
		var body struct {
			Note string `json:"note"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		actorID, _ := uuid.Parse(strings.TrimSpace(p.Subject))
		out, err := app.Reconciliation.ResolveReconciliationCase(r.Context(), appcommerceadmin.ResolveReconciliationInput{
			CaseID:     caseID,
			ResolvedBy: actorID,
			Status:     "ignored",
			Note:       body.Note,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "open reconciliation case not found")
			return
		}
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_argument", err.Error())
			return
		}
		if app.EnterpriseAudit != nil {
			cid := caseID.String()
			_ = app.EnterpriseAudit.Record(r.Context(), compliance.EnterpriseAuditRecord{
				ActorType:    compliance.ActorUser,
				ActorID:      &p.Subject,
				Action:       compliance.ActionCommerceReconciliationCaseResolved,
				ResourceType: "commerce.reconciliation_case",
				ResourceID:   &cid,
				Metadata:     compliance.SanitizeJSONBytes([]byte(`{"source":"admin_commerce_reconciliation_ignore","resolution_kind":"ignored"}`)),
			})
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func requestRefundAdminCommerceReconciliation(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app == nil || app.Commerce == nil || app.Reconciliation == nil {
			writeCapabilityNotConfigured(w, r.Context(), "commerce.refund", "commerce service is required")
			return
		}
		if err := adminCommerceRequirePrincipal(r); err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		caseID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "caseId")))
		if err != nil || caseID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_case_id", "invalid reconciliation case id")
			return
		}
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthorized", "missing principal")
			return
		}
		var body struct {
			AmountMinor *int64 `json:"amount_minor"`
			Reason      string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		actorID, _ := uuid.Parse(strings.TrimSpace(p.Subject))
		out, err := app.Reconciliation.RefundFromReconciliationCase(r.Context(), appcommerceadmin.RefundFromReconciliationCaseInput{
			CaseID:      caseID,
			AmountMinor: body.AmountMinor,
			Reason:      body.Reason,
			RequestedBy: actorID,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "reconciliation case or order not found")
			return
		}
		if err != nil {
			writeCommerceAdminRefundErr(w, r.Context(), err)
			return
		}
		if app.EnterpriseAudit != nil {
			cid := caseID.String()
			mdBytes, _ := json.Marshal(map[string]any{"source": "admin_commerce_request_refund", "refund_request_id": out.RefundRequest.ID})
			md := compliance.SanitizeJSONBytes(mdBytes)
			_ = app.EnterpriseAudit.Record(r.Context(), compliance.EnterpriseAuditRecord{
				ActorType:    compliance.ActorUser,
				ActorID:      &p.Subject,
				Action:       compliance.ActionCommerceReconciliationRefundRequested,
				ResourceType: "commerce.reconciliation_case",
				ResourceID:   &cid,
				Metadata:     md,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"refund_id":          out.LedgerRefundID,
			"refund_request_id":  out.RefundRequest.ID,
			"state":              out.LedgerState,
			"amount_minor":       out.LedgerAmountMinor,
			"currency":           out.LedgerCurrency,
			"refund_request_row": out.RefundRequest,
		})
	}
}

func createAdminCommerceOrderRefund(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app == nil || app.Commerce == nil || app.Reconciliation == nil {
			writeCapabilityNotConfigured(w, r.Context(), "commerce.refund", "commerce service is required")
			return
		}
		idem, err := requireWriteIdempotencyKey(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		if err := adminCommerceRequirePrincipal(r); err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		orderID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "orderId")))
		if err != nil || orderID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_order_id", "invalid order id")
			return
		}
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthorized", "missing principal")
			return
		}
		var body struct {
			AmountMinor *int64 `json:"amount_minor"`
			Currency    string `json:"currency"`
			Reason      string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		actorID, _ := uuid.Parse(strings.TrimSpace(p.Subject))
		out, err := app.Reconciliation.CreateOrderRefund(r.Context(), appcommerceadmin.CreateOrderRefundInput{
			OrderID:        orderID,
			AmountMinor:    body.AmountMinor,
			Currency:       body.Currency,
			Reason:         body.Reason,
			RequestedBy:    actorID,
			IdempotencyKey: idem,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "order not found for company")
			return
		}
		if err != nil {
			writeCommerceAdminRefundErr(w, r.Context(), err)
			return
		}
		if app.EnterpriseAudit != nil {
			rid := out.RefundRequest.ID
			mdBytes, _ := json.Marshal(map[string]any{"source": "admin_order_refund", "ledger_refund_id": out.LedgerRefundID})
			md := compliance.SanitizeJSONBytes(mdBytes)
			_ = app.EnterpriseAudit.Record(r.Context(), compliance.EnterpriseAuditRecord{
				ActorType:    compliance.ActorUser,
				ActorID:      &p.Subject,
				Action:       compliance.ActionRefundRequested,
				ResourceType: "commerce.refund_request",
				ResourceID:   &rid,
				Metadata:     md,
			})
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func writeCommerceAdminRefundErr(w http.ResponseWriter, ctx context.Context, err error) {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "commerceadmin: case_not_actionable"):
		writeAPIError(w, ctx, http.StatusConflict, "case_not_actionable", msg)
	case strings.Contains(msg, "commerceadmin: refund_not_supported"):
		writeAPIError(w, ctx, http.StatusBadRequest, "refund_not_supported_for_case_type", msg)
	case strings.Contains(msg, "commerceadmin: order_required"), strings.Contains(msg, "commerceadmin: invalid_order_id"):
		writeAPIError(w, ctx, http.StatusBadRequest, "invalid_order", msg)
	case strings.Contains(msg, "commerceadmin: nothing_to_refund"):
		writeAPIError(w, ctx, http.StatusConflict, "nothing_to_refund", msg)
	case strings.Contains(msg, "commerceadmin: invalid_amount"):
		writeAPIError(w, ctx, http.StatusBadRequest, "invalid_amount", msg)
	case strings.Contains(msg, "commerceadmin: refund execution not configured"):
		writeCapabilityNotConfigured(w, ctx, "commerce.refund", msg)
	default:
		writeCommerceServiceError(w, ctx, err)
	}
}
