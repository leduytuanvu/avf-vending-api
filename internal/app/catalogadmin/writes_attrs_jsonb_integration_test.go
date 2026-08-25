package catalogadmin

import (
	"context"
	"errors"
	"testing"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/pgjson"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func catalogWritesExecModePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := testCatalogWritesDSN(t)
	catalogWritesMigrate(t, dsn)
	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	ctx := context.Background()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func pgState(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

func TestCatalogWriteInsertProduct_uncastByteAttrs_QueryExecModeExec_22P02(t *testing.T) {
	pool := catalogWritesExecModePool(t)
	ctx := context.Background()
	sku := "ATTRS-UNCAST-" + uuid.NewString()[:8]
	_, err := pool.Exec(ctx, `
INSERT INTO products (sku, name, description, attrs, active)
VALUES ($1, $2, '', $3, false)`,
		sku, "Uncast Attrs", []byte(`{}`),
	)
	require.Error(t, err)
	require.Equal(t, "22P02", pgState(err), "got %v", err)
}

func TestCatalogWriteInsertProduct_textJSONAttrs_QueryExecModeExec(t *testing.T) {
	pool := catalogWritesExecModePool(t)
	ctx := context.Background()
	q := db.New(pool)
	row, err := q.CatalogWriteInsertProduct(ctx, db.CatalogWriteInsertProductParams{
		Sku:           "ATTRS-OK-" + uuid.NewString()[:8],
		Name:          "Cast Attrs",
		Description:   "",
		Attrs:         pgjson.RequiredString(nil),
		Active:        false,
		AllergenCodes: []string{},
	})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, row.ID)
}

func TestCreateProduct_defaultAttrs_QueryExecModeExec(t *testing.T) {
	pool := catalogWritesExecModePool(t)
	svc, err := NewService(db.New(pool), pool, nil)
	require.NoError(t, err)
	row, err := svc.CreateProduct(context.Background(), CreateProductInput{
		Sku:    "ATTRS-SVC-" + uuid.NewString()[:8],
		Name:   "Service Attrs",
		Active: false,
	})
	require.NoError(t, err)
	require.JSONEq(t, `{}`, string(row.Attrs))
}

func TestCreateProduct_activeWithoutPrimaryMedia_invalidArgument(t *testing.T) {
	pool := catalogWritesExecModePool(t)
	svc, err := NewService(db.New(pool), pool, nil)
	require.NoError(t, err)
	_, err = svc.CreateProduct(context.Background(), CreateProductInput{
		Sku:    "ATTRS-ACTIVE-" + uuid.NewString()[:8],
		Name:   "Active No Media",
		Active: true,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidArgument)
	require.Contains(t, err.Error(), "primaryMediaId")
}
