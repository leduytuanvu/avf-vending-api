package httpserver

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestParseAdminFleetOrganizationScope_orgAdminCrossTenantDenied(t *testing.T) {
	t.Parallel()
	orgMine := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	orgOther := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("organizationId", orgOther.String())
	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	p := auth.Principal{
		Subject:        uuid.NewString(),
		Roles:          []string{auth.RoleOrgAdmin},
		OrganizationID: orgMine,
	}
	req = req.WithContext(auth.WithPrincipal(req.Context(), p))
	if _, err := parseAdminFleetOrganizationScope(req); err == nil {
		t.Fatal("expected tenant scope error for cross-org path organizationId")
	}
}

func TestParseAdminFleetOrganizationScope_orgAdminSameTenant(t *testing.T) {
	t.Parallel()
	org := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("organizationId", org.String())
	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	p := auth.Principal{
		Subject:        uuid.NewString(),
		Roles:          []string{auth.RoleOrgAdmin},
		OrganizationID: org,
	}
	req = req.WithContext(auth.WithPrincipal(req.Context(), p))
	got, err := parseAdminFleetOrganizationScope(req)
	if err != nil || got != org {
		t.Fatalf("got (%v, %v) want (%v, nil)", got, err, org)
	}
}
