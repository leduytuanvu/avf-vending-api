package auth

import (
	"testing"

	"github.com/google/uuid"
)

func TestHasPermission_adminAllBypass(t *testing.T) {
	org := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	p := Principal{Subject: "a", Roles: []string{RolePlatformAdmin}, OrganizationID: org}
	if !HasPermission(p, PermCatalogWrite) || !HasPermission(p, PermAuditRead) {
		t.Fatal("platform_admin must satisfy any permission via admin.all")
	}
}

func TestHasPermission_legacyDottedStillMatches(t *testing.T) {
	p := Principal{Roles: []string{"catalog_manager"}}
	if !HasPermission(p, "catalog.product.read") || !HasPermission(p, "catalog.product.write") {
		t.Fatal("legacy JWT permission strings must map to canonical RBAC")
	}
}

func TestHasPermission_orgAdminBroad(t *testing.T) {
	p := Principal{Roles: []string{RoleOrgAdmin}}
	for _, perm := range []string{
		PermCatalogWrite, PermRefundsWrite, PermFleetRead, PermOTAWrite, PermAuditRead,
		PermUserRoles, PermCatalogDelete, PermMediaWrite, PermUserSessionsRevoke,
	} {
		if !HasPermission(p, perm) {
			t.Fatalf("org_admin should have %s", perm)
		}
	}
}

func TestHasPermission_viewerReadOnly(t *testing.T) {
	p := Principal{Roles: []string{"viewer"}}
	for _, perm := range []string{
		PermCatalogRead, PermFleetRead, PermSiteRead, PermTechnicianRead,
		PermCommerceRead, PermPaymentRead, PermReportsRead, PermMediaRead,
	} {
		if !HasPermission(p, perm) {
			t.Fatalf("viewer should have %s", perm)
		}
	}
	for _, perm := range []string{PermCatalogWrite, PermUserWrite, PermRefundsWrite, PermFleetWrite, PermAuditRead, PermSiteWrite, PermUserRead} {
		if HasPermission(p, perm) {
			t.Fatalf("viewer must not have %s", perm)
		}
	}
}

func TestHasPermission_orgMemberMatchesViewerBaseline(t *testing.T) {
	v := Principal{Roles: []string{"viewer"}}
	m := Principal{Roles: []string{RoleOrgMember}}
	for _, perm := range []string{PermPaymentRead, PermMediaRead, PermSiteRead, PermCatalogRead} {
		if HasPermission(v, perm) != HasPermission(m, perm) {
			t.Fatalf("org_member and viewer should agree on %s", perm)
		}
	}
	if HasPermission(m, PermUserWrite) {
		t.Fatal("org_member must not write users")
	}
}

func TestHasPermission_catalogManagerCatalogOnly(t *testing.T) {
	p := Principal{Roles: []string{"catalog_manager"}}
	if !HasPermission(p, PermCatalogRead) || !HasPermission(p, PermCatalogWrite) {
		t.Fatal("catalog_manager needs catalog read/write")
	}
	if !HasPermission(p, PermCatalogDelete) || !HasPermission(p, PermMediaWrite) || !HasPermission(p, PermMediaRead) {
		t.Fatal("catalog_manager needs delete + media read/write")
	}
	if HasPermission(p, PermRefundsWrite) || HasPermission(p, PermFleetRead) {
		t.Fatal("catalog_manager must not have refunds or fleet")
	}
	if HasPermission(p, PermUserRead) {
		t.Fatal("catalog_manager must not administer users")
	}
}

func TestHasPermission_financeRefunds(t *testing.T) {
	p := Principal{Roles: []string{"finance"}}
	if !HasPermission(p, PermRefundsWrite) || !HasPermission(p, PermCommerceRead) || !HasPermission(p, PermPaymentRead) {
		t.Fatal("finance needs refunds, commerce read, and payment.read")
	}
	if HasPermission(p, PermCatalogWrite) || HasPermission(p, PermUserRead) {
		t.Fatal("finance must not manage catalog writes or users")
	}
}

func TestHasPermission_supportReadOnly(t *testing.T) {
	p := Principal{Roles: []string{"support"}}
	for _, perm := range []string{PermFleetRead, PermCommerceRead, PermAuditRead, PermTelemetryRead} {
		if !HasPermission(p, perm) {
			t.Fatalf("support should have %s", perm)
		}
	}
	if HasPermission(p, PermUserWrite) || HasPermission(p, PermUserRoles) || HasPermission(p, PermRefundsWrite) || HasPermission(p, PermUserRead) {
		t.Fatal("support must not mutate users, roles, or refunds")
	}
}

func TestHasPermission_technicianNoCatalogWrite(t *testing.T) {
	p := Principal{Roles: []string{RoleTechnician}}
	if !HasPermission(p, PermTechnicianOperate) {
		t.Fatal("technician should have technician:operate")
	}
	if !HasPermission(p, PermSetupWrite) || !HasPermission(p, PermDeviceCommandsWrite) {
		t.Fatal("technician needs setup and machine commands")
	}
	if HasPermission(p, PermCatalogWrite) || HasPermission(p, PermUserRead) {
		t.Fatal("technician must not write catalog or read users")
	}
}

func TestHasPermission_roleChangeUpdatesEffectivePermissions(t *testing.T) {
	u := Principal{Roles: []string{"viewer"}}
	a := Principal{Roles: []string{"catalog_manager"}}
	if HasPermission(u, PermCatalogWrite) || !HasPermission(a, PermCatalogWrite) {
		t.Fatal("viewer vs catalog_manager should differ on catalog write")
	}
}

func TestHasPermission_inventoryManagerFleetRead(t *testing.T) {
	p := Principal{Roles: []string{"inventory_manager"}}
	if !HasPermission(p, PermInventoryAdjust) || !HasPermission(p, PermFleetRead) {
		t.Fatal("inventory_manager needs inventory adjust and fleet read")
	}
	if HasPermission(p, PermCatalogWrite) {
		t.Fatal("inventory_manager must not write catalog")
	}
}

func TestHasPermission_fleetManager(t *testing.T) {
	p := Principal{Roles: []string{"fleet_manager"}}
	if !HasPermission(p, PermFleetWrite) || !HasPermission(p, PermDeviceCommandsWrite) {
		t.Fatal("fleet_manager needs fleet write and machine commands")
	}
	if !HasPermission(p, PermSiteRead) || !HasPermission(p, PermSiteWrite) ||
		!HasPermission(p, PermTechnicianRead) || !HasPermission(p, PermTechnicianWrite) {
		t.Fatal("fleet_manager needs site and technician scopes")
	}
	if !CanFleetMachineLifecycle(p) {
		t.Fatal("fleet_manager may run machine lifecycle mutations")
	}
	if HasPermission(p, PermCatalogWrite) {
		t.Fatal("fleet_manager must not write catalog")
	}
}

func TestHasPermission_technicianManagerLifecycle(t *testing.T) {
	p := Principal{Roles: []string{"technician_manager"}}
	if !HasPermission(p, PermFleetWrite) || !HasPermission(p, PermInventoryAdjust) {
		t.Fatal("technician_manager needs fleet write and inventory adjust")
	}
	if CanFleetMachineLifecycle(p) {
		t.Fatal("technician_manager must not pass lifecycle gate")
	}
}

func TestHasPermission_unknownRoleEmpty(t *testing.T) {
	p := Principal{Roles: []string{"unknown_role_xyz"}}
	if HasPermission(p, PermCatalogRead) {
		t.Fatal("unknown role grants nothing")
	}
}

func TestHasAnyPermission_union(t *testing.T) {
	p := Principal{Roles: []string{"viewer"}}
	if HasAnyPermission(p, PermCatalogWrite, PermCatalogRead) != true {
		t.Fatal("expected catalog:read")
	}
	if HasAnyPermission(p, PermCatalogWrite, PermRefundsWrite) {
		t.Fatal("viewer has neither")
	}
}
