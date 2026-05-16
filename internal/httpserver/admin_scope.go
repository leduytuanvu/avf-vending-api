package httpserver

import (
	"fmt"
	"net/http"

	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
)

// adminCatalogScopeID is a legacy compatibility shim for service methods
// that still accept a scope UUID during the single-company migration. REST
// contracts no longer accept company path or query parameters.
func adminCatalogScopeID(r *http.Request) (uuid.UUID, error) {
	if _, ok := auth.PrincipalFromContext(r.Context()); !ok {
		return uuid.Nil, fmt.Errorf("missing principal")
	}
	return uuid.Nil, nil
}
