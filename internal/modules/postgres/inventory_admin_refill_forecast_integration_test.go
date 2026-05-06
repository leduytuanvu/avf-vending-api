package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestInventoryAdminRefillForecastSlots_unknownOrganizationEmpty(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := db.New(pool)
	end := time.Now().UTC()
	start := end.AddDate(0, 0, -14)
	unknownOrg := uuid.New()
	rows, err := q.InventoryAdminRefillForecastSlots(ctx, db.InventoryAdminRefillForecastSlotsParams{
		OrganizationID: unknownOrg,
		Column2:        start,
		Column3:        end,
		Column4:        uuid.Nil,
		Column5:        uuid.Nil,
		Column6:        uuid.Nil,
		Column7:        false,
	})
	require.NoError(t, err)
	require.Empty(t, rows)
}
