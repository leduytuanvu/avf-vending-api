package httpserver

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
)

// adminCatalogOrganization returns the organization id for catalog/inventory reads.
// platform_admin must pass organization_id query; org admins use the token org scope.
func adminCatalogOrganizationID(r *http.Request) (uuid.UUID, error) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return uuid.Nil, fmt.Errorf("missing principal")
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
