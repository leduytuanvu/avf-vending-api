package auth

import "github.com/google/uuid"

// CanAccessCompanyAdminData is kept for legacy route call sites during the
// single-company migration. Access is role/permission based only; scopeID is ignored
// except that a nil UUID still indicates a malformed legacy request.
func CanAccessCompanyAdminData(p Principal, scopeID uuid.UUID) bool {
	if scopeID == uuid.Nil {
		return false
	}
	return p.CanAccessAdminRoutes()
}
