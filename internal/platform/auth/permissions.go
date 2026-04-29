package auth

import (
	"strings"
)

// Fine-grained permission strings for interactive /v1 RBAC (JWT roles map to permission sets).
// Canonical form uses colon namespaces (e.g. user:read, catalog:write). Legacy dotted strings from
// older deployments are accepted in HasPermission via legacyPermissionCanonical.
//
// RBAC overview: platform_admin → admin.all and may target any organization when HTTP handlers supply an explicit
// organizationId / organization_id query (see CanAccessOrganizationAdminData). org_admin receives orgScopedPermissions
// and is limited to JWT org scope. Specialized roles map to subsets in permissionsByNormalizedRole.
const (
	PermAdminAll = "admin.all"

	// Interactive account / directory administration.
	PermUserRead           = "user:read"
	PermUserWrite          = "user:write"
	PermUserRoles          = "user:roles"
	PermUserSessionsRevoke = "user:sessions:revoke"

	// Aliases — prefer PermUser* in new code.
	PermAuthUserRead   = PermUserRead
	PermAuthUserWrite  = PermUserWrite
	PermAuthRoleWrite  = PermUserRoles
	PermAuthUsersRead  = PermUserRead
	PermAuthUsersWrite = PermUserWrite

	PermCatalogRead   = "catalog:read"
	PermCatalogWrite  = "catalog:write"
	PermCatalogDelete = "catalog:delete"

	PermFleetRead  = "fleet:read"
	PermFleetWrite = "fleet:write"

	// Site scope uses the same fleet tenant gates as other fleet routes.
	PermSiteRead  = "fleet:read"
	PermSiteWrite = "fleet:write"

	PermTechnicianOperate = "technician:operate"
	PermTechnicianRead    = PermTechnicianOperate
	PermTechnicianWrite   = PermTechnicianOperate

	PermInventoryRead   = "inventory:read"
	PermInventoryAdjust = "inventory:write"
	PermInventoryWrite  = PermInventoryAdjust

	PermCommerceRead = "commerce:read"

	PermPaymentRead = "payment:read"

	PermRefundsWrite = "payment:refund"

	PermCashRead  = "cash:read"
	PermCashWrite = "cash:write"

	PermReportsRead = "report:read"

	PermTelemetryRead = "telemetry:read"

	PermDeviceCommandsWrite = "machine:command"

	PermSetupWrite = "setup:machine"

	PermOTARead  = "ota:read"
	PermOTAWrite = "ota:write"

	PermAuditRead = "audit:read"

	PermMediaRead  = "media:read"
	PermMediaWrite = "media:write"
)

// legacyPermissionCanonical maps historical permission strings to current canonical values.
var legacyPermissionCanonical = map[string]string{
	"auth.user.read":         PermUserRead,
	"auth.user.write":        PermUserWrite,
	"auth.role.write":        PermUserRoles,
	"catalog.product.read":   PermCatalogRead,
	"catalog.product.write":  PermCatalogWrite,
	"catalog.product.delete": PermCatalogDelete,
	"fleet.machine.read":     PermFleetRead,
	"fleet.machine.write":    PermFleetWrite,
	"fleet.site.read":        PermSiteRead,
	"fleet.site.write":       PermSiteWrite,
	"technician.read":        PermTechnicianOperate,
	"technician.write":       PermTechnicianOperate,
	"inventory.read":         PermInventoryRead,
	"inventory.adjust":       PermInventoryAdjust,
	"commerce.order.read":    PermCommerceRead,
	"payment.read":           PermPaymentRead,
	"payment.refund":         PermRefundsWrite,
	"cash.read":              PermCashRead,
	"cash.write":             PermCashWrite,
	"reports.read":           PermReportsRead,
	"telemetry.read":         PermTelemetryRead,
	"command.dispatch":       PermDeviceCommandsWrite,
	"setup.write":            PermSetupWrite,
	"ota.read":               PermOTARead,
	"ota.write":              PermOTAWrite,
	"audit.read":             PermAuditRead,
	"media.read":             PermMediaRead,
	"media.write":            PermMediaWrite,
}

func canonicalPermission(p string) string {
	if p == "" {
		return ""
	}
	if c, ok := legacyPermissionCanonical[p]; ok {
		return c
	}
	return p
}

// orgScopedPermissions is the full interactive capability set for organization administrators.
var orgScopedPermissions = []string{
	PermUserRead, PermUserWrite, PermUserRoles, PermUserSessionsRevoke,
	PermCatalogRead, PermCatalogWrite, PermCatalogDelete,
	PermMediaRead, PermMediaWrite,
	PermFleetRead, PermFleetWrite,
	PermSiteRead, PermSiteWrite,
	PermTechnicianRead, PermTechnicianWrite,
	PermInventoryRead, PermInventoryAdjust,
	PermCommerceRead, PermPaymentRead, PermRefundsWrite,
	PermCashRead, PermCashWrite,
	PermReportsRead,
	PermTelemetryRead,
	PermDeviceCommandsWrite,
	PermSetupWrite,
	PermOTARead, PermOTAWrite,
	PermAuditRead,
}

// viewerPermissions is read-only interactive access for tenant dashboards (no user administration).
var viewerPermissions = []string{
	PermCatalogRead,
	PermFleetRead,
	PermSiteRead,
	PermTechnicianRead,
	PermInventoryRead,
	PermCommerceRead,
	PermPaymentRead,
	PermCashRead,
	PermReportsRead,
	PermTelemetryRead,
	PermMediaRead,
	PermOTARead,
}

// orgMemberPermissions matches viewerPermissions: safe read-only org member baseline (JWT role org_member).
var orgMemberPermissions = append([]string(nil), viewerPermissions...)

var permissionsByNormalizedRole = map[string][]string{
	RolePlatformAdmin: {PermAdminAll},
	RoleOrgAdmin:      append([]string(nil), orgScopedPermissions...),
	RoleOrgMember:     orgMemberPermissions,
	"finance_admin": {
		PermCommerceRead, PermPaymentRead, PermRefundsWrite,
		PermCashRead, PermCashWrite,
		PermReportsRead,
	},
	"finance": {
		PermCommerceRead, PermPaymentRead, PermRefundsWrite,
		PermCashRead, PermCashWrite,
		PermReportsRead,
	},
	"support": {
		PermFleetRead, PermSiteRead, PermTechnicianRead,
		PermInventoryRead, PermCommerceRead, PermPaymentRead,
		PermReportsRead, PermTelemetryRead, PermAuditRead,
	},
	"catalog_manager": {
		PermCatalogRead, PermCatalogWrite, PermCatalogDelete,
		PermMediaRead, PermMediaWrite,
	},
	"fleet_manager": {
		PermFleetRead, PermFleetWrite,
		PermSiteRead, PermSiteWrite,
		PermTechnicianRead, PermTechnicianWrite,
		PermDeviceCommandsWrite,
		PermInventoryRead,
	},
	"technician_manager": {
		PermFleetRead, PermFleetWrite,
		PermSiteRead,
		PermTechnicianRead, PermTechnicianWrite,
		PermInventoryRead, PermInventoryAdjust,
		PermSetupWrite,
		PermTelemetryRead,
		PermDeviceCommandsWrite,
	},
	"inventory_manager": {
		PermInventoryRead, PermInventoryAdjust,
		PermFleetRead,
	},
	RoleTechnician: {
		PermFleetRead,
		PermTechnicianRead,
		PermInventoryRead,
		PermSetupWrite,
		PermTelemetryRead,
		PermDeviceCommandsWrite,
	},
	"viewer": viewerPermissions,
}

func normalizeRoleKey(role string) string {
	return strings.TrimSpace(strings.ToLower(role))
}

// PermissionsForRole expands one JWT role string into granted permissions (may be empty).
func PermissionsForRole(role string) []string {
	perms, ok := permissionsByNormalizedRole[normalizeRoleKey(role)]
	if !ok {
		return nil
	}
	out := make([]string, len(perms))
	copy(out, perms)
	return out
}

// HasPermission reports whether the principal has want via JWT roles (union). PermAdminAll satisfies any want.
// Legacy dotted permission strings (e.g. catalog.product.read) still match after canonicalization.
func HasPermission(p Principal, want string) bool {
	if want == "" {
		return false
	}
	wantCanon := canonicalPermission(want)
	for _, role := range p.Roles {
		for _, perm := range PermissionsForRole(role) {
			if perm == PermAdminAll || canonicalPermission(perm) == wantCanon {
				return true
			}
		}
	}
	return false
}

// HasAnyPermission reports whether the principal holds any of wants.
func HasAnyPermission(p Principal, wants ...string) bool {
	for _, w := range wants {
		if HasPermission(p, w) {
			return true
		}
	}
	return false
}

// CanFleetMachineLifecycle reports whether the principal may disable, retire, or rotate credentials
// for machines. Requires fleet write plus platform_admin, org_admin, or fleet_manager.
// Other roles (e.g. technician_manager) may hold fleet write for provisioning but must not
// perform destructive lifecycle operations until finer-grained fleet permissions exist.
func CanFleetMachineLifecycle(p Principal) bool {
	if !HasPermission(p, PermFleetWrite) {
		return false
	}
	if p.HasRole(RolePlatformAdmin) || p.HasRole(RoleOrgAdmin) || p.HasRole("fleet_manager") {
		return true
	}
	return false
}
