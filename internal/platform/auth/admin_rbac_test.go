package auth_test

import (
	"testing"

	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
)

func TestCanAccessCompanyAdminData(t *testing.T) {
	t.Parallel()
	orgA := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	pBO := auth.Principal{Roles: []string{auth.RoleOrgAdmin}}

	if auth.CanAccessCompanyAdminData(pBO, orgA) != true {
		t.Fatal("admin should access single-company data")
	}
	pPA := auth.Principal{Roles: []string{auth.RolePlatformAdmin}}
	if auth.CanAccessCompanyAdminData(pPA, orgA) != true {
		t.Fatal("platform_admin should access any org")
	}
}
