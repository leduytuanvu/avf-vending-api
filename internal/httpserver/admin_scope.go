package httpserver

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// adminCatalogOrganization resolves which organization an admin-catalog-style route targets.
//
// Layers (all must succeed before handlers run):
//
// • JWT middleware (`/v1` bearer access) rejects missing/invalid tokens (401).
// • `RequireInteractiveAccountActive`, `RequireDenyMachinePrincipal`, mutation rate-limit, and per-route RBAC run in `mountV1`/`mountAdmin*`.
// • This resolver enforces org/tenant mismatch for non-platform admins (typically HTTP 400 `invalid_scope`); callers should not assume cross-tenant writes ever succeed silently.
//
// Prefer path-scoped `{organizationId}` routes when migrating new admin surfaces — they simplify operator mental models versus query-only selectors.
func adminCatalogOrganizationID(r *http.Request) (uuid.UUID, error) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return uuid.Nil, fmt.Errorf("missing principal")
	}
	if raw := strings.TrimSpace(chi.URLParam(r, "organizationId")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil {
			return uuid.Nil, fmt.Errorf("invalid organizationId path parameter")
		}
		if !p.HasRole(auth.RolePlatformAdmin) && (!p.HasOrganization() || p.OrganizationID != id) {
			return uuid.Nil, fmt.Errorf("organization scope mismatch")
		}
		return id, nil
	}
	if p.HasRole(auth.RolePlatformAdmin) {
		raw := strings.TrimSpace(r.URL.Query().Get("organization_id"))
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil {
			return uuid.Nil, fmt.Errorf("organization_id query is required for platform administrators")
		}
		return id, nil
	}
	if !p.HasOrganization() {
		return uuid.Nil, fmt.Errorf("organization scope is required")
	}
	return p.OrganizationID, nil
}
