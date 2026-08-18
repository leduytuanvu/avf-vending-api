package postgres_test

import (
	"context"
	"testing"
	"time"

	appaudit "github.com/avf/avf-vending-api/internal/app/audit"
	appcatalogadmin "github.com/avf/avf-vending-api/internal/app/catalogadmin"
	appfleetadmin "github.com/avf/avf-vending-api/internal/app/fleetadmin"
	"github.com/avf/avf-vending-api/internal/app/listscope"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/pgxutil"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func testPoolWithQueryExecMode(t *testing.T, mode pgx.QueryExecMode) *pgxpool.Pool {
	t.Helper()
	dsn := testDSN(t)
	migrateUp(t, dsn)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pcfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	pcfg.ConnConfig.DefaultQueryExecMode = mode
	pgxutil.ApplyUUIDArrayCodecsToPoolConfig(pcfg)
	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	testfixtures.EnsureDevCommerceIntegrationData(t, pool)
	return pool
}

func TestFleetAdminListMachines_uuidArrayEnrichment(t *testing.T) {
	pool := testPool(t)
	q := pgxutil.NewQueries(pool)
	svc, err := appfleetadmin.NewService(q)
	require.NoError(t, err)
	ctx := context.Background()
	for _, limit := range []int32{1, 20, 100, 200} {
		out, err := svc.ListMachines(ctx, listscope.AdminFleet{Limit: limit, Offset: 0})
		require.NoError(t, err, "limit=%d", limit)
		require.NotNil(t, out)
		require.GreaterOrEqual(t, out.Meta.Total, int64(0))
	}
	detail, err := svc.GetMachine(ctx, uuid.Nil, testfixtures.DevMachineID)
	require.NoError(t, err)
	require.NotNil(t, detail)
}

func TestCatalogAdminListProducts_omittedUUIDFilters(t *testing.T) {
	pool := testPool(t)
	svc, err := appcatalogadmin.NewService(pgxutil.NewQueries(pool), pool, nil)
	require.NoError(t, err)
	ctx := context.Background()
	res, err := svc.ListProducts(ctx, appcatalogadmin.ListProductsParams{Limit: 20, Offset: 0})
	require.NoError(t, err)
	require.NotNil(t, res)

	unknown := uuid.MustParse("99999999-9999-4999-8999-999999999999")
	filtered, err := svc.ListProducts(ctx, appcatalogadmin.ListProductsParams{
		Limit: 20, Offset: 0, BrandID: &unknown,
	})
	require.NoError(t, err)
	require.Equal(t, int64(0), filtered.TotalCount)

	both, err := svc.ListProducts(ctx, appcatalogadmin.ListProductsParams{
		Limit: 20, Offset: 0, BrandID: &unknown, CategoryID: &unknown,
	})
	require.NoError(t, err)
	require.Equal(t, int64(0), both.TotalCount)
}

func TestEnterpriseAuditListEvents_omittedTimestamps(t *testing.T) {
	pool := testPool(t)
	audit := appaudit.NewService(pool)
	ctx := context.Background()
	from := time.Now().UTC().Add(-24 * time.Hour)
	to := time.Now().UTC().Add(24 * time.Hour)
	cases := []appaudit.EventListParams{
		{Limit: 5, Offset: 0},
		{Limit: 5, Offset: 0, From: &from},
		{Limit: 5, Offset: 0, To: &to},
		{Limit: 5, Offset: 0, From: &from, To: &to},
	}
	for i, p := range cases {
		out, err := audit.ListEvents(ctx, p)
		require.NoError(t, err, "case %d", i)
		require.NotNil(t, out)
	}
}

func TestUUIDArray_codecOnly_noWrap(t *testing.T) {
	pool := testPool(t)
	q := db.New(pool)
	ctx := context.Background()
	_, err := q.FleetAdminListActiveTechnicianAssignmentsForMachines(ctx, []uuid.UUID{testfixtures.DevMachineID})
	require.NoError(t, err, "codec-only uuid[] fleet enrichment")
	_, err = q.RuntimeProductPrimaryMediaReady(ctx, []uuid.UUID{testfixtures.DevProductWater})
	require.NoError(t, err, "codec-only uuid[] media readiness")
}

func TestFleetAdminListMachines_uuidArrayEnrichment_queryExecMode(t *testing.T) {
	modes := []pgx.QueryExecMode{pgx.QueryExecModeExec, pgx.QueryExecModeSimpleProtocol}
	ctx := context.Background()
	for _, mode := range modes {
		t.Run(mode.String(), func(t *testing.T) {
			pool := testPoolWithQueryExecMode(t, mode)
			svc, err := appfleetadmin.NewService(pgxutil.NewQueries(pool))
			require.NoError(t, err)
			for _, limit := range []int32{1, 20} {
				out, err := svc.ListMachines(ctx, listscope.AdminFleet{Limit: limit, Offset: 0})
				require.NoError(t, err, "mode=%s limit=%d", mode, limit)
				require.NotNil(t, out)
			}
		})
	}
}

func TestCatalogAdminListProducts_omittedUUIDFilters_queryExecMode(t *testing.T) {
	modes := []pgx.QueryExecMode{pgx.QueryExecModeExec, pgx.QueryExecModeSimpleProtocol}
	ctx := context.Background()
	for _, mode := range modes {
		t.Run(mode.String(), func(t *testing.T) {
			pool := testPoolWithQueryExecMode(t, mode)
			svc, err := appcatalogadmin.NewService(pgxutil.NewQueries(pool), pool, nil)
			require.NoError(t, err)
			res, err := svc.ListProducts(ctx, appcatalogadmin.ListProductsParams{Limit: 20, Offset: 0})
			require.NoError(t, err, "mode=%s", mode)
			require.NotNil(t, res)
		})
	}
}
