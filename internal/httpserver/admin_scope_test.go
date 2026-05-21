package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	appmediaadmin "github.com/avf/avf-vending-api/internal/app/mediaadmin"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func testMediaAdminWithCompanyID(t *testing.T, companyID uuid.UUID) *appmediaadmin.Service {
	t.Helper()
	return appmediaadmin.TestServiceWithMediaCompanyID(companyID)
}

func TestResolveProductImageCompanyID_usesConfiguredMediaCompanyID(t *testing.T) {
	t.Parallel()
	companyID := uuid.MustParse("0194a1b2-c3d4-7890-abcd-ef1234567890")
	svc := testMediaAdminWithCompanyID(t, companyID)
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/product-images", nil)
	p := auth.Principal{Roles: []string{auth.RoleOrgAdmin}}

	got, err := resolveProductImageCompanyID(req, p, svc)
	require.NoError(t, err)
	require.Equal(t, companyID, got)
}

func TestResolveProductImageCompanyID_explicitOverrideAdmin(t *testing.T) {
	t.Parallel()
	configured := uuid.MustParse("0194a1b2-c3d4-7890-abcd-ef1234567890")
	override := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	svc := testMediaAdminWithCompanyID(t, configured)
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/product-images?company_id="+override.String(), nil)
	p := auth.Principal{Roles: []string{auth.RoleOrgAdmin}}

	got, err := resolveProductImageCompanyID(req, p, svc)
	require.NoError(t, err)
	require.Equal(t, override, got)
}

func TestResolveProductImageCompanyID_invalidOverride(t *testing.T) {
	t.Parallel()
	svc := testMediaAdminWithCompanyID(t, uuid.MustParse("0194a1b2-c3d4-7890-abcd-ef1234567890"))
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/product-images?company_id=not-a-uuid", nil)
	p := auth.Principal{Roles: []string{auth.RoleOrgAdmin}}

	_, err := resolveProductImageCompanyID(req, p, svc)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid company_id")
}

func TestResolveProductImageCompanyID_missingConfiguration(t *testing.T) {
	t.Parallel()
	svc := testMediaAdminWithCompanyID(t, uuid.Nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/product-images", nil)
	p := auth.Principal{Roles: []string{auth.RoleOrgAdmin}}

	_, err := resolveProductImageCompanyID(req, p, svc)
	require.ErrorIs(t, err, errMediaCompanyNotConfigured)
}
