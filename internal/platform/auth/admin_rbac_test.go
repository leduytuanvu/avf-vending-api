package auth_test

import (
	"testing"

	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
)

func TestCanAccessOrganizationAdminData(t *testing.T) {
	t.Parallel()
	orgA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	orgB := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	pBO := auth.Principal{Roles: []string{auth.RoleOrgAdmin}, OrganizationID: orgB}

	if auth.CanAccessOrganizationAdminData(pBO, orgB) != true {
		t.Fatal("org_admin should access own org")
	}
	if auth.CanAccessOrganizationAdminData(pBO, orgA) != false {
		t.Fatal("org_admin must not access other org")
	}

	pPA := auth.Principal{Roles: []string{auth.RolePlatformAdmin}}
	if auth.CanAccessOrganizationAdminData(pPA, orgA) != true {
		t.Fatal("platform_admin should access any org")
	}
}
