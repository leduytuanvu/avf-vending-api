package postgres_test

import (
	"context"
	"testing"
	"time"

	appcatalogadmin "github.com/avf/avf-vending-api/internal/app/catalogadmin"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPriceBook_preview_orgWidePrice(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := db.New(pool)
	svc, err := appcatalogadmin.NewService(q, pool, nil)
	require.NoError(t, err)

	at := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	out, err := svc.PreviewPricing(ctx, appcatalogadmin.PricingPreviewParams{
		OrganizationID: testfixtures.DevOrganizationID,
		ProductIDs:     []uuid.UUID{testfixtures.DevProductWater},
		At:             at,
	})
	require.NoError(t, err)
	require.NotEmpty(t, out.Lines)
	require.Equal(t, testfixtures.DevProductWater, out.Lines[0].ProductID)
	require.Positive(t, out.Lines[0].EffectiveMinor)
	require.NotEqual(t, uuid.Nil, out.Lines[0].PriceBookID)
}
