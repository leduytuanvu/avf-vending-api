package httpserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/app/listscope"
	"github.com/avf/avf-vending-api/internal/app/reporting"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// rbac:inherited-mount: `/v1/reports` guarded in server.go; admin org-report routes sit under JWT + tenant-scoped report parsers.

func reportingMaxSpan(app *api.HTTPApplication) time.Duration {
	if app != nil && app.ReportingSyncMaxSpan > 0 {
		return app.ReportingSyncMaxSpan
	}
	return reporting.DefaultReportingWindow
}

func isReportingExportRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	if wantsCSV(r) {
		return true
	}
	p := r.URL.Path
	return strings.HasSuffix(p, "/export.csv") || strings.HasSuffix(p, "/export")
}

func reportingMaxSpanForRequest(r *http.Request, app *api.HTTPApplication) time.Duration {
	syncSpan := reportingMaxSpan(app)
	if !isReportingExportRequest(r) {
		return syncSpan
	}
	exportSpan := syncSpan
	if app != nil && app.ReportingExportMaxSpan > 0 {
		exportSpan = app.ReportingExportMaxSpan
	}
	if exportSpan > syncSpan {
		return exportSpan
	}
	return syncSpan
}

func mountReportingRoutes(r chi.Router, app *api.HTTPApplication) {
	if app == nil || app.Reporting == nil {
		return
	}
	svc := app.Reporting
	r.Get("/sales-summary", getReportSalesSummary(app, svc))
	r.Get("/payments-summary", getReportPaymentsSummary(app, svc))
	r.Get("/fleet-health", getReportFleetHealth(app, svc))
	r.Get("/inventory-exceptions", getReportInventoryExceptions(app, svc))
}

func mountAdminOrganizationReportingRoutes(r chi.Router, app *api.HTTPApplication) {
	if app == nil || app.Reporting == nil {
		return
	}
	svc := app.Reporting
	r.Route("/organizations/{organizationId}/reports", func(r chi.Router) {
		r.Get("/sales", getAdminOrgReportSales(app, svc))
		r.Get("/payments", getAdminOrgReportPayments(app, svc))
		r.Get("/refunds", getAdminOrgReportRefunds(app, svc))
		r.Get("/cash", getAdminOrgReportCash(app, svc))
		r.Get("/inventory-low-stock", getAdminOrgReportInventoryLowStock(app, svc))
		r.Get("/machine-health", getAdminOrgReportMachineHealth(app, svc))
		r.Get("/failed-vends", getAdminOrgReportFailedVends(app, svc))
		r.Get("/reconciliation-queue", getAdminOrgReportReconciliationQueue(app, svc))
		r.Get("/vends", getAdminOrgReportVends(app, svc))
		r.Get("/inventory", getAdminOrgReportInventoryUnified(app, svc))
		r.Get("/machines", getAdminOrgReportMachines(app, svc))
		r.Get("/products", getAdminOrgReportProducts(app, svc))
		r.Get("/reconciliation", getAdminOrgReportReconciliationBI(app, svc))
		r.Get("/commands", getAdminOrgReportCommandFailures(app, svc))
		r.Get("/fills", getAdminOrgReportTechnicianFills(app, svc))
		r.Get("/export", getAdminOrgReportExportCSV(app, svc))
	})
}

func parseRequiredRFC3339Range(q url.Values) (time.Time, time.Time, error) {
	rawFrom := strings.TrimSpace(q.Get("from"))
	rawTo := strings.TrimSpace(q.Get("to"))
	if rawFrom == "" || rawTo == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("from and to query parameters are required (RFC3339)")
	}
	from, err := time.Parse(time.RFC3339Nano, rawFrom)
	if err != nil {
		from, err = time.Parse(time.RFC3339, rawFrom)
	}
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid from timestamp")
	}
	to, err := time.Parse(time.RFC3339Nano, rawTo)
	if err != nil {
		to, err = time.Parse(time.RFC3339, rawTo)
	}
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid to timestamp")
	}
	return from.UTC(), to.UTC(), nil
}

func parseReportingOrganization(r *http.Request, app *api.HTTPApplication) (listscope.ReportingQuery, error) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return listscope.ReportingQuery{}, listscope.ErrInvalidListQuery
	}
	qv := r.URL.Query()
	from, to, err := parseRequiredRFC3339Range(qv)
	if err != nil {
		return listscope.ReportingQuery{}, listscope.ErrInvalidListQuery
	}
	maxSpan := reportingMaxSpanForRequest(r, app)
	if err := reporting.ValidateReportingWindow(from, to, maxSpan); err != nil {
		return listscope.ReportingQuery{}, listscope.ErrInvalidListQuery
	}
	tz := parseReportingTimezone(qv)
	if !validateReportingTimezone(tz) {
		return listscope.ReportingQuery{}, listscope.ErrInvalidListQuery
	}
	var orgID uuid.UUID
	if p.HasRole(auth.RolePlatformAdmin) {
		raw := strings.TrimSpace(qv.Get("organization_id"))
		id, perr := uuid.Parse(raw)
		if perr != nil || id == uuid.Nil {
			return listscope.ReportingQuery{}, api.ErrCommerceOrganizationQueryRequired
		}
		orgID = id
	} else {
		if !p.HasOrganization() {
			return listscope.ReportingQuery{}, api.ErrCommerceOrganizationQueryRequired
		}
		orgID = p.OrganizationID
		if raw := strings.TrimSpace(qv.Get("organization_id")); raw != "" {
			qid, perr := uuid.Parse(raw)
			if perr != nil || qid != orgID {
				return listscope.ReportingQuery{}, listscope.ErrInvalidListQuery
			}
		}
	}
	return listscope.ReportingQuery{
		IsPlatformAdmin: p.HasRole(auth.RolePlatformAdmin),
		OrganizationID:  orgID,
		From:            from,
		To:              to,
		Timezone:        tz,
	}, nil
}

func parseReportingTimezone(q url.Values) string {
	tz := strings.TrimSpace(q.Get("timezone"))
	if tz == "" {
		return "UTC"
	}
	return tz
}

func validateReportingTimezone(tz string) bool {
	_, err := time.LoadLocation(tz)
	return err == nil
}

func parseAdminOrganizationReportingQuery(r *http.Request, paged bool, app *api.HTTPApplication) (listscope.ReportingQuery, int, error) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return listscope.ReportingQuery{}, http.StatusUnauthorized, fmt.Errorf("missing principal")
	}
	orgID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "organizationId")))
	if err != nil || orgID == uuid.Nil {
		return listscope.ReportingQuery{}, http.StatusBadRequest, fmt.Errorf("invalid organizationId")
	}
	if !p.HasRole(auth.RolePlatformAdmin) {
		if !p.HasOrganization() || p.OrganizationID != orgID {
			return listscope.ReportingQuery{}, http.StatusForbidden, fmt.Errorf("organization access denied")
		}
	}
	qv := r.URL.Query()
	from, to, err := parseRequiredRFC3339Range(qv)
	maxSpan := reportingMaxSpanForRequest(r, app)
	if err != nil || reporting.ValidateReportingWindow(from, to, maxSpan) != nil {
		return listscope.ReportingQuery{}, http.StatusBadRequest, fmt.Errorf("invalid reporting date range")
	}
	tz := parseReportingTimezone(qv)
	if !validateReportingTimezone(tz) {
		return listscope.ReportingQuery{}, http.StatusBadRequest, fmt.Errorf("invalid timezone")
	}
	out := listscope.ReportingQuery{
		IsPlatformAdmin: p.HasRole(auth.RolePlatformAdmin),
		OrganizationID:  orgID,
		From:            from,
		To:              to,
		Timezone:        tz,
	}
	if paged {
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			return listscope.ReportingQuery{}, http.StatusBadRequest, err
		}
		out.Limit = limit
		out.Offset = offset
	}
	return out, http.StatusOK, nil
}

func applyOptionalReportFilters(qv url.Values, dst *listscope.ReportingQuery) error {
	if raw := strings.TrimSpace(qv.Get("site_id")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil {
			return fmt.Errorf("invalid site_id")
		}
		dst.SiteIDFilter = id
	}
	if raw := strings.TrimSpace(qv.Get("machine_id")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil {
			return fmt.Errorf("invalid machine_id")
		}
		dst.MachineIDFilter = id
	}
	if raw := strings.TrimSpace(qv.Get("product_id")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil {
			return fmt.Errorf("invalid product_id")
		}
		dst.ProductIDFilter = id
	}
	return nil
}

func normalizeReconciliationScope(raw string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return "all", nil
	}
	switch s {
	case "open", "closed", "all":
		return s, nil
	default:
		return "", fmt.Errorf("invalid reconciliation_scope (open|closed|all)")
	}
}

func normalizeInventoryReportKind(raw string) (string, error) {
	k := strings.ToLower(strings.TrimSpace(raw))
	if k == "" {
		return "low_stock", nil
	}
	switch k {
	case "low_stock", "movement":
		return k, nil
	default:
		return "", fmt.Errorf("invalid kind (low_stock|movement)")
	}
}

func normalizeSalesGroupBy(raw string) (string, error) {
	g := strings.ToLower(strings.TrimSpace(raw))
	if g == "" {
		return "day", nil
	}
	switch g {
	case "day", "site", "machine", "payment_method", "none", "product":
		return g, nil
	default:
		return "", fmt.Errorf("invalid group_by")
	}
}

func normalizePaymentsGroupBy(raw string) (string, error) {
	g := strings.ToLower(strings.TrimSpace(raw))
	if g == "" {
		return "day", nil
	}
	switch g {
	case "day", "payment_method", "status", "none":
		return g, nil
	default:
		return "", fmt.Errorf("invalid group_by")
	}
}

func normalizeExceptionKind(raw string) (string, error) {
	k := strings.ToLower(strings.TrimSpace(raw))
	if k == "" {
		return "all", nil
	}
	switch k {
	case "all", "low_stock", "out_of_stock":
		return k, nil
	default:
		return "", fmt.Errorf("invalid exception_kind")
	}
}

func parseSalesReportingQuery(r *http.Request, app *api.HTTPApplication) (listscope.ReportingQuery, error) {
	base, err := parseReportingOrganization(r, app)
	if err != nil {
		return listscope.ReportingQuery{}, err
	}
	gb, err := normalizeSalesGroupBy(r.URL.Query().Get("group_by"))
	if err != nil {
		return listscope.ReportingQuery{}, listscope.ErrInvalidListQuery
	}
	base.GroupBy = gb
	return base, nil
}

func parsePaymentsReportingQuery(r *http.Request, app *api.HTTPApplication) (listscope.ReportingQuery, error) {
	base, err := parseReportingOrganization(r, app)
	if err != nil {
		return listscope.ReportingQuery{}, err
	}
	gb, err := normalizePaymentsGroupBy(r.URL.Query().Get("group_by"))
	if err != nil {
		return listscope.ReportingQuery{}, listscope.ErrInvalidListQuery
	}
	base.GroupBy = gb
	return base, nil
}

func parseFleetReportingQuery(r *http.Request, app *api.HTTPApplication) (listscope.ReportingQuery, error) {
	base, err := parseReportingOrganization(r, app)
	if err != nil {
		return listscope.ReportingQuery{}, err
	}
	if strings.TrimSpace(r.URL.Query().Get("group_by")) != "" {
		return listscope.ReportingQuery{}, listscope.ErrInvalidListQuery
	}
	return base, nil
}

func parseInventoryReportingQuery(r *http.Request, app *api.HTTPApplication) (listscope.ReportingQuery, error) {
	base, err := parseReportingOrganization(r, app)
	if err != nil {
		return listscope.ReportingQuery{}, err
	}
	if strings.TrimSpace(r.URL.Query().Get("group_by")) != "" {
		return listscope.ReportingQuery{}, listscope.ErrInvalidListQuery
	}
	kind, err := normalizeExceptionKind(r.URL.Query().Get("exception_kind"))
	if err != nil {
		return listscope.ReportingQuery{}, listscope.ErrInvalidListQuery
	}
	base.ExceptionKind = kind
	limit, offset, err := parseAdminLimitOffset(r)
	if err != nil {
		return listscope.ReportingQuery{}, listscope.ErrInvalidListQuery
	}
	base.Limit = limit
	base.Offset = offset
	return base, nil
}

func parseCashReportingQuery(r *http.Request, app *api.HTTPApplication) (listscope.ReportingQuery, error) {
	base, err := parseReportingOrganization(r, app)
	if err != nil {
		return listscope.ReportingQuery{}, err
	}
	if strings.TrimSpace(r.URL.Query().Get("group_by")) != "" ||
		strings.TrimSpace(r.URL.Query().Get("exception_kind")) != "" {
		return listscope.ReportingQuery{}, listscope.ErrInvalidListQuery
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("site_id")); raw != "" {
		sid, perr := uuid.Parse(raw)
		if perr != nil || sid == uuid.Nil {
			return listscope.ReportingQuery{}, listscope.ErrInvalidListQuery
		}
		base.SiteIDFilter = sid
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("machine_id")); raw != "" {
		mid, perr := uuid.Parse(raw)
		if perr != nil || mid == uuid.Nil {
			return listscope.ReportingQuery{}, listscope.ErrInvalidListQuery
		}
		base.MachineIDFilter = mid
	}
	return base, nil
}

func getReportSalesSummary(app *api.HTTPApplication, svc api.ReportingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, err := parseSalesReportingQuery(r, app)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		out, err := svc.SalesSummary(r.Context(), q)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "reporting_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func getReportPaymentsSummary(app *api.HTTPApplication, svc api.ReportingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, err := parsePaymentsReportingQuery(r, app)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		out, err := svc.PaymentsSummary(r.Context(), q)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "reporting_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func getReportFleetHealth(app *api.HTTPApplication, svc api.ReportingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, err := parseFleetReportingQuery(r, app)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		out, err := svc.FleetHealth(r.Context(), q)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func getReportInventoryExceptions(app *api.HTTPApplication, svc api.ReportingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, err := parseInventoryReportingQuery(r, app)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		out, err := svc.InventoryExceptions(r.Context(), q)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "reporting_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func writeAdminReportQueryError(w http.ResponseWriter, r *http.Request, status int, err error) {
	code := "invalid_report_query"
	if status == http.StatusForbidden {
		code = "forbidden"
	}
	if status == http.StatusUnauthorized {
		code = "unauthorized"
	}
	writeAPIError(w, r.Context(), status, code, err.Error())
}

func wantsCSV(r *http.Request) bool {
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	return format == "csv" || format == "text/csv"
}

func recordReportExportAudit(r *http.Request, app *api.HTTPApplication, orgID uuid.UUID, reportName string) {
	if app == nil || app.EnterpriseAudit == nil {
		return
	}
	at, aid := compliance.ActorUser, ""
	if p, ok := auth.PrincipalFromContext(r.Context()); ok {
		at, aid = p.Actor()
	}
	md, _ := json.Marshal(map[string]any{
		"report":        reportName,
		"query":         r.URL.RawQuery,
		"export_format": "csv",
	})
	rid := reportName
	_ = app.EnterpriseAudit.Record(r.Context(), compliance.EnterpriseAuditRecord{
		OrganizationID: orgID,
		ActorType:      at,
		ActorID:        stringPtrOrNil(aid),
		Action:         "reports.exported",
		ResourceType:   "report",
		ResourceID:     &rid,
		Metadata:       md,
	})
}

func getAdminOrgReportSales(app *api.HTTPApplication, svc api.ReportingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, status, err := parseAdminOrganizationReportingQuery(r, false, app)
		if err != nil {
			writeAdminReportQueryError(w, r, status, err)
			return
		}
		if err := applyOptionalReportFilters(r.URL.Query(), &q); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_filters", err.Error())
			return
		}
		gb, err := normalizeSalesGroupBy(r.URL.Query().Get("group_by"))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_group_by", err.Error())
			return
		}
		q.GroupBy = gb
		out, err := svc.SalesSummary(r.Context(), q)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "reporting_error", err.Error())
			return
		}
		if wantsCSV(r) {
			recordReportExportAudit(r, app, q.OrganizationID, "sales")
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			w.Header().Set("Content-Disposition", `attachment; filename="sales.csv"`)
			if err := reporting.WriteSalesSummaryCSV(w, out); err != nil {
				writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			}
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func getAdminOrgReportPayments(app *api.HTTPApplication, svc api.ReportingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, status, err := parseAdminOrganizationReportingQuery(r, false, app)
		if err != nil {
			writeAdminReportQueryError(w, r, status, err)
			return
		}
		if err := applyOptionalReportFilters(r.URL.Query(), &q); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_filters", err.Error())
			return
		}
		out, err := svc.PaymentSettlement(r.Context(), q)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "reporting_error", err.Error())
			return
		}
		if wantsCSV(r) {
			recordReportExportAudit(r, app, q.OrganizationID, "payments")
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			w.Header().Set("Content-Disposition", `attachment; filename="payments.csv"`)
			if err := reporting.WritePaymentSettlementCSV(w, out); err != nil {
				writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			}
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func getAdminOrgReportRefunds(app *api.HTTPApplication, svc api.ReportingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, status, err := parseAdminOrganizationReportingQuery(r, true, app)
		if err != nil {
			writeAdminReportQueryError(w, r, status, err)
			return
		}
		if err := applyOptionalReportFilters(r.URL.Query(), &q); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_filters", err.Error())
			return
		}
		out, err := svc.Refunds(r.Context(), q)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "reporting_error", err.Error())
			return
		}
		if wantsCSV(r) {
			recordReportExportAudit(r, app, q.OrganizationID, "refunds")
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			w.Header().Set("Content-Disposition", `attachment; filename="refunds.csv"`)
			if err := reporting.WriteRefundsCSV(w, out); err != nil {
				writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			}
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func getAdminOrgReportCash(app *api.HTTPApplication, svc api.ReportingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, status, err := parseAdminOrganizationReportingQuery(r, true, app)
		if err != nil {
			writeAdminReportQueryError(w, r, status, err)
			return
		}
		if err := applyOptionalReportFilters(r.URL.Query(), &q); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_filters", err.Error())
			return
		}
		if wantsCSV(r) {
			rows, err := svc.CashCollectionsExport(r.Context(), q)
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "reporting_error", err.Error())
				return
			}
			recordReportExportAudit(r, app, q.OrganizationID, "cash")
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			w.Header().Set("Content-Disposition", `attachment; filename="cash.csv"`)
			if err := reporting.WriteCashCollectionsCSV(w, q.OrganizationID.String(), q.From.UTC().Format(time.RFC3339Nano), q.To.UTC().Format(time.RFC3339Nano), rows); err != nil {
				writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			}
			return
		}
		out, err := svc.CashCollectionsReport(r.Context(), q)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "reporting_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func getAdminOrgReportInventoryLowStock(app *api.HTTPApplication, svc api.ReportingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = app
		q, status, err := parseAdminOrganizationReportingQuery(r, true, app)
		if err != nil {
			writeAdminReportQueryError(w, r, status, err)
			return
		}
		if err := applyOptionalReportFilters(r.URL.Query(), &q); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_filters", err.Error())
			return
		}
		q.ExceptionKind = "low_stock"
		out, err := svc.InventoryExceptions(r.Context(), q)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "reporting_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func getAdminOrgReportMachineHealth(app *api.HTTPApplication, svc api.ReportingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = app
		q, status, err := parseAdminOrganizationReportingQuery(r, true, app)
		if err != nil {
			writeAdminReportQueryError(w, r, status, err)
			return
		}
		if err := applyOptionalReportFilters(r.URL.Query(), &q); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_filters", err.Error())
			return
		}
		out, err := svc.MachineHealth(r.Context(), q)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "reporting_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func getAdminOrgReportFailedVends(app *api.HTTPApplication, svc api.ReportingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = app
		q, status, err := parseAdminOrganizationReportingQuery(r, true, app)
		if err != nil {
			writeAdminReportQueryError(w, r, status, err)
			return
		}
		if err := applyOptionalReportFilters(r.URL.Query(), &q); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_filters", err.Error())
			return
		}
		out, err := svc.FailedVends(r.Context(), q)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "reporting_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func getAdminOrgReportReconciliationQueue(app *api.HTTPApplication, svc api.ReportingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = app
		q, status, err := parseAdminOrganizationReportingQuery(r, true, app)
		if err != nil {
			writeAdminReportQueryError(w, r, status, err)
			return
		}
		if err := applyOptionalReportFilters(r.URL.Query(), &q); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_filters", err.Error())
			return
		}
		out, err := svc.ReconciliationQueue(r.Context(), q)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "reporting_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}
