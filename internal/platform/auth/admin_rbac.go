package auth

import (
	"github.com/google/uuid"
)

// CanAccessOrganizationAdminData reports whether an interactive principal may target organization
// orgID on tenant-scoped admin APIs (user directory, sessions, etc.).
//
// Rules:
//   - platform_admin: any organization.
//   - Other roles: only when the JWT carries the same organization_id as orgID (org-bound tokens).
func CanAccessOrganizationAdminData(p Principal, orgID uuid.UUID) bool {
	if orgID == uuid.Nil {
		return false
	}
	if p.HasRole(RolePlatformAdmin) {
		return true
	}
	return p.OrganizationID == orgID
}
