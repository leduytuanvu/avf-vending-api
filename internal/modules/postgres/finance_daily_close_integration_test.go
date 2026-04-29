package postgres_test

import (
	"context"
	"testing"

	appfinance "github.com/avf/avf-vending-api/internal/app/finance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestFinanceDailyClose_IdempotentReplayAndDuplicateScope(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := db.New(pool)
	svc := appfinance.NewService(q, nil)

	closeDate := "2024-06-15"
	tz := "UTC"

	in := appfinance.CreateDailyCloseInput{
		OrganizationID: testfixtures.DevOrganizationID,
		CloseDate:      closeDate,
		Timezone:       tz,
		IdempotencyKey: "idem-replay-1",
		ActorType:      "user",
	}

	first, err := svc.CreateDailyClose(ctx, in)
	require.NoError(t, err)
	require.NotEmpty(t, first.ID)

	second, err := svc.CreateDailyClose(ctx, in)
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)

	in.IdempotencyKey = "idem-replay-2"
	_, err = svc.CreateDailyClose(ctx, in)
	require.ErrorIs(t, err, appfinance.ErrDuplicateDailyClose)
}

func TestFinanceDailyClose_TenantIsolation(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := db.New(pool)
	svc := appfinance.NewService(q, nil)

	in := appfinance.CreateDailyCloseInput{
		OrganizationID: testfixtures.DevOrganizationID,
		CloseDate:      "2024-07-01",
		Timezone:       "UTC",
		IdempotencyKey: "idem-tenant-1",
		ActorType:      "user",
	}
	v, err := svc.CreateDailyClose(ctx, in)
	require.NoError(t, err)
	id, err := uuid.Parse(v.ID)
	require.NoError(t, err)

	_, err = svc.GetDailyClose(ctx, uuid.New(), id)
	require.ErrorIs(t, err, appfinance.ErrDailyCloseNotFound)
}
