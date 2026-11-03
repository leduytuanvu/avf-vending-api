package catalogadmin

import (
	"context"
	"testing"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPutMachineProductLists_supplementalOnlyLeavesPrimary(t *testing.T) {
	pool := catalogWritesExecModePool(t)
	ctx := context.Background()
	svc, err := NewService(db.New(pool), pool, nil)
	require.NoError(t, err)

	siteID := id.NewUUIDV7()
	machineID := id.NewUUIDV7()
	_, err = pool.Exec(ctx, `
INSERT INTO sites (id, name, code, status)
VALUES ($1, 'List bind site', $2, 'active')
`, siteID, "lb-site-"+siteID.String()[:8])
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, site_id, serial_number, code, name, status, credential_version)
VALUES ($1, $2, $3, $4, 'List bind machine', 'active', 0)
`, machineID, siteID, "lb-sn-"+machineID.String(), "lbm-"+machineID.String()[:8])
	require.NoError(t, err)

	primary, err := svc.CreateProductList(ctx, CreateProductListInput{
		Name: "Primary " + uuid.NewString()[:8],
		Kind: ProductListKindCustom,
	})
	require.NoError(t, err)
	supp1, err := svc.CreateProductList(ctx, CreateProductListInput{
		Name: "Supp 1 " + uuid.NewString()[:8],
		Kind: ProductListKindCustom,
	})
	require.NoError(t, err)
	supp2, err := svc.CreateProductList(ctx, CreateProductListInput{
		Name: "Supp 2 " + uuid.NewString()[:8],
		Kind: ProductListKindCustom,
	})
	require.NoError(t, err)
	draft, err := svc.CreateProductList(ctx, CreateProductListInput{
		Name:   "Draft " + uuid.NewString()[:8],
		Kind:   ProductListKindCustom,
		Status: "draft",
	})
	require.NoError(t, err)

	full, err := svc.PutMachineProductLists(ctx, PutMachineProductListsInput{
		MachineID:           machineID,
		PrimaryListID:       primary.ID,
		SupplementalListIDs: []uuid.UUID{supp1.ID},
	})
	require.NoError(t, err)
	require.NotNil(t, full.PrimaryListID)
	require.Equal(t, primary.ID, *full.PrimaryListID)
	require.Equal(t, []uuid.UUID{supp1.ID}, full.SupplementalListIDs)

	updated, err := svc.PutMachineProductLists(ctx, PutMachineProductListsInput{
		MachineID:           machineID,
		SupplementalListIDs: []uuid.UUID{supp2.ID, primary.ID},
	})
	require.NoError(t, err)
	require.NotNil(t, updated.PrimaryListID)
	require.Equal(t, primary.ID, *updated.PrimaryListID, "supplemental-only must not replace primary")
	require.Equal(t, []uuid.UUID{supp2.ID}, updated.SupplementalListIDs)

	_, err = svc.PutMachineProductLists(ctx, PutMachineProductListsInput{
		MachineID:           machineID,
		SupplementalListIDs: []uuid.UUID{draft.ID},
	})
	require.ErrorIs(t, err, ErrInvalidArgument)
	require.Contains(t, err.Error(), "published")

	again, err := svc.GetMachineProductLists(ctx, machineID)
	require.NoError(t, err)
	require.NotNil(t, again.PrimaryListID)
	require.Equal(t, primary.ID, *again.PrimaryListID)
	require.Equal(t, []uuid.UUID{supp2.ID}, again.SupplementalListIDs)
}

func TestPutMachineProductLists_emptySupplementalRequiresPrimary(t *testing.T) {
	pool := catalogWritesExecModePool(t)
	ctx := context.Background()
	svc, err := NewService(db.New(pool), pool, nil)
	require.NoError(t, err)

	siteID := id.NewUUIDV7()
	machineID := id.NewUUIDV7()
	_, err = pool.Exec(ctx, `
INSERT INTO sites (id, name, code, status)
VALUES ($1, 'List empty site', $2, 'active')
`, siteID, "le-site-"+siteID.String()[:8])
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, site_id, serial_number, code, name, status, credential_version)
VALUES ($1, $2, $3, $4, 'List empty machine', 'active', 0)
`, machineID, siteID, "le-sn-"+machineID.String(), "lem-"+machineID.String()[:8])
	require.NoError(t, err)

	_, err = svc.PutMachineProductLists(ctx, PutMachineProductListsInput{
		MachineID: machineID,
	})
	require.ErrorIs(t, err, ErrInvalidArgument)
	require.Contains(t, err.Error(), "primaryListId required")
}
