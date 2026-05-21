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

// requireCatalogPrincipalUUID ensures an authenticated principal is present for catalog admin routes
// and resolves the single-company catalog/media scope when MEDIA_COMPANY_ID is configured.
// Resolution order matches product-image upload: explicit admin override → MEDIA_COMPANY_ID.
// When media company is not configured, returns uuid.Nil (legacy single-tenant catalog without media scope).
func requireCatalogPrincipalUUID(r *http.Request, mediaSvc *appmediaadmin.Service) (uuid.UUID, error) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return uuid.Nil, fmt.Errorf("missing principal")
	}
	if mediaSvc == nil {
		return uuid.Nil, nil
	}
	id, err := resolveProductImageCompanyID(r, p, mediaSvc)
	if err == nil {
		return id, nil
	}
	if errors.Is(err, errMediaCompanyNotConfigured) {
		return uuid.Nil, nil
	}
	return uuid.Nil, err
}

// requireCatalogMediaCompanyScope requires MEDIA_COMPANY_ID (or admin override) for media-bound catalog writes.
func requireCatalogMediaCompanyScope(r *http.Request, mediaSvc *appmediaadmin.Service) (uuid.UUID, error) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return uuid.Nil, fmt.Errorf("missing principal")
	}
	if mediaSvc == nil {
		return uuid.Nil, errMediaCompanyNotConfigured
	}
	return resolveProductImageCompanyID(r, p, mediaSvc)
}

// resolveProductImageCompanyID picks the media company id for product image upload and catalog media scope.
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
