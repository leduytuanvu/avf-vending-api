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

func TestCatalogOrg_pathOrganization_denies_orgAdmin_crossTenantPath(t *testing.T) {
	t.Parallel()
	mine := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	theirs := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	r := httptest.NewRequest("GET", "/v1/admin/organizations/"+theirs.String()+"/noop", nil)
	rc := chi.NewRouteContext()
	rc.URLParams.Add("organizationId", theirs.String())
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rc))

	p := auth.Principal{Roles: []string{auth.RoleOrgAdmin}, OrganizationID: mine, Subject: "op-1"}
	r = r.WithContext(auth.WithPrincipal(r.Context(), p))

	_, err := adminCatalogOrganizationID(r)
	if err == nil || !strings.Contains(err.Error(), "organization scope mismatch") {
		t.Fatalf("expected organization scope mismatch, got %v", err)
	}
}
