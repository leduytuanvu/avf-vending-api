package productmastercatalog

import (
	"testing"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestApplyListSnapshotPrices_overridesWhenValidIncludingZero(t *testing.T) {
	t.Parallel()
	idOverride := uuid.New()
	idInherit := uuid.New()
	records := []Record{
		{ProductID: idOverride, BasePriceMinor: 15_000},
		{ProductID: idInherit, BasePriceMinor: 9_000},
	}
	applyListSnapshotPrices(records, []db.ProductListSnapshotRowsRow{
		{ID: idOverride, UnitPriceMinor: pgtype.Int8{Int64: 0, Valid: true}},
		{ID: idInherit, UnitPriceMinor: pgtype.Int8{}},
	})
	require.Equal(t, int64(0), records[0].BasePriceMinor)
	require.Equal(t, int64(9_000), records[1].BasePriceMinor)
}
