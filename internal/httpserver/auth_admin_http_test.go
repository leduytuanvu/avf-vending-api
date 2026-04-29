package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/avf/avf-vending-api/internal/app/api"
	appauth "github.com/avf/avf-vending-api/internal/app/auth"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func TestMountV1_adminAuthUsersRoutesRegistered(t *testing.T) {
	t.Parallel()
	r := chi.NewRouter()
	// Wire a non-nil Auth service so admin auth routes register (handlers are not invoked by chi.Walk).
	app := &api.HTTPApplication{Auth: &appauth.Service{}}
	cfg := &config.Config{}
	writeRL := func(h http.Handler) http.Handler { return h }
	mountV1(r, app, zap.NewNop(), cfg, stubAccessTokenValidator{}, writeRL, nil)

	var routes []string
	if err := chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes = append(routes, method+" "+route)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"GET /v1/admin/auth/users",
		"POST /v1/admin/auth/users",
		"GET /v1/admin/auth/users/{accountId}",
		"PATCH /v1/admin/auth/users/{accountId}",
		"PATCH /v1/admin/auth/users/{accountId}/status",
		"POST /v1/admin/auth/users/{accountId}/activate",
		"POST /v1/admin/auth/users/{accountId}/deactivate",
		"POST /v1/admin/auth/users/{accountId}/reset-password",
		"POST /v1/admin/auth/users/{accountId}/revoke-sessions",
		"POST /v1/admin/auth/users/{accountId}/roles",
		"PUT /v1/admin/auth/users/{accountId}/roles",
		"PATCH /v1/admin/auth/users/{accountId}/roles",
		"GET /v1/admin/users",
		"POST /v1/admin/users",
		"GET /v1/admin/users/{userId}",
		"PATCH /v1/admin/users/{userId}",
		"PATCH /v1/admin/users/{userId}/status",
		"POST /v1/admin/users/{userId}/roles",
		"PUT /v1/admin/users/{userId}/roles",
		"PATCH /v1/admin/users/{userId}/roles",
		"POST /v1/admin/users/{userId}/enable",
		"POST /v1/admin/users/{userId}/disable",
		"POST /v1/admin/users/{userId}/revoke-sessions",
		"POST /v1/admin/users/{userId}/reset-password",
		"GET /v1/admin/organizations/{organizationId}/users",
		"POST /v1/admin/organizations/{organizationId}/users",
		"GET /v1/admin/organizations/{organizationId}/users/{userId}",
		"PATCH /v1/admin/organizations/{organizationId}/users/{userId}",
		"PATCH /v1/admin/organizations/{organizationId}/users/{userId}/status",
		"POST /v1/admin/organizations/{organizationId}/users/{userId}/roles",
		"PATCH /v1/admin/organizations/{organizationId}/users/{userId}/roles",
		"POST /v1/admin/organizations/{organizationId}/users/{userId}/enable",
		"POST /v1/admin/organizations/{organizationId}/users/{userId}/disable",
		"POST /v1/admin/organizations/{organizationId}/users/{userId}/revoke-sessions",
		"POST /v1/admin/organizations/{organizationId}/users/{userId}/reset-password",
		"POST /v1/auth/change-password",
		"POST /v1/auth/password/change",
		"POST /v1/auth/password/reset/request",
		"POST /v1/auth/password/reset/confirm",
	}
	for _, w := range want {
		var found bool
		for _, got := range routes {
			if strings.Contains(got, w) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing route pattern %q in:\n%s", w, strings.Join(routes, "\n"))
		}
	}
}

func TestAdminAuthOrganizationID_orgAdminCrossTenantDenied(t *testing.T) {
	t.Parallel()
	orgMine := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	orgOther := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("organizationId", orgOther.String())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	p := auth.Principal{
		Subject:        uuid.NewString(),
		Roles:          []string{auth.RoleOrgAdmin},
		OrganizationID: orgMine,
	}
	req = req.WithContext(auth.WithPrincipal(req.Context(), p))
	if _, err := adminAuthOrganizationID(req); err == nil {
		t.Fatal("expected organization scope mismatch for org_admin")
	}
}

func TestAdminAuthOrganizationID_orgAdminSameTenant(t *testing.T) {
	t.Parallel()
	org := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("organizationId", org.String())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	p := auth.Principal{
		Subject:        uuid.NewString(),
		Roles:          []string{auth.RoleOrgAdmin},
		OrganizationID: org,
	}
	req = req.WithContext(auth.WithPrincipal(req.Context(), p))
	got, err := adminAuthOrganizationID(req)
	if err != nil || got != org {
		t.Fatalf("got (%v, %v) want (%v, nil)", got, err, org)
	}
}
