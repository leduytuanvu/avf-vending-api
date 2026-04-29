package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
)

func testOrgID() uuid.UUID {
	return uuid.MustParse("11111111-1111-1111-1111-111111111111")
}

func TestRBAC_viewerBlockedAuditRead(t *testing.T) {
	org := testOrgID()
	p := auth.Principal{
		Subject:        "550e8400-e29b-41d4-a716-446655440099",
		Roles:          []string{"viewer"},
		OrganizationID: org,
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/audit/events", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), p))
	h := auth.RequireAnyPermission(auth.PermAuditRead)(okHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("audit read: got %d want 403", rec.Code)
	}
}

func TestRBAC_viewerAllowedCatalogRead_blockedCatalogWrite(t *testing.T) {
	org := testOrgID()
	p := auth.Principal{
		Subject:        "550e8400-e29b-41d4-a716-446655440000",
		Roles:          []string{"viewer"},
		OrganizationID: org,
	}
	h := auth.RequireAnyPermission(auth.PermCatalogRead)(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), p))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("catalog read: %d", rec.Code)
	}

	h = auth.RequireAnyPermission(auth.PermCatalogWrite)(okHandler())
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("catalog write: got %d want 403", rec.Code)
	}
}

func TestRBAC_catalogManager_canCatalogWrite_notRefunds(t *testing.T) {
	org := testOrgID()
	p := auth.Principal{Subject: "550e8400-e29b-41d4-a716-446655440001", Roles: []string{"catalog_manager"}, OrganizationID: org}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), p))

	h := auth.RequireAnyPermission(auth.PermCatalogWrite)(okHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("catalog write: %d", rec.Code)
	}

	h = auth.RequireAnyPermission(auth.PermRefundsWrite)(okHandler())
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("refunds: got %d want 403", rec.Code)
	}
}

func TestRBAC_financeAdmin_canRefund(t *testing.T) {
	org := testOrgID()
	p := auth.Principal{Roles: []string{"finance_admin"}, OrganizationID: org}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), p))
	h := auth.RequireAnyPermission(auth.PermRefundsWrite)(okHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("refunds write: %d", rec.Code)
	}
}

func TestRBAC_financeAdmin_cannotCatalogWrite(t *testing.T) {
	org := testOrgID()
	p := auth.Principal{Roles: []string{"finance_admin"}, OrganizationID: org}
	req := httptest.NewRequest(http.MethodPatch, "/", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), p))
	h := auth.RequireAnyPermission(auth.PermCatalogWrite)(okHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("finance must not write catalog: got %d", rec.Code)
	}
}

func TestRBAC_fleetLifecycle_fleetManagerAllowed_technicianManagerBlocked(t *testing.T) {
	org := testOrgID()
	reqBase := func(roles ...string) *http.Request {
		p := auth.Principal{Roles: roles, OrganizationID: org}
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		return req.WithContext(auth.WithPrincipal(req.Context(), p))
	}

	h := auth.RequireFleetMachineLifecycle(okHandler())

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, reqBase("fleet_manager"))
	if rec.Code != http.StatusOK {
		t.Fatalf("fleet_manager: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, reqBase("technician_manager"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("technician_manager lifecycle: got %d want 403", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, reqBase(auth.RoleOrgAdmin))
	if rec.Code != http.StatusOK {
		t.Fatalf("org_admin: %d", rec.Code)
	}
}

func TestRBAC_technician_cannotCatalogWrite(t *testing.T) {
	org := testOrgID()
	p := auth.Principal{Roles: []string{auth.RoleTechnician}, OrganizationID: org}
	req := httptest.NewRequest(http.MethodPut, "/", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), p))
	h := auth.RequireAnyPermission(auth.PermCatalogWrite)(okHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("got %d want 403", rec.Code)
	}
}

func TestRBAC_technician_cannotTriggerOTAWrite(t *testing.T) {
	org := testOrgID()
	p := auth.Principal{Roles: []string{auth.RoleTechnician}, OrganizationID: org}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), p))
	h := auth.RequireAnyPermission(auth.PermOTAWrite)(okHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("got %d want 403", rec.Code)
	}
}

func TestRBAC_orgAdmin_passesCatalogAndRefunds(t *testing.T) {
	org := testOrgID()
	p := auth.Principal{Roles: []string{auth.RoleOrgAdmin}, OrganizationID: org}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), p))
	for _, perm := range []string{auth.PermCatalogWrite, auth.PermRefundsWrite, auth.PermFleetWrite} {
		h := auth.RequireAnyPermission(perm)(okHandler())
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d", perm, rec.Code)
		}
	}
}

func TestRBAC_machinePrincipal_bypassesInteractiveCommercePermission(t *testing.T) {
	mid := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	org := testOrgID()
	p := auth.Principal{Roles: []string{auth.RoleMachine}, MachineIDs: []uuid.UUID{mid}, OrganizationID: org}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), p))
	h := auth.RequireInteractivePermissionOrMachinePrincipal(auth.PermCommerceRead)(okHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("machine bypass: %d", rec.Code)
	}
}

func TestRBAC_interactiveAccountDisabled_rejected(t *testing.T) {
	org := testOrgID()
	p := auth.Principal{Roles: []string{"viewer"}, OrganizationID: org, AccountStatus: "disabled"}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), p))
	h := auth.RequireInteractiveAccountActive(okHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("got %d want 403", rec.Code)
	}
}

func TestRBAC_technician_cannotUserRead(t *testing.T) {
	org := testOrgID()
	p := auth.Principal{Roles: []string{auth.RoleTechnician}, OrganizationID: org}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), p))
	h := auth.RequireAnyPermission(auth.PermUserRead)(okHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("technician user admin: got %d want 403", rec.Code)
	}
}

func TestRBAC_support_cannotUserRead(t *testing.T) {
	org := testOrgID()
	p := auth.Principal{Roles: []string{"support"}, OrganizationID: org}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), p))
	h := auth.RequireAnyPermission(auth.PermUserRead)(okHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("support user admin: got %d want 403", rec.Code)
	}
}

func TestRBAC_orgAdmin_canUserReadAndSessionsRevoke(t *testing.T) {
	org := testOrgID()
	p := auth.Principal{Roles: []string{auth.RoleOrgAdmin}, OrganizationID: org}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), p))
	h := auth.RequireAnyPermission(auth.PermUserRead)(okHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("org_admin user read: %d", rec.Code)
	}
	h = auth.RequireAnyPermission(auth.PermUserSessionsRevoke)(okHandler())
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("org_admin sessions revoke: %d", rec.Code)
	}
}

func TestRBAC_technician_cannotFleetSiteWrite(t *testing.T) {
	org := testOrgID()
	p := auth.Principal{Roles: []string{auth.RoleTechnician}, OrganizationID: org}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), p))
	h := auth.RequireAnyPermission(auth.PermSiteWrite, auth.PermFleetWrite)(okHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("technician fleet write: got %d want 403", rec.Code)
	}
}

func TestRBAC_support_fleetReadOnly(t *testing.T) {
	org := testOrgID()
	p := auth.Principal{Roles: []string{"support"}, OrganizationID: org}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), p))
	h := auth.RequireAnyPermission(auth.PermFleetRead)(okHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("support fleet read: %d", rec.Code)
	}
	h = auth.RequireAnyPermission(auth.PermFleetWrite)(okHandler())
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("support must not fleet write: %d", rec.Code)
	}
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
}
