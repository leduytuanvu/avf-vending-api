package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/app/listscope"
	apppayments "github.com/avf/avf-vending-api/internal/app/payments"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// mountAdminPaymentOpsRoutes registers P1.2 finance/payment operations under /v1/admin/organizations/{organizationId}/payments/...
func mountAdminPaymentOpsRoutes(r chi.Router, app *api.HTTPApplication, writeRL func(http.Handler) http.Handler) {
	if app == nil || app.PaymentOps == nil {
		return
	}
	if writeRL == nil {
		writeRL = func(next http.Handler) http.Handler { return next }
	}
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermCommerceRead, auth.PermPaymentRead))
		r.Get("/organizations/{organizationId}/payments/reconciliation", getAdminPaymentReconciliationDrift(app))
		r.Get("/organizations/{organizationId}/payments/webhook-events", listAdminPaymentWebhookEvents(app))
		r.Get("/organizations/{organizationId}/payments/settlements", listAdminPaymentSettlements(app))
		r.Get("/organizations/{organizationId}/payments/disputes", listAdminPaymentDisputes(app))
		r.Get("/organizations/{organizationId}/payments/export", exportAdminPaymentsFinanceCSV(app))
	})
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermRefundsWrite))
		r.With(writeRL).Post("/organizations/{organizationId}/payments/settlements/import", importAdminPaymentSettlements(app))
		r.With(writeRL).Post("/organizations/{organizationId}/payments/disputes/{disputeId}/resolve", resolveAdminPaymentDispute(app))
	})
}

func parseAdminFinanceExportWindow(r *http.Request) (from time.Time, to time.Time, err error) {
	q := r.URL.Query()
	fromRaw := strings.TrimSpace(q.Get("from"))
	toRaw := strings.TrimSpace(q.Get("to"))
	if fromRaw == "" || toRaw == "" {
		return time.Time{}, time.Time{}, errors.New("from and to query parameters are required (RFC3339 or RFC3339Nano)")
	}
	from, err = time.Parse(time.RFC3339Nano, fromRaw)
	if err != nil {
		from, err = time.Parse(time.RFC3339, fromRaw)
	}
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	to, err = time.Parse(time.RFC3339Nano, toRaw)
	if err != nil {
		to, err = time.Parse(time.RFC3339, toRaw)
	}
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	from = from.UTC()
	to = to.UTC()
	if to.Before(from) {
		return time.Time{}, time.Time{}, errors.New("to must be >= from")
	}
	return from, to, nil
}

func getAdminPaymentReconciliationDrift(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminCommerceOrganizationID(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		q := r.URL.Query()
		var staleSecs int64
		if v := strings.TrimSpace(q.Get("stale_after_seconds")); v != "" {
			sec, perr := strconv.ParseInt(v, 10, 64)
			if perr != nil || sec <= 0 {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_query", "stale_after_seconds must be a positive integer")
				return
			}
			staleSecs = sec
		} else if v := strings.TrimSpace(q.Get("stale_hours")); v != "" {
			hours, herr := strconv.ParseFloat(v, 64)
			if herr != nil || hours <= 0 {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_query", "stale_hours must be a positive number")
				return
			}
			staleSecs = int64(hours * float64(time.Hour/time.Second))
		}
		limit := int32(100)
		if v := strings.TrimSpace(q.Get("limit")); v != "" {
			n, lerr := strconv.Atoi(v)
			if lerr != nil || n <= 0 {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_query", "limit must be a positive integer")
				return
			}
			limit = int32(n)
		}
		out, err := app.PaymentOps.ListPaymentReconciliationDrift(r.Context(), orgID, staleSecs, limit)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func listAdminPaymentWebhookEvents(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminCommerceOrganizationID(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			writeV1ListError(w, r.Context(), listscope.ErrInvalidListQuery)
			return
		}
		out, err := app.PaymentOps.ListWebhookEvents(r.Context(), orgID, limit, offset)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func listAdminPaymentSettlements(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminCommerceOrganizationID(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			writeV1ListError(w, r.Context(), listscope.ErrInvalidListQuery)
			return
		}
		out, err := app.PaymentOps.ListSettlements(r.Context(), orgID, limit, offset)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func importAdminPaymentSettlements(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminCommerceOrganizationID(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		var body struct {
			Provider    string                             `json:"provider"`
			Settlements []apppayments.SettlementImportItem `json:"settlements"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		out, err := app.PaymentOps.ImportSettlements(r.Context(), orgID, body.Provider, body.Settlements)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_argument", err.Error())
			return
		}
		p, ok := auth.PrincipalFromContext(r.Context())
		if ok && app.EnterpriseAudit != nil {
			meta, _ := json.Marshal(map[string]any{
				"provider":             strings.TrimSpace(body.Provider),
				"settlement_row_count": len(body.Settlements),
			})
			if aerr := app.EnterpriseAudit.RecordCritical(r.Context(), compliance.EnterpriseAuditRecord{
				OrganizationID: orgID,
				ActorType:      compliance.ActorUser,
				ActorID:        &p.Subject,
				Action:         compliance.ActionPaymentSettlementImported,
				ResourceType:   "payments.settlement_import",
				Metadata:       compliance.SanitizeJSONBytes(meta),
			}); aerr != nil {
				writeAPIError(w, r.Context(), http.StatusInternalServerError, "audit_failed", "could not record audit event")
				return
			}
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func listAdminPaymentDisputes(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminCommerceOrganizationID(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			writeV1ListError(w, r.Context(), listscope.ErrInvalidListQuery)
			return
		}
		out, err := app.PaymentOps.ListDisputes(r.Context(), orgID, limit, offset)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func resolveAdminPaymentDispute(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminCommerceOrganizationID(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		did, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "disputeId")))
		if err != nil || did == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_dispute_id", "invalid dispute id")
			return
		}
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthorized", "missing principal")
			return
		}
		actorID, _ := uuid.Parse(strings.TrimSpace(p.Subject))
		var body struct {
			Status string `json:"status"`
			Note   string `json:"note"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		out, err := app.PaymentOps.ResolveDispute(r.Context(), apppayments.ResolveDisputeInput{
			OrganizationID: orgID,
			DisputeID:      did,
			Status:         body.Status,
			Note:           body.Note,
			ResolvedBy:     actorID,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "dispute not found or already terminal")
			return
		}
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_argument", err.Error())
			return
		}
		if app.EnterpriseAudit != nil {
			dids := did.String()
			meta, _ := json.Marshal(map[string]any{"status": strings.TrimSpace(body.Status)})
			if aerr := app.EnterpriseAudit.RecordCritical(r.Context(), compliance.EnterpriseAuditRecord{
				OrganizationID: orgID,
				ActorType:      compliance.ActorUser,
				ActorID:        &p.Subject,
				Action:         compliance.ActionPaymentDisputeResolved,
				ResourceType:   "payments.dispute",
				ResourceID:     &dids,
				Metadata:       compliance.SanitizeJSONBytes(meta),
			}); aerr != nil {
				writeAPIError(w, r.Context(), http.StatusInternalServerError, "audit_failed", "could not record audit event")
				return
			}
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func exportAdminPaymentsFinanceCSV(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminCommerceOrganizationID(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		from, to, err := parseAdminFinanceExportWindow(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_range", err.Error())
			return
		}
		var buf bytes.Buffer
		if err := app.PaymentOps.WriteFinanceExportCSV(r.Context(), &buf, orgID, from, to); err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="payments-`+orgID.String()+`.csv"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(buf.Bytes())
	}
}
