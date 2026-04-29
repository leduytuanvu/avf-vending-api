package httpserver

import (
	"net/http"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/app/reporting"
)

func getAdminOrgReportVends(app *api.HTTPApplication, svc api.ReportingService) http.HandlerFunc {
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
		out, err := svc.VendSummary(r.Context(), q)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "reporting_error", err.Error())
			return
		}
		if wantsCSV(r) {
			recordReportExportAudit(r, app, q.OrganizationID, "vends")
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			w.Header().Set("Content-Disposition", `attachment; filename="vends.csv"`)
			if err := reporting.WriteVendSummaryCSV(w, out); err != nil {
				writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			}
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func getAdminOrgReportInventoryUnified(app *api.HTTPApplication, svc api.ReportingService) http.HandlerFunc {
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
		kind, err := normalizeInventoryReportKind(r.URL.Query().Get("kind"))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_kind", err.Error())
			return
		}
		q.InventoryReportKind = kind
		switch kind {
		case "movement":
			out, err := svc.StockMovement(r.Context(), q)
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "reporting_error", err.Error())
				return
			}
			if wantsCSV(r) {
				recordReportExportAudit(r, app, q.OrganizationID, "inventory_movement")
				w.Header().Set("Content-Type", "text/csv; charset=utf-8")
				w.Header().Set("Content-Disposition", `attachment; filename="inventory-movement.csv"`)
				if err := reporting.WriteStockMovementCSV(w, out); err != nil {
					writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
				}
				return
			}
			writeJSON(w, http.StatusOK, out)
			return
		default:
			ek := strings.TrimSpace(r.URL.Query().Get("exception_kind"))
			if ek == "" {
				ek = "low_stock"
			}
			excKind, err := normalizeExceptionKind(ek)
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_exception_kind", err.Error())
				return
			}
			q.ExceptionKind = excKind
			out, err := svc.InventoryExceptions(r.Context(), q)
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "reporting_error", err.Error())
				return
			}
			if wantsCSV(r) {
				recordReportExportAudit(r, app, q.OrganizationID, "inventory_low_stock")
				w.Header().Set("Content-Type", "text/csv; charset=utf-8")
				w.Header().Set("Content-Disposition", `attachment; filename="inventory-low-stock.csv"`)
				if err := reporting.WriteInventoryExceptionsCSV(w, out); err != nil {
					writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
				}
				return
			}
			writeJSON(w, http.StatusOK, out)
		}
	}
}

func getAdminOrgReportMachines(app *api.HTTPApplication, svc api.ReportingService) http.HandlerFunc {
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
		out, err := svc.MachineHealth(r.Context(), q)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "reporting_error", err.Error())
			return
		}
		if wantsCSV(r) {
			recordReportExportAudit(r, app, q.OrganizationID, "machines")
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			w.Header().Set("Content-Disposition", `attachment; filename="machines.csv"`)
			if err := reporting.WriteMachineHealthCSV(w, out); err != nil {
				writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			}
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func getAdminOrgReportProducts(app *api.HTTPApplication, svc api.ReportingService) http.HandlerFunc {
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
		out, err := svc.ProductPerformance(r.Context(), q)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "reporting_error", err.Error())
			return
		}
		if wantsCSV(r) {
			recordReportExportAudit(r, app, q.OrganizationID, "products")
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			w.Header().Set("Content-Disposition", `attachment; filename="products.csv"`)
			if err := reporting.WriteProductPerformanceCSV(w, out); err != nil {
				writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			}
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func getAdminOrgReportReconciliationBI(app *api.HTTPApplication, svc api.ReportingService) http.HandlerFunc {
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
		scope, err := normalizeReconciliationScope(r.URL.Query().Get("reconciliation_scope"))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_reconciliation_scope", err.Error())
			return
		}
		q.ReconciliationScope = scope
		out, err := svc.ReconciliationBI(r.Context(), q)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "reporting_error", err.Error())
			return
		}
		if wantsCSV(r) {
			recordReportExportAudit(r, app, q.OrganizationID, "reconciliation_bi")
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			w.Header().Set("Content-Disposition", `attachment; filename="reconciliation.csv"`)
			if err := reporting.WriteReconciliationBICSV(w, out); err != nil {
				writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			}
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func getAdminOrgReportCommandFailures(app *api.HTTPApplication, svc api.ReportingService) http.HandlerFunc {
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
		out, err := svc.CommandFailures(r.Context(), q)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "reporting_error", err.Error())
			return
		}
		if wantsCSV(r) {
			recordReportExportAudit(r, app, q.OrganizationID, "command_failures")
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			w.Header().Set("Content-Disposition", `attachment; filename="command-failures.csv"`)
			if err := reporting.WriteCommandFailuresCSV(w, out); err != nil {
				writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			}
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func getAdminOrgReportTechnicianFills(app *api.HTTPApplication, svc api.ReportingService) http.HandlerFunc {
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
		out, err := svc.TechnicianFillOperations(r.Context(), q)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "reporting_error", err.Error())
			return
		}
		if wantsCSV(r) {
			recordReportExportAudit(r, app, q.OrganizationID, "technician_fills")
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			w.Header().Set("Content-Disposition", `attachment; filename="technician-fills.csv"`)
			if err := reporting.WriteTechnicianFillOpsCSV(w, out); err != nil {
				writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			}
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func getAdminOrgReportExportCSV(app *api.HTTPApplication, svc api.ReportingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("report")))
		if name == "" {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_report", "report query parameter is required")
			return
		}
		switch name {
		case "sales":
			q := r.URL.Query()
			q.Set("format", "csv")
			r.URL.RawQuery = q.Encode()
			getAdminOrgReportSales(app, svc)(w, r)
		case "payments":
			q := r.URL.Query()
			q.Set("format", "csv")
			r.URL.RawQuery = q.Encode()
			getAdminOrgReportPayments(app, svc)(w, r)
		case "products":
			q := r.URL.Query()
			q.Set("format", "csv")
			r.URL.RawQuery = q.Encode()
			getAdminOrgReportProducts(app, svc)(w, r)
		case "reconciliation":
			q := r.URL.Query()
			q.Set("format", "csv")
			r.URL.RawQuery = q.Encode()
			getAdminOrgReportReconciliationBI(app, svc)(w, r)
		case "machines":
			q := r.URL.Query()
			q.Set("format", "csv")
			r.URL.RawQuery = q.Encode()
			getAdminOrgReportMachines(app, svc)(w, r)
		case "vends":
			q := r.URL.Query()
			q.Set("format", "csv")
			r.URL.RawQuery = q.Encode()
			getAdminOrgReportVends(app, svc)(w, r)
		case "inventory":
			q := r.URL.Query()
			q.Set("format", "csv")
			r.URL.RawQuery = q.Encode()
			getAdminOrgReportInventoryUnified(app, svc)(w, r)
		case "commands", "command_failures":
			q := r.URL.Query()
			q.Set("format", "csv")
			r.URL.RawQuery = q.Encode()
			getAdminOrgReportCommandFailures(app, svc)(w, r)
		case "fills", "technician_fills":
			q := r.URL.Query()
			q.Set("format", "csv")
			r.URL.RawQuery = q.Encode()
			getAdminOrgReportTechnicianFills(app, svc)(w, r)
		default:
			writeAPIError(w, r.Context(), http.StatusBadRequest, "unknown_report", "unsupported report export name")
		}
	}
}
