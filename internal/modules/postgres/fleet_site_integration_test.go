package postgres_test

import (
	"context"
	"encoding/json"
	"testing"

	appfleet "github.com/avf/avf-vending-api/internal/app/fleet"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestFleetAdminSites_CreateWithAddress_QueryExecModeExec(t *testing.T) {
	pool := execModePool(t)
	ctx := context.Background()
	svc := appfleet.NewService(postgres.NewFleetRepository(pool))

	address := []byte(`{"line1":"68B Đường 21","houseNumber":"68B","street":"Đường 21","ward":"Hiệp Bình","district":"Quận Thủ Đức","districtCode":"762","city":"Thành phố Hồ Chí Minh","provinceCode":"79","region":"Miền Nam","country":"VN"}`)
	code := "addr-site-" + uuid.NewString()[:8]
	created, err := svc.CreateSite(ctx, appfleet.CreateSiteInput{
		Name:     "Miền Nam",
		Timezone: "Asia/Ho_Chi_Minh",
		Code:     code,
		Address:  address,
	})
	require.NoError(t, err)
	require.True(t, json.Valid(created.Address))
	require.JSONEq(t, string(address), string(created.Address))

	got, err := svc.GetSite(ctx, uuid.Nil, created.ID)
	require.NoError(t, err)
	require.JSONEq(t, string(address), string(got.Address))

	emptyCode := "addr-empty-" + uuid.NewString()[:8]
	emptyCreated, err := svc.CreateSite(ctx, appfleet.CreateSiteInput{
		Name:     "No Address",
		Timezone: "UTC",
		Code:     emptyCode,
	})
	require.NoError(t, err)
	require.JSONEq(t, "{}", string(emptyCreated.Address))
}

// Regression: admin sites CRUD must work without company / scope identifiers (single-tenant).
func TestFleetAdminSites_CreateGetListPatchDeactivate_withoutCompanyScope(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	svc := appfleet.NewService(postgres.NewFleetRepository(pool))

	code := "tst-site-" + uuid.NewString()[:8]
	created, err := svc.CreateSite(ctx, appfleet.CreateSiteInput{
		Name:     "Integration Site",
		Timezone: "UTC",
		Code:     code,
	})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, created.ID)
	require.Equal(t, code, created.Code)
	require.Equal(t, "active", created.Status)

	got, err := svc.GetSite(ctx, uuid.Nil, created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
	require.Equal(t, "Integration Site", got.Name)

	items, total, err := svc.ListSites(ctx, appfleet.ListSitesInput{Limit: 500, Offset: 0})
	require.NoError(t, err)
	require.Positive(t, total)
	found := false
	for _, it := range items {
		if it.ID == created.ID {
			found = true
			break
		}
	}
	require.True(t, found, "listed sites must include the created row")

	newName := "Integration Site Renamed"
	updated, err := svc.UpdateSite(ctx, appfleet.UpdateSiteInput{SiteID: created.ID, Name: &newName})
	require.NoError(t, err)
	require.Equal(t, newName, updated.Name)

	archived, err := svc.DeactivateSite(ctx, uuid.Nil, created.ID)
	require.NoError(t, err)
	require.Equal(t, "archived", archived.Status)
}
