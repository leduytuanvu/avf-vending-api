package httpserver

import (
	"net/http"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/app/reporting"
	"github.com/go-chi/chi/v5"
)

// rbac:inherited-mount: mounted under JWT + ReportsRead scopes from server.go (same subtree as CSV rate limits).

func mountAdminReportingCSVExports(r chi.Router, app *api.HTTPApplication) {
	if app == nil || app.Reporting == nil {
		return
	}
	svc := app.Reporting
	r.Get("/reports/sales-summary/export.csv", getAdminSalesSummaryExportCSV(app, svc))
	r.Get("/reports/payments-summary/export.csv", getAdminPaymentsSummaryExportCSV(app, svc))
	r.Get("/reports/cash-collections/export.csv", getAdminCashCollectionsExportCSV(app, svc))
}

func getAdminSalesSummaryExportCSV(app *api.HTTPApplication, svc api.ReportingService) http.HandlerFunc {
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
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="sales-summary.csv"`)
		recordReportExportAudit(r, app, q.OrganizationID, "sales-summary")
		if err := reporting.WriteSalesSummaryCSV(w, out); err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
	}
}

func getAdminPaymentsSummaryExportCSV(app *api.HTTPApplication, svc api.ReportingService) http.HandlerFunc {
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
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="payments-summary.csv"`)
		recordReportExportAudit(r, app, q.OrganizationID, "payments-summary")
		if err := reporting.WritePaymentsSummaryCSV(w, out); err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
	}
}

func getAdminCashCollectionsExportCSV(app *api.HTTPApplication, svc api.ReportingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, err := parseCashReportingQuery(r, app)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		rows, err := svc.CashCollectionsExport(r.Context(), q)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "reporting_error", err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="cash-collections.csv"`)
		recordReportExportAudit(r, app, q.OrganizationID, "cash-collections")
		orgStr := q.OrganizationID.String()
		from := q.From.UTC().Format(time.RFC3339Nano)
		to := q.To.UTC().Format(time.RFC3339Nano)
		if err := reporting.WriteCashCollectionsCSV(w, orgStr, from, to, rows); err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
	}
}
