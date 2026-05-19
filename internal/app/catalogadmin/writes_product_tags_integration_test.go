package catalogadmin

import (
	"context"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func testCatalogWritesDSN(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration tests in -short mode")
	}
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	return dsn
}

func catalogWritesMigrate(t *testing.T, dsn string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	goBin := os.Getenv("GO_BIN")
	if goBin == "" {
		goBin = "go"
	}
	repoRoot := testfixtures.RepoRoot(t)
	absRoot, err := filepath.Abs(repoRoot)
	require.NoError(t, err)
	migrationsDir := filepath.Join(absRoot, "migrations")
	cmd := exec.CommandContext(ctx, goBin, "run", "github.com/pressly/goose/v3/cmd/goose@v3.27.0",
		"-dir", migrationsDir,
		"postgres", dsn, "up",
	)
	cmd.Dir = absRoot
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", string(out))
}

func catalogWritesPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := testCatalogWritesDSN(t)
	catalogWritesMigrate(t, dsn)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func TestProductTags_CreateUpdateReplaceClear_OmitUnchanged(t *testing.T) {
	t.Parallel()
	pool := catalogWritesPool(t)
	ctx := context.Background()
	svc, err := NewService(db.New(pool), pool, nil)
	require.NoError(t, err)

	t1, err := svc.CreateTag(ctx, CreateTagInput{Slug: "pt-int-" + uuid.NewString()[:8], Name: "PT Int", Active: true})
	require.NoError(t, err)
	t2, err := svc.CreateTag(ctx, CreateTagInput{Slug: "pt-int2-" + uuid.NewString()[:8], Name: "PT Int 2", Active: false})
	require.NoError(t, err)

	sku := "SKU-TAG-" + uuid.NewString()[:8]
	p, err := svc.CreateProduct(ctx, CreateProductInput{
		Sku:           sku,
		Name:          "Tagged",
		Description:   "d",
		Active:        false,
		AgeRestricted: false,
		TagIDs:        []uuid.UUID{t1.ID, t1.ID},
	})
	require.NoError(t, err)

	by, err := svc.ProductTagsByProductIDs(ctx, []uuid.UUID{p.ID})
	require.NoError(t, err)
	require.Len(t, by[p.ID], 1)
	require.Equal(t, t1.ID, by[p.ID][0].ID)

	bad := id.NewUUIDV7()
	_, err = svc.UpdateProduct(ctx, UpdateProductInput{
		ProductID:     p.ID,
		Sku:           sku,
		Name:          "Tagged",
		Description:   "d",
		Active:        false,
		AgeRestricted: false,
		AllergenCodes: []string{},
		TagIDs:        &[]uuid.UUID{bad},
	})
	require.ErrorIs(t, err, ErrInvalidArgument)

	ptr := func(xs []uuid.UUID) *[]uuid.UUID { return &xs }
	p2, err := svc.UpdateProduct(ctx, UpdateProductInput{
		ProductID:     p.ID,
		Sku:           sku,
		Name:          "Tagged",
		Description:   "d",
		Active:        false,
		AgeRestricted: false,
		AllergenCodes: []string{},
		TagIDs:        ptr([]uuid.UUID{t2.ID}),
	})
	require.NoError(t, err)
	require.Equal(t, p.ID, p2.ID)
	by, err = svc.ProductTagsByProductIDs(ctx, []uuid.UUID{p.ID})
	require.NoError(t, err)
	require.Len(t, by[p.ID], 1)
	require.Equal(t, t2.ID, by[p.ID][0].ID)

	_, err = svc.UpdateProduct(ctx, UpdateProductInput{
		ProductID:     p.ID,
		Sku:           sku,
		Name:          "Tagged",
		Description:   "d",
		Active:        false,
		AgeRestricted: false,
		AllergenCodes: []string{},
		TagIDs:        ptr([]uuid.UUID{}),
	})
	require.NoError(t, err)
	by, err = svc.ProductTagsByProductIDs(ctx, []uuid.UUID{p.ID})
	require.NoError(t, err)
	require.Len(t, by[p.ID], 0)

	_, err = svc.UpdateProduct(ctx, UpdateProductInput{
		ProductID:     p.ID,
		Sku:           sku,
		Name:          "Tagged 2",
		Description:   "d",
		Active:        false,
		AgeRestricted: false,
		AllergenCodes: []string{},
		TagIDs:        ptr([]uuid.UUID{t1.ID}),
	})
	require.NoError(t, err)
	by, err = svc.ProductTagsByProductIDs(ctx, []uuid.UUID{p.ID})
	require.NoError(t, err)
	require.Len(t, by[p.ID], 1)

	_, err = svc.UpdateProduct(ctx, UpdateProductInput{
		ProductID:     p.ID,
		Sku:           sku,
		Name:          "Tagged 3",
		Description:   "d",
		Active:        false,
		AgeRestricted: false,
		AllergenCodes: []string{},
		TagIDs:        nil,
	})
	require.NoError(t, err)
	by, err = svc.ProductTagsByProductIDs(ctx, []uuid.UUID{p.ID})
	require.NoError(t, err)
	require.Len(t, by[p.ID], 1, "omit tagIds must not clear tags")
}
