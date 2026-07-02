package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	appcatalogadmin "github.com/avf/avf-vending-api/internal/app/catalogadmin"
	"github.com/avf/avf-vending-api/internal/config"
	domainoperator "github.com/avf/avf-vending-api/internal/domain/operator"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Enterprise flow security rules (17) — contract tests via middleware and mount wiring.
func TestEnterpriseFlowSecurityRules(t *testing.T) {
	rules := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{"01_admin_no_bearer_401", rule01AdminNoBearer401},
		{"02_machine_jwt_admin_403", rule02MachineJWTAdmin403},
		{"03_user_jwt_admin_allowed_middleware", rule03UserJWTAdminAllowed},
		{"04_viewer_catalog_write_403", rule04ViewerCatalogWrite403},
		{"05_viewer_audit_read_403", rule05ViewerAuditRead403},
		{"06_finance_refunds_write_ok", rule06FinanceRefundsWriteOK},
		{"07_finance_catalog_write_403", rule07FinanceCatalogWrite403},
		{"08_catalog_manager_refunds_403", rule08CatalogManagerRefunds403},
		{"09_payment_providers_no_bearer_401", rule09PaymentProvidersNoBearer401},
		{"10_lifecycle_machine_jwt_blocked", rule10LifecycleMachineJWTBlocked},
		{"11_require_deny_machine_non_machine_ok", rule11NonMachinePassesDeny},
		{"12_operator_ended_reason_normalized", rule12EndedReasonNormalized},
		{"13_admin_interactive_inactive_blocked", rule13InactiveAccountBlocked},
		{"14_technician_read_fleet_ok", rule14TechnicianReadFleetOK},
		{"15_platform_admin_lifecycle_ok", rule15PlatformAdminLifecycleOK},
		{"16_enterprise_routes_no_bearer_401", rule16EnterpriseRoutesNoBearer401},
		{"17_public_activation_claim_no_admin_auth", rule17PublicClaimNoAdminAuth},
	}
	for _, rule := range rules {
		t.Run(rule.name, rule.fn)
	}
}

func rule01AdminNoBearer401(t *testing.T) {
	h := testMountV1ForAdminREST(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/machines", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d", rec.Code)
	}
}

func rule02MachineJWTAdmin403(t *testing.T) {
	var called bool
	h := auth.RequireDenyMachinePrincipal(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/machines", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{Roles: []string{auth.RoleMachine}}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || called {
		t.Fatalf("machine must be blocked from admin")
	}
}

func rule03UserJWTAdminAllowed(t *testing.T) {
	h := auth.RequireDenyMachinePrincipal(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{Roles: []string{auth.RoleOrgAdmin}}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("user admin should pass deny-machine middleware")
	}
}

func rule04ViewerCatalogWrite403(t *testing.T) {
	org := testFixtureCompanyUUID()
	app := testHTTPAppForRBAC(t)
	r, iss := testMountV1WithIssuer(t, app)
	tok, _, err := iss.IssueAccessJWT(
		uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		org,
		[]string{"viewer"},
		"active",
	)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/products", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer catalog write: %d", rec.Code)
	}
}

func rule05ViewerAuditRead403(t *testing.T) {
	p := auth.Principal{Subject: "550e8400-e29b-41d4-a716-446655440099", Roles: []string{"viewer"}}
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/audit/events", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), p))
	h := auth.RequireAnyPermission(auth.PermAuditRead)(okHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("audit read: got %d want 403", rec.Code)
	}
}

func rule06FinanceRefundsWriteOK(t *testing.T) {
	p := auth.Principal{Roles: []string{"finance_admin"}}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), p))
	h := auth.RequireAnyPermission(auth.PermRefundsWrite)(okHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("refunds write: %d", rec.Code)
	}
}

func rule07FinanceCatalogWrite403(t *testing.T) {
	p := auth.Principal{Roles: []string{"finance_admin"}}
	req := httptest.NewRequest(http.MethodPatch, "/", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), p))
	h := auth.RequireAnyPermission(auth.PermCatalogWrite)(okHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("finance catalog write: %d", rec.Code)
	}
}

func rule08CatalogManagerRefunds403(t *testing.T) {
	p := auth.Principal{Subject: "550e8400-e29b-41d4-a716-446655440001", Roles: []string{"catalog_manager"}}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), p))
	h := auth.RequireAnyPermission(auth.PermRefundsWrite)(okHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("refunds: got %d want 403", rec.Code)
	}
}

func rule09PaymentProvidersNoBearer401(t *testing.T) {
	h := testMountV1ForAdminREST(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/payment/providers", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("payment providers: %d", rec.Code)
	}
}

func rule10LifecycleMachineJWTBlocked(t *testing.T) {
	var called bool
	h := auth.RequireDenyMachinePrincipal(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/machines/x/suspend", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{Roles: []string{auth.RoleMachine}}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || called {
		t.Fatal("machine JWT must not reach lifecycle handlers")
	}
}

func rule11NonMachinePassesDeny(t *testing.T) {
	h := auth.RequireDenyMachinePrincipal(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/machines", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{Roles: []string{auth.RoleOrgAdmin}}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTeapot {
		t.Fatalf("non-machine principal should pass: %d", rec.Code)
	}
}

func rule12EndedReasonNormalized(t *testing.T) {
	if domainoperator.NormalizeEndedReason("admin_forced_takeover") != domainoperator.EndedReasonSupersededByAdminTakeover {
		t.Fatal("legacy takeover alias not normalized")
	}
	if domainoperator.NormalizeEndedReason("") != domainoperator.EndedReasonUnknown {
		t.Fatal("empty ended_reason should be unknown")
	}
}

func rule13InactiveAccountBlocked(t *testing.T) {
	h := auth.RequireInteractiveAccountActive(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{AccountStatus: "disabled"}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("inactive account must be forbidden: %d", rec.Code)
	}
}

func rule14TechnicianReadFleetOK(t *testing.T) {
	p := auth.Principal{Roles: []string{"technician"}}
	h := auth.RequireAnyPermission(auth.PermFleetRead, auth.PermTechnicianRead)(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), p))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("technician fleet read: %d", rec.Code)
	}
}

func rule15PlatformAdminLifecycleOK(t *testing.T) {
	p := auth.Principal{Roles: []string{auth.RolePlatformAdmin}}
	h := auth.RequireFleetMachineLifecycle(okHandler())
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), p))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("platform admin lifecycle: %d", rec.Code)
	}
}

func rule16EnterpriseRoutesNoBearer401(t *testing.T) {
	h := testMountV1ForAdminREST(t)
	paths := []string{
		"/v1/admin/machines/11111111-1111-1111-1111-111111111111/runtime-sessions/current",
		"/v1/admin/machines/11111111-1111-1111-1111-111111111111/ops-overview",
		"/v1/admin/machines/11111111-1111-1111-1111-111111111111/timeline/unified",
	}
	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s: got %d", path, rec.Code)
		}
	}
}

func rule17PublicClaimNoAdminAuth(t *testing.T) {
	h := testMountV1ForAdminREST(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/setup/activation-codes/claim", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code == http.StatusForbidden {
		t.Fatal("public claim must not require admin bearer")
	}
}

func testHTTPAppForRBAC(t *testing.T) *api.HTTPApplication {
	t.Helper()
	return &api.HTTPApplication{CatalogAdmin: new(appcatalogadmin.Service)}
}

func testMountV1WithIssuer(t *testing.T, app *api.HTTPApplication) (http.Handler, *auth.SessionIssuer) {
	t.Helper()
	secret := testJWTSecret(t)
	iss := testSessionIssuer(t, secret)
	r := chi.NewRouter()
	cfg := &config.Config{}
	cfg.TransportBoundary.MachineRESTLegacyEnabled = true
	v := auth.NewHS256AccessTokenValidator(secret, 45*time.Second)
	writeRL := func(h http.Handler) http.Handler { return h }
	mountV1(r, app, zap.NewNop(), cfg, v, writeRL, nil)
	return r, iss
}
