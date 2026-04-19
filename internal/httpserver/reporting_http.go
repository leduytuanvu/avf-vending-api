package httpserver

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/app/listscope"
	"github.com/avf/avf-vending-api/internal/app/reporting"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func mountReportingRoutes(r chi.Router, app *api.HTTPApplication) {
	if app == nil || app.Reporting == nil {
		return
	}
	svc := app.Reporting
	r.Get("/sales-summary", getReportSalesSummary(svc))
	r.Get("/payments-summary", getReportPaymentsSummary(svc))
	r.Get("/fleet-health", getReportFleetHealth(svc))
	r.Get("/inventory-exceptions", getReportInventoryExceptions(svc))
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

func parseReportingOrganization(r *http.Request) (listscope.ReportingQuery, error) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return listscope.ReportingQuery{}, listscope.ErrInvalidListQuery
	}
	qv := r.URL.Query()
	from, to, err := parseRequiredRFC3339Range(qv)
	if err != nil {
		return listscope.ReportingQuery{}, listscope.ErrInvalidListQuery
	}
	if err := reporting.ValidateReportingWindow(from, to); err != nil {
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
	}, nil
}

func normalizeSalesGroupBy(raw string) (string, error) {
	g := strings.ToLower(strings.TrimSpace(raw))
	if g == "" {
		return "day", nil
	}
	switch g {
	case "day", "site", "machine", "payment_method", "none":
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

func parseSalesReportingQuery(r *http.Request) (listscope.ReportingQuery, error) {
	base, err := parseReportingOrganization(r)
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

func parsePaymentsReportingQuery(r *http.Request) (listscope.ReportingQuery, error) {
	base, err := parseReportingOrganization(r)
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

func parseFleetReportingQuery(r *http.Request) (listscope.ReportingQuery, error) {
	base, err := parseReportingOrganization(r)
	if err != nil {
		return listscope.ReportingQuery{}, err
	}
	if strings.TrimSpace(r.URL.Query().Get("group_by")) != "" {
		return listscope.ReportingQuery{}, listscope.ErrInvalidListQuery
	}
	return base, nil
}

func parseInventoryReportingQuery(r *http.Request) (listscope.ReportingQuery, error) {
	base, err := parseReportingOrganization(r)
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

func getReportSalesSummary(svc api.ReportingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, err := parseSalesReportingQuery(r)
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

func getReportPaymentsSummary(svc api.ReportingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, err := parsePaymentsReportingQuery(r)
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

func getReportFleetHealth(svc api.ReportingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, err := parseFleetReportingQuery(r)
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

func getReportInventoryExceptions(svc api.ReportingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, err := parseInventoryReportingQuery(r)
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
