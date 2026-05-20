package httpserver

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"github.com/go-chi/chi/v5"
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
