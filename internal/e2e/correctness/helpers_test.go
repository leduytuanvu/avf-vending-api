package correctness

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func testDSN(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping e2e correctness tests in -short mode")
	}
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	return dsn
}

func migrateUp(t *testing.T, dsn string) {
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

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := testDSN(t)
	migrateUp(t, dsn)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	testfixtures.EnsureDevCommerceIntegrationData(t, pool)
	return pool
}

func insertOrganizationAndSite(t *testing.T, ctx context.Context, pool *pgxpool.Pool, orgID, siteID uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(ctx, `INSERT INTO organizations (id, name, slug, status) VALUES ($1, $2, $3, 'active')`,
		orgID, "p06-e2e-"+orgID.String()[:8], "p06-"+orgID.String())
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO sites (id, organization_id, name, code, status) VALUES ($1, $2, 'site', '', 'active')`, siteID, orgID)
	require.NoError(t, err)
}

func insertMachine(t *testing.T, ctx context.Context, pool *pgxpool.Pool, orgID, siteID, machineID uuid.UUID, machineStatus string, credentialVersion int64) {
	t.Helper()
	_, err := pool.Exec(ctx, `
INSERT INTO machines (id, organization_id, site_id, serial_number, status, credential_version)
VALUES ($1, $2, $3, $4, $5, $6)`, machineID, orgID, siteID, "sn-p06-"+machineID.String(), machineStatus, credentialVersion)
	require.NoError(t, err)
}

func insertOrganizationSiteMachine(t *testing.T, ctx context.Context, pool *pgxpool.Pool, orgID, siteID, machineID uuid.UUID, machineStatus string, credentialVersion int64) {
	t.Helper()
	insertOrganizationAndSite(t, ctx, pool, orgID, siteID)
	insertMachine(t, ctx, pool, orgID, siteID, machineID, machineStatus, credentialVersion)
}
