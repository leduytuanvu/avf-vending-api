package catalogadmin

import (
	"context"
	"fmt"
	"testing"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestReplaceProductListItems_explicitPriceAndProductIdsPreserve(t *testing.T) {
	pool := catalogWritesExecModePool(t)
	ctx := context.Background()
	svc, err := NewService(db.New(pool), pool, nil)
	require.NoError(t, err)

	p1, err := svc.CreateProduct(ctx, CreateProductInput{
		Sku:    "LIST-P1-" + uuid.NewString()[:8],
		Name:   "List P1",
		Active: false,
	})
	require.NoError(t, err)
	p2, err := svc.CreateProduct(ctx, CreateProductInput{
		Sku:    "LIST-P2-" + uuid.NewString()[:8],
		Name:   "List P2",
		Active: false,
	})
	require.NoError(t, err)

	list, err := svc.CreateProductList(ctx, CreateProductListInput{
		Name: "List prices " + uuid.NewString()[:8],
		Kind: ProductListKindCustom,
	})
	require.NoError(t, err)

	price := int64(12000)
	zero := int64(0)
	detail, err := svc.ReplaceProductListItems(ctx, list.ID, []ProductListItemInput{
		{ProductID: p1.ID, UnitPriceMinor: &price},
		{ProductID: p2.ID, UnitPriceMinor: &zero},
	})
	require.NoError(t, err)
	require.Len(t, detail.Items, 2)
	byID := map[uuid.UUID]int64{}
	for _, item := range detail.Items {
		require.True(t, item.UnitPriceMinor.Valid)
		byID[item.ProductID] = item.UnitPriceMinor.Int64
	}
	require.Equal(t, int64(12000), byID[p1.ID])
	require.Equal(t, int64(0), byID[p2.ID])

	again, err := svc.ReplaceProductListItems(ctx, list.ID, []ProductListItemInput{
		{ProductID: p1.ID},
		{ProductID: p2.ID},
	})
	require.NoError(t, err)
	byID = map[uuid.UUID]int64{}
	for _, item := range again.Items {
		require.True(t, item.UnitPriceMinor.Valid)
		byID[item.ProductID] = item.UnitPriceMinor.Int64
	}
	require.Equal(t, int64(12000), byID[p1.ID], "productIds-only replace must keep list prices")
	require.Equal(t, int64(0), byID[p2.ID])

	imported, err := svc.ImportProductListJSON(ctx, uuid.Nil, list.ID, []byte(fmt.Sprintf(
		`{"productIds":[%q]}`, p1.ID.String(),
	)))
	require.NoError(t, err)
	require.Equal(t, 2, len(imported.Detail.Items))
	for _, item := range imported.Detail.Items {
		if item.ProductID == p1.ID {
			require.True(t, item.UnitPriceMinor.Valid)
			require.Equal(t, int64(12000), item.UnitPriceMinor.Int64)
		}
	}
}
