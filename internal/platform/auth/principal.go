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
	// RoleMachine is issued to kiosk/device principals after activation; token must include machine_ids.
	RoleMachine = "machine"
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
	// AccountStatus comes from JWT claim account_status when present (e.g. interactive login JWTs). Empty means unknown/legacy and is treated as active for routing.
	AccountStatus string
	// JWTAudience is the raw JWT "aud" claim when a single audience was present (best-effort).
	JWTAudience string
	// JWTType is the JWT "typ" claim when present (e.g. machine for vending runtime tokens).
	JWTType string
	// JTI is the JWT ID claim when present (access-token revocation / logout).
	JTI string
	// TokenUse distinguishes interactive access vs MFA challenge JWTs when present.
	TokenUse string
	// MFAEnrollment is true when the MFA pending JWT was issued for first-time MFA provisioning.
	MFAEnrollment bool
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

// IsMachinePrincipal reports kiosk/runtime JWTs that must be blocked from admin/reporting routes.
func (p Principal) IsMachinePrincipal() bool {
	return p.HasRole(RoleMachine)
}

// CanAccessMachineRead is a coarse JWT claim check only; it does not prove tenant ownership.
// Prefer httpserver.RequireMachineTenantAccess for /v1 routes keyed by machineId (DB-backed org match).
func (p Principal) CanAccessMachineRead(machineID uuid.UUID) bool {
	if machineID == uuid.Nil {
		return false
	}
	if p.HasRole(RolePlatformAdmin) {
		return true
	}
	return p.AllowsMachine(machineID)
}

// InteractiveAccountDisabled reports whether this interactive principal is blocked by account status (JWT claim).
func (p Principal) InteractiveAccountDisabled() bool {
	if p.IsMachinePrincipal() {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(p.AccountStatus), "disabled")
}

// CanAccessAdminRoutes reports whether the principal has any mapped RBAC permission for interactive admin APIs.
func (p Principal) CanAccessAdminRoutes() bool {
	if p.IsMachinePrincipal() {
		return false
	}
	for _, role := range p.Roles {
		if len(PermissionsForRole(role)) > 0 {
			return true
		}
	}
	return false
}
