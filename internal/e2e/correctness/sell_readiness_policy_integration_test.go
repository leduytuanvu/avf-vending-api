package correctness

import (
	"context"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"testing"

	appcatalogadmin "github.com/avf/avf-vending-api/internal/app/catalogadmin"
	"github.com/avf/avf-vending-api/internal/app/sellreadiness"
	"github.com/avf/avf-vending-api/internal/app/setupapp"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestP06_SellReadiness_createInactiveProductWithoutMedia(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := db.New(pool)
	svc, err := appcatalogadmin.NewService(q, pool, nil)
	require.NoError(t, err)
	row, err := svc.CreateProduct(ctx, appcatalogadmin.CreateProductInput{
		Sku:    "draft-" + uuid.NewString(),
		Name:   "Draft No Media",
		Active: false,
	})
	require.NoError(t, err)
	require.False(t, row.Active)
}

func TestP06_SellReadiness_createActiveProductWithoutPrimaryRejected(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := db.New(pool)
	svc, err := appcatalogadmin.NewService(q, pool, nil)
	require.NoError(t, err)
	_, err = svc.CreateProduct(ctx, appcatalogadmin.CreateProductInput{
		Sku:    "active-" + uuid.NewString(),
		Name:   "Active Missing Primary",
		Active: true,
	})
	require.Error(t, err)
}

func TestP06_SellReadiness_activateDraftWithoutReadyPrimaryRejected(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := db.New(pool)
	svc, err := appcatalogadmin.NewService(q, pool, nil)
	require.NoError(t, err)
	row, err := svc.CreateProduct(ctx, appcatalogadmin.CreateProductInput{
		Sku:    "flip-" + uuid.NewString(),
		Name:   "Flip Active",
		Active: false,
	})
	require.NoError(t, err)

	in := appcatalogadmin.UpdateProductInput{
		ProductID:     row.ID,
		Sku:           row.Sku,
		Name:          row.Name,
		Description:   row.Description,
		Attrs:         row.Attrs,
		Active:        true,
		AgeRestricted: row.AgeRestricted,
		AllergenCodes: row.AllergenCodes,
	}
	if row.CategoryID.Valid {
		u := uuid.UUID(row.CategoryID.Bytes)
		in.CategoryID = &u
	}
	if row.BrandID.Valid {
		u := uuid.UUID(row.BrandID.Bytes)
		in.BrandID = &u
	}
	_, err = svc.UpdateProduct(ctx, in)
	require.Error(t, err)
	require.ErrorIs(t, err, appcatalogadmin.ErrInvalidArgument)
}

func TestP06_SellReadiness_runtimePrimaryMediaReady_falseWithoutPrimary(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := db.New(pool)
	row, err := q.CatalogWriteInsertProduct(ctx, db.CatalogWriteInsertProductParams{
		Sku:           "sql-" + uuid.NewString(),
		Name:          "SQL Insert",
		Description:   "",
		Attrs:         []byte(`{}`),
		Active:        true,
		AgeRestricted: false,
		AllergenCodes: []string{},
	})
	require.NoError(t, err)
	ready, err := q.RuntimeProductPrimaryMediaReady(ctx, []uuid.UUID{row.ID})
	require.NoError(t, err)
	require.Len(t, ready, 1)
	require.False(t, ready[0].Ready)
}

func TestP06_SellReadiness_sellableSlotProductID_filter(t *testing.T) {
	t.Parallel()
	pid := id.NewUUIDV7()
	save := setupapp.SlotConfigSaveInput{
		Items: []setupapp.SlotConfigSaveItem{
			{ProductID: &pid, MaxQuantity: 2, PriceMinor: 100},
			{ProductID: nil, MaxQuantity: 0, PriceMinor: 0},
		},
	}
	got := sellreadiness.SellableSlotProductIDs(save)
	require.Len(t, got, 1)
	require.Equal(t, pid, got[0])
}
