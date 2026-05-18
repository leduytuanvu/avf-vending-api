package httpserver

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestParseAdminFleetCompanyScope_orgAdminCrossCompanyDenied(t *testing.T) {
	t.Skip("obsolete company-scoped REST contract removed")
	t.Parallel()
	orgMine := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	_ = orgMine
	orgOther := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("companyPathToken", orgOther.String())
	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	p := auth.Principal{
		Subject: uuid.NewString(),
		Roles:   []string{auth.RoleOrgAdmin},
	}
	req = req.WithContext(auth.WithPrincipal(req.Context(), p))
	if _, err := parseAdminFleetCompanyScope(req); err == nil {
		t.Fatal("expected company scope error for cross-org path company token")
	}
}

func TestParseAdminFleetCompanyScope_orgAdminSameCompany(t *testing.T) {
	t.Skip("obsolete company-scoped REST contract removed")
	t.Parallel()
	org := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	_ = org
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("companyPathToken", org.String())
	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	p := auth.Principal{
		Subject: uuid.NewString(),
		Roles:   []string{auth.RoleOrgAdmin},
	}
	req = req.WithContext(auth.WithPrincipal(req.Context(), p))
	got, err := parseAdminFleetCompanyScope(req)
	if err != nil || got != org {
		t.Fatalf("got (%v, %v) want (%v, nil)", got, err, org)
	}
}
