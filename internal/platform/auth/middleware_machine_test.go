package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestRequireDenyMachinePrincipal_allowsNonMachine(t *testing.T) {
	t.Parallel()
	h := RequireDenyMachinePrincipal(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/machines", nil)
	orgID := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	req = req.WithContext(WithPrincipal(req.Context(), Principal{
		Subject:        "op-1",
		Roles:          []string{RoleOrgAdmin},
		OrganizationID: orgID,
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTeapot {
		t.Fatalf("got status %d", rec.Code)
	}
}

func TestRequireDenyMachinePrincipal_blocksMachineRole(t *testing.T) {
	t.Parallel()
	var nextCalled bool
	h := RequireDenyMachinePrincipal(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		nextCalled = true
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/machines", nil)
	req = req.WithContext(WithPrincipal(req.Context(), Principal{
		Subject: "dev-1",
		Roles:   []string{RoleMachine},
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("got status %d want 403", rec.Code)
	}
	if nextCalled {
		t.Fatal("downstream handler was invoked for machine principal")
	}
}

func TestP06_Auth_MachineJWTBlockedFromAdminREST(t *testing.T) {
	t.Parallel()
	var nextCalled bool
	h := RequireDenyMachinePrincipal(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		nextCalled = true
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/machines", nil)
	req = req.WithContext(WithPrincipal(req.Context(), Principal{
		Subject: "m-1",
		Roles:   []string{RoleMachine},
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("got status %d want 403", rec.Code)
	}
	if nextCalled {
		t.Fatal("machine JWT must not reach admin REST handlers")
	}
}
