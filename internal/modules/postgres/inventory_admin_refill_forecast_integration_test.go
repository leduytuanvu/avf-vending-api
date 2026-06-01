package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestInventoryAdminRefillForecastSlots_unknownCompanyEmpty(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := db.New(pool)
	end := time.Now().UTC()
	start := end.AddDate(0, 0, -14)
	rows, err := q.InventoryAdminRefillForecastSlots(ctx, db.InventoryAdminRefillForecastSlotsParams{
		Column1: start,
		Column2: end,
		Column3: uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Column4: uuid.Nil,
		Column5: uuid.Nil,
		Column6: false,
	})
	require.NoError(t, err)
	require.Empty(t, rows)
}
