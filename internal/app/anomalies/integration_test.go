package anomalies_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/anomalies"
	"github.com/avf/avf-vending-api/internal/app/inventoryadmin"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func testDSN(t *testing.T) string {
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
	return pool
}

func TestP24_Sync_OfflineMachineCreatesAnomaly(t *testing.T) {
	t.Parallel()
	pool := testPool(t)
	ctx := context.Background()
	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	q := db.New(pool)
	inv, err := inventoryadmin.NewService(q)
	require.NoError(t, err)
	svc, err := anomalies.NewService(pool, inv)
	require.NoError(t, err)

	slug := "anom-offline-" + uuid.NewString()
	_, err = pool.Exec(ctx, `INSERT INTO organizations (id, name, slug, status) VALUES ($1, 'a', $2, 'active')`, orgID, slug)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO sites (id, organization_id, name, code, status) VALUES ($1, $2, 's', '', 'active')`, siteID, orgID)
	require.NoError(t, err)
	past := time.Now().UTC().Add(-4 * time.Hour)
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, organization_id, site_id, serial_number, status, last_seen_at, credential_version)
VALUES ($1, $2, $3, $4, 'active', $5, 1)`, machineID, orgID, siteID, "sn-offline-p24-"+uuid.NewString()[:8], past)
	require.NoError(t, err)

	require.NoError(t, svc.Sync(ctx, orgID))

	var count int
	err = pool.QueryRow(ctx, `
SELECT count(*) FROM inventory_anomalies
WHERE organization_id = $1 AND machine_id = $2 AND anomaly_type = 'machine_offline_too_long' AND status = 'open'`,
		orgID, machineID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestP24_Sync_RepeatedVendFailure_Deduped(t *testing.T) {
	t.Parallel()
	pool := testPool(t)
	ctx := context.Background()
	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	productID := uuid.New()
	q := db.New(pool)
	inv, err := inventoryadmin.NewService(q)
	require.NoError(t, err)
	svc, err := anomalies.NewService(pool, inv)
	require.NoError(t, err)

	slug := "anom-vend-" + uuid.NewString()
	_, err = pool.Exec(ctx, `INSERT INTO organizations (id, name, slug, status) VALUES ($1, 'b', $2, 'active')`, orgID, slug)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO sites (id, organization_id, name, code, status) VALUES ($1, $2, 's', '', 'active')`, siteID, orgID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, organization_id, site_id, serial_number, status, last_seen_at, credential_version)
VALUES ($1, $2, $3, $4, 'active', now(), 1)`, machineID, orgID, siteID, "sn-vend-p24-"+uuid.NewString()[:8])
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO products (id, organization_id, sku, name) VALUES ($1, $2, 'SKU1', 'Cola')`, productID, orgID)
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		orderID := uuid.New()
		_, err = pool.Exec(ctx, `
INSERT INTO orders (id, organization_id, machine_id, status, currency, subtotal_minor, tax_minor, total_minor)
VALUES ($1, $2, $3, 'failed', 'USD', 0, 0, 0)`, orderID, orgID, machineID)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `
INSERT INTO vend_sessions (order_id, machine_id, slot_index, product_id, state, failure_reason, completed_at)
VALUES ($1, $2, 0, $3, 'failed', 'mechanical', now())`, orderID, machineID, productID)
		require.NoError(t, err)
	}

	require.NoError(t, svc.Sync(ctx, orgID))
	require.NoError(t, svc.Sync(ctx, orgID))

	var count int
	err = pool.QueryRow(ctx, `
SELECT count(*) FROM inventory_anomalies
WHERE organization_id = $1 AND machine_id = $2 AND anomaly_type = 'repeated_vend_failure' AND status = 'open'`,
		orgID, machineID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestP24_RestockSuggestions_seededVelocity(t *testing.T) {
	t.Parallel()
	pool := testPool(t)
	ctx := context.Background()
	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	planogramID := uuid.New()
	productID := uuid.New()
	q := db.New(pool)
	inv, err := inventoryadmin.NewService(q)
	require.NoError(t, err)
	svc, err := anomalies.NewService(pool, inv)
	require.NoError(t, err)

	slug := "anom-rest-" + uuid.NewString()
	_, err = pool.Exec(ctx, `INSERT INTO organizations (id, name, slug, status) VALUES ($1, 'c', $2, 'active')`, orgID, slug)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO sites (id, organization_id, name, code, status) VALUES ($1, $2, 's', '', 'active')`, siteID, orgID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, organization_id, site_id, serial_number, status, last_seen_at, credential_version)
VALUES ($1, $2, $3, $4, 'active', now(), 1)`, machineID, orgID, siteID, "sn-rest-p24-"+uuid.NewString()[:8])
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO products (id, organization_id, sku, name) VALUES ($1, $2, 'SKU2', 'Water')`, productID, orgID)
	require.NoError(t, err)

	fix := func(sql string, args ...any) {
		_, e := pool.Exec(ctx, sql, args...)
		require.NoError(t, e)
	}
	fix(`INSERT INTO planograms (id, organization_id, name, revision, status) VALUES ($1, $2, 'P1', 1, 'published')`, planogramID, orgID)
	fix(`INSERT INTO slots (planogram_id, slot_index, product_id, max_quantity) VALUES ($1, 0, $2, 20)`, planogramID, productID)
	fix(`INSERT INTO machine_slot_state (machine_id, planogram_id, slot_index, current_quantity, price_minor, planogram_revision_applied)
VALUES ($1, $2, 0, 5, 100, 1)`, machineID, planogramID)

	orderID := uuid.New()
	fix(`INSERT INTO orders (id, organization_id, machine_id, status, currency, subtotal_minor, tax_minor, total_minor)
VALUES ($1, $2, $3, 'completed', 'USD', 100, 0, 100)`, orderID, orgID, machineID)
	fix(`INSERT INTO vend_sessions (order_id, machine_id, slot_index, product_id, state, completed_at)
VALUES ($1, $2, 0, $3, 'success', now())`, orderID, machineID, productID)

	out, err := svc.RestockSuggestions(ctx, inventoryadmin.RefillForecastParams{
		OrganizationID:     orgID,
		VelocityWindowDays: 14,
		Limit:              50,
		Offset:             0,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(out.Items), 1)
	it := out.Items[0]
	require.Equal(t, machineID.String(), it.MachineID)
	require.Equal(t, productID.String(), it.ProductID)
	require.Greater(t, it.UnitsSoldInWindow, int64(0))
	require.Greater(t, it.SuggestedRefillQuantity, int32(0))
}

func TestP24_Resolve_thenClosed(t *testing.T) {
	t.Parallel()
	pool := testPool(t)
	ctx := context.Background()
	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	q := db.New(pool)
	inv, err := inventoryadmin.NewService(q)
	require.NoError(t, err)
	svc, err := anomalies.NewService(pool, inv)
	require.NoError(t, err)

	slug := "anom-res-" + uuid.NewString()
	_, err = pool.Exec(ctx, `INSERT INTO organizations (id, name, slug, status) VALUES ($1, 'd', $2, 'active')`, orgID, slug)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO sites (id, organization_id, name, code, status) VALUES ($1, $2, 's', '', 'active')`, siteID, orgID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, organization_id, site_id, serial_number, status, last_seen_at, credential_version)
VALUES ($1, $2, $3, $4, 'active', now(), 1)`, machineID, orgID, siteID, "sn-res-p24-"+uuid.NewString()[:8])
	require.NoError(t, err)

	anomalyID := uuid.New()
	_, err = pool.Exec(ctx, `
INSERT INTO inventory_anomalies (id, organization_id, machine_id, anomaly_type, fingerprint, status, payload)
VALUES ($1, $2, $3, 'telemetry_missing', $4, 'open', '{}')`,
		anomalyID, orgID, machineID, "telemetry_missing|"+machineID.String())
	require.NoError(t, err)

	require.NoError(t, svc.Resolve(ctx, orgID, anomalyID, uuid.Nil, "ack"))

	var st string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM inventory_anomalies WHERE id = $1`, anomalyID).Scan(&st))
	require.Equal(t, "resolved", st)
}

func TestP24_Ignore_thenIgnored(t *testing.T) {
	t.Parallel()
	pool := testPool(t)
	ctx := context.Background()
	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	q := db.New(pool)
	inv, err := inventoryadmin.NewService(q)
	require.NoError(t, err)
	svc, err := anomalies.NewService(pool, inv)
	require.NoError(t, err)

	slug := "anom-ign-" + uuid.NewString()
	_, err = pool.Exec(ctx, `INSERT INTO organizations (id, name, slug, status) VALUES ($1, 'e', $2, 'active')`, orgID, slug)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO sites (id, organization_id, name, code, status) VALUES ($1, $2, 's', '', 'active')`, siteID, orgID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, organization_id, site_id, serial_number, status, last_seen_at, credential_version)
VALUES ($1, $2, $3, $4, 'active', now(), 1)`, machineID, orgID, siteID, "sn-ign-p24-"+uuid.NewString()[:8])
	require.NoError(t, err)

	anomalyID := uuid.New()
	_, err = pool.Exec(ctx, `
INSERT INTO inventory_anomalies (id, organization_id, machine_id, anomaly_type, fingerprint, status, payload)
VALUES ($1, $2, $3, 'telemetry_missing', $4, 'open', '{}')`,
		anomalyID, orgID, machineID, "telemetry_missing|"+machineID.String()+"-b")
	require.NoError(t, err)

	require.NoError(t, svc.Ignore(ctx, orgID, anomalyID, uuid.Nil, "noise"))

	var st string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM inventory_anomalies WHERE id = $1`, anomalyID).Scan(&st))
	require.Equal(t, "ignored", st)
}
