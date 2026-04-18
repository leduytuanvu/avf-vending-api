package auth

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Well-known role names for coarse route checks (maps to JWT "roles" array).
const (
	RolePlatformAdmin = "platform_admin"
	RoleOrgAdmin      = "org_admin"
	RoleOrgMember     = "org_member"
	RoleTechnician    = "technician"
	RoleService       = "service"
)

// ActorType is stored in audit_logs.actor_type.
const (
	ActorTypeUser    = "user"
	ActorTypeService = "service"
)

// Principal is the authenticated subject after token validation.
type Principal struct {
	Subject        string
	ActorType      string
	Roles          []string
	OrganizationID uuid.UUID
	SiteID         uuid.UUID
	MachineIDs     []uuid.UUID
	TechnicianID   uuid.UUID
	ExpiresAt      time.Time
}

// Actor returns stable audit tuple for this principal.
func (p Principal) Actor() (actorType string, actorID string) {
	if strings.TrimSpace(p.Subject) == "" {
		return ActorTypeUser, ""
	}
	if p.ActorType != "" {
		return p.ActorType, p.Subject
	}
	if p.HasRole(RoleService) {
		return ActorTypeService, p.Subject
	}
	return ActorTypeUser, p.Subject
}

// HasRole reports whether any role matches case-insensitively.
func (p Principal) HasRole(want string) bool {
	for _, r := range p.Roles {
		if strings.EqualFold(strings.TrimSpace(r), strings.TrimSpace(want)) {
			return true
		}
	}
	return false
}

// HasAnyRole reports whether the principal holds at least one of the wanted roles.
func (p Principal) HasAnyRole(want ...string) bool {
	for _, w := range want {
		if p.HasRole(w) {
			return true
		}
	}
	return false
}

// HasOrganization reports whether an organization scope is present.
func (p Principal) HasOrganization() bool {
	return p.OrganizationID != uuid.Nil
}

// HasSite reports whether a site scope is present.
func (p Principal) HasSite() bool {
	return p.SiteID != uuid.Nil
}

// AllowsMachine reports explicit machine allow-list from the token (device/technician scoping).
func (p Principal) AllowsMachine(machineID uuid.UUID) bool {
	if machineID == uuid.Nil {
		return false
	}
	for _, m := range p.MachineIDs {
		if m == machineID {
			return true
		}
	}
	return false
}

// CanAccessMachineRead gates per-machine reads at the HTTP edge.
// platform_admin may read any machine; org_admin with org scope may read fleet for that tenant
// (repositories must still verify machine.organization_id matches the principal org).
// Explicit machine_ids in the token always win for technicians and service accounts.
func (p Principal) CanAccessMachineRead(machineID uuid.UUID) bool {
	if machineID == uuid.Nil {
		return false
	}
	if p.HasRole(RolePlatformAdmin) {
		return true
	}
	if p.AllowsMachine(machineID) {
		return true
	}
	if p.HasOrganization() && p.HasRole(RoleOrgAdmin) {
		return true
	}
	return false
}

// CanAccessAdminRoutes is true for platform or org administrators.
func (p Principal) CanAccessAdminRoutes() bool {
	return p.HasAnyRole(RolePlatformAdmin, RoleOrgAdmin)
}
