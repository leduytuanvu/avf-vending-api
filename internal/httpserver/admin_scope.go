package httpserver

import (
	"fmt"
	"net/http"

	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
)

// requireCatalogPrincipalUUID ensures an authenticated principal is present for catalog admin routes.
// Single-company deployments do not accept scope query parameters.
func requireCatalogPrincipalUUID(r *http.Request) (uuid.UUID, error) {
	if _, ok := auth.PrincipalFromContext(r.Context()); !ok {
		return uuid.Nil, fmt.Errorf("missing principal")
	}
	return uuid.Nil, nil
}
