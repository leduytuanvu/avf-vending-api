package httpserver

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	appmediaadmin "github.com/avf/avf-vending-api/internal/app/mediaadmin"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
)

var errMediaCompanyNotConfigured = errors.New("media company id not configured")

// requireCatalogPrincipalUUID ensures an authenticated principal is present for catalog admin routes.
// Single-company deployments do not accept scope query parameters.
func requireCatalogPrincipalUUID(r *http.Request) (uuid.UUID, error) {
	if _, ok := auth.PrincipalFromContext(r.Context()); !ok {
		return uuid.Nil, fmt.Errorf("missing principal")
	}
	return uuid.Nil, nil
}

// resolveProductImageCompanyID picks the media company id for product image upload.
// Resolution order: explicit request override (admin/platform_admin) → configured MEDIA_COMPANY_ID.
func resolveProductImageCompanyID(r *http.Request, p auth.Principal, svc *appmediaadmin.Service) (uuid.UUID, error) {
	for _, key := range []string{"company_id", "companyId", "mediaCompanyId"} {
		raw := strings.TrimSpace(r.FormValue(key))
		if raw == "" {
			raw = strings.TrimSpace(r.URL.Query().Get(key))
		}
		if raw == "" {
			continue
		}
		if !p.HasAnyRole(auth.RolePlatformAdmin, auth.RoleOrgAdmin) {
			return uuid.Nil, fmt.Errorf("forbidden company override")
		}
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil {
			return uuid.Nil, fmt.Errorf("invalid company_id")
		}
		return id, nil
	}
	if svc != nil {
		if id := svc.ConfiguredMediaCompanyID(); id != uuid.Nil {
			return id, nil
		}
	}
	return uuid.Nil, errMediaCompanyNotConfigured
}
