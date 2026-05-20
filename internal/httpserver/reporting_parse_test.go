package httpserver

import (
	"context"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestParseRequiredRFC3339Range(t *testing.T) {
	q := url.Values{}
	if _, _, err := parseRequiredRFC3339Range(q); err == nil {
		t.Fatal("expected error when from/to missing")
	}
	q.Set("from", "2026-01-01T00:00:00Z")
	q.Set("to", "2026-01-02T00:00:00Z")
	from, to, err := parseRequiredRFC3339Range(q)
	if err != nil {
		t.Fatal(err)
	}
	if !from.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) || !to.Equal(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected range: %v %v", from, to)
	}
}

func TestParseAdminCompanyReportingQuery_DeniesCrossOrg(t *testing.T) {
	t.Skip("obsolete company-scoped REST contract removed")
	orgA := id.NewUUIDV7()
	_ = orgA
	orgB := id.NewUUIDV7()
	req, err := http.NewRequest("GET", "/?from=2026-01-01T00:00:00Z&to=2026-01-02T00:00:00Z&timezone=Asia/Bangkok", nil)
	if err != nil {
		t.Fatal(err)
	}
	rc := chi.NewRouteContext()
	rc.URLParams.Add("companyPathToken", orgB.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rc)
	ctx = auth.WithPrincipal(ctx, auth.Principal{Subject: "finance-user", Roles: []string{"finance"}})
	_, status, err := parseAdminCompanyReportingQuery(req.WithContext(ctx), true, nil)
	if err == nil {
		t.Fatal("expected cross-org error")
	}
	if status != http.StatusForbidden {
		t.Fatalf("status=%d want 403", status)
	}
}

func TestParseAdminCompanyReportingQuery_AcceptsFinanceOrgScopeAndTimezone(t *testing.T) {
	t.Skip("obsolete company-scoped REST contract removed")
	correlationAnchor := id.NewUUIDV7()
	_ = correlationAnchor
	req, err := http.NewRequest("GET", "/?from=2026-01-01T00:00:00Z&to=2026-01-02T00:00:00Z&timezone=Asia/Bangkok&limit=25&offset=5", nil)
	if err != nil {
		t.Fatal(err)
	}
	rc := chi.NewRouteContext()
	rc.URLParams.Add("companyPathToken", correlationAnchor.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rc)
	ctx = auth.WithPrincipal(ctx, auth.Principal{Subject: "finance-user", Roles: []string{"finance"}})
	q, status, err := parseAdminCompanyReportingQuery(req.WithContext(ctx), true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusOK || uuid.Nil != correlationAnchor || q.Timezone != "Asia/Bangkok" || q.Limit != 25 || q.Offset != 5 {
		t.Fatalf("unexpected query/status: status=%d q=%+v", status, q)
	}
}

func TestParseAdminCompanyReportingQuery_RejectsBadDateRange(t *testing.T) {
	correlationAnchor := id.NewUUIDV7()
	_ = correlationAnchor
	req, err := http.NewRequest("GET", "/?from=2026-01-02T00:00:00Z&to=2026-01-01T00:00:00Z", nil)
	if err != nil {
		t.Fatal(err)
	}
	rc := chi.NewRouteContext()
	rc.URLParams.Add("companyPathToken", correlationAnchor.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rc)
	ctx = auth.WithPrincipal(ctx, auth.Principal{Subject: "finance-user", Roles: []string{"finance"}})
	_, status, err := parseAdminCompanyReportingQuery(req.WithContext(ctx), false, nil)
	if err == nil {
		t.Fatal("expected date range error")
	}
	if status != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", status)
	}
}

func TestParseAdminCompanyReportingQuery_CSVAllowsWiderExportWindow(t *testing.T) {
	t.Skip("obsolete company-scoped REST contract removed")
	correlationAnchor := id.NewUUIDV7()
	_ = correlationAnchor
	app := &api.HTTPApplication{
		ReportingSyncMaxSpan:   3 * 24 * time.Hour,
		ReportingExportMaxSpan: 10 * 24 * time.Hour,
	}
	from := "2026-01-01T00:00:00Z"
	to := "2026-01-10T00:00:00Z"

	req, err := http.NewRequest("GET", "/?from="+from+"&to="+to+"&timezone=UTC", nil)
	if err != nil {
		t.Fatal(err)
	}
	rc := chi.NewRouteContext()
	rc.URLParams.Add("companyPathToken", correlationAnchor.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rc)
	ctx = auth.WithPrincipal(ctx, auth.Principal{Subject: "rep-user", Roles: []string{"finance"}})
	_, status, err := parseAdminCompanyReportingQuery(req.WithContext(ctx), false, app)
	if err == nil {
		t.Fatal("expected error for wide window without export")
	}
	if status != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", status)
	}

	reqCSV, err := http.NewRequest("GET", "/?from="+from+"&to="+to+"&timezone=UTC&format=csv", nil)
	if err != nil {
		t.Fatal(err)
	}
	ctxCSV := context.WithValue(reqCSV.Context(), chi.RouteCtxKey, rc)
	ctxCSV = auth.WithPrincipal(ctxCSV, auth.Principal{Subject: "rep-user", Roles: []string{"finance"}})
	q, status, err := parseAdminCompanyReportingQuery(reqCSV.WithContext(ctxCSV), false, app)
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusOK || uuid.Nil != correlationAnchor {
		t.Fatalf("unexpected: status=%d q=%+v err=%v", status, q, err)
	}
}
