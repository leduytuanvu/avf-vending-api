package httpserver

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/avf/avf-vending-api/internal/platform/auth"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestCatalogOrg_pathCompany_denies_orgAdmin_crossCompanyPath(t *testing.T) {
	t.Skip("obsolete company-scoped REST contract removed")
	t.Parallel()
	mine := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	_ = mine
	theirs := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	r := httptest.NewRequest("GET", "/v1/admin/companies/"+theirs.String()+"/noop", nil)
	rc := chi.NewRouteContext()
	rc.URLParams.Add("companyPathToken", theirs.String())
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rc))

	p := auth.Principal{Roles: []string{auth.RoleOrgAdmin}}
	r = r.WithContext(auth.WithPrincipal(r.Context(), p))

	_, err := requireCatalogPrincipalUUID(r)
	if err == nil || !strings.Contains(err.Error(), "company scope mismatch") {
		t.Fatalf("expected company scope mismatch, got %v", err)
	}
}
