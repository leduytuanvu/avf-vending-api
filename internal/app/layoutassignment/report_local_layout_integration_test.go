package layoutassignment_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/layoutassignment"
	"github.com/avf/avf-vending-api/internal/app/physicaltopology"
	"github.com/avf/avf-vending-api/internal/platform/pgxutil"
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

func buildSixByTenReportSlotsJSON(t *testing.T, assignedCode string, productID uuid.UUID) []byte {
	t.Helper()
	type slotRow struct {
		SlotCode     string `json:"slotCode"`
		SlotOrdinal  int32  `json:"slotOrdinal"`
		PhysicalLane int32  `json:"physicalLane"`
		ProductID    string `json:"productId,omitempty"`
		MaxQuantity  int32  `json:"maxQuantity"`
	}
	rows := make([]slotRow, 0, 60)
	for _, code := range physicaltopology.AllSlotCodes(6, 10) {
		idx := physicaltopology.SlotIndexFromCode(code, 10)
		row := slotRow{
			SlotCode:     code,
			SlotOrdinal:  idx,
			PhysicalLane: idx,
			MaxQuantity:  6,
		}
		if code == assignedCode {
			row.ProductID = productID.String()
		}
		rows = append(rows, row)
	}
	raw, err := json.Marshal(rows)
	require.NoError(t, err)
	return raw
}

func TestReportLocalLayout_materializesNamedLayout_withLegacyCurrentSlotConfigs(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	machineID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-00000000d195")
	layoutID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-00000000d196")
	legacyCabinetID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-00000000d197")
	legacyLayoutID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-00000000d198")

	_, err := pool.Exec(ctx, `
DELETE FROM machine_local_layout_mirror WHERE machine_id = $1;
DELETE FROM machine_slot_configs WHERE machine_id = $1;
DELETE FROM machine_slot_layouts WHERE machine_id = $1;
DELETE FROM machine_cabinets WHERE machine_id = $1;
DELETE FROM machine_layout_slots WHERE layout_id = $2;
DELETE FROM machine_layouts WHERE id = $2;
UPDATE machines SET active_layout_id = NULL WHERE id = $1;
DELETE FROM machines WHERE id = $1;
`, machineID, layoutID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, code, name, status, machine_type, site_id)
VALUES ($1, 'ITD195', 'report-local-layout-it', 'online', 'tcn', $2)
`, machineID, testfixtures.DevSiteID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
INSERT INTO machine_layouts (id, machine_id, name, status, grid_rows, grid_cols, layout_revision, fingerprint)
VALUES ($1, $2, 'Layout 1 IT', 'READY', 6, 10, 1, 'it-fp')
`, layoutID, machineID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `UPDATE machines SET active_layout_id = $2 WHERE id = $1`, machineID, layoutID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
INSERT INTO machine_cabinets (id, machine_id, cabinet_code, title, sort_order, cabinet_index, status)
VALUES ($1, $2, 'LEGACY', 'Legacy cabinet', 0, 0, 'active')
`, legacyCabinetID, machineID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
INSERT INTO machine_slot_layouts (id, machine_id, machine_cabinet_id, layout_key, revision, layout_spec, status)
VALUES ($1, $2, $3, 'legacy', 1, '{"rows":6,"cols":10}', 'published')
`, legacyLayoutID, machineID, legacyCabinetID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
INSERT INTO machine_slot_configs (
  machine_id, machine_cabinet_id, machine_slot_layout_id, slot_code, slot_index,
  product_id, max_quantity, price_minor, is_current
)
VALUES
  ($1, $2, $3, 'A1', 1, $4, 10, 5000, true),
  ($1, $2, $3, 'A2', 2, $5, 10, 6000, true)
`, machineID, legacyCabinetID, legacyLayoutID, testfixtures.DevProductCola, testfixtures.DevProductWater)
	require.NoError(t, err)

	t.Cleanup(func() {
		cctx := context.Background()
		_, _ = pool.Exec(cctx, `
DELETE FROM machine_local_layout_mirror WHERE machine_id = $1;
DELETE FROM machine_slot_configs WHERE machine_id = $1;
DELETE FROM machine_slot_layouts WHERE machine_id = $1;
DELETE FROM machine_cabinets WHERE machine_id = $1;
DELETE FROM machine_layout_slots WHERE layout_id = $2;
DELETE FROM machine_layouts WHERE id = $2;
UPDATE machines SET active_layout_id = NULL WHERE id = $1;
DELETE FROM machines WHERE id = $1;
`, machineID, layoutID)
	})

	slotsJSON := buildSixByTenReportSlotsJSON(t, "A1", testfixtures.DevProductCola)
	svc := &layoutassignment.Service{Pool: pool}
	out, err := svc.ReportLocalLayout(ctx, layoutassignment.MachineAuthContext{MachineID: machineID}, layoutassignment.ReportLocalLayoutInput{
		MachineID:        machineID,
		LocalLayoutID:    layoutID,
		Revision:         1,
		Rows:             6,
		Columns:          10,
		SlotsJSON:        slotsJSON,
		Fingerprint:      "it-report-local-layout-1",
		DeviceInstanceID: "it-device-1",
		LocalGeneration:  1,
	})
	require.NoError(t, err)
	require.True(t, out.Accepted)

	q := pgxutil.NewQueries(pool)
	slotRows, err := q.ListMachineLayoutSlots(ctx, layoutID)
	require.NoError(t, err)
	require.Len(t, slotRows, 60)
	assigned := 0
	for _, row := range slotRows {
		if row.ProductID.Valid && row.ProductID.Bytes != uuid.Nil {
			assigned++
		}
	}
	require.Greater(t, assigned, 0)

	detail, err := svc.GetMachineLayoutDetail(ctx, machineID, layoutID)
	require.NoError(t, err)
	require.NotEmpty(t, detail.Slots)
	hasA1Product := false
	for _, sl := range detail.Slots {
		if sl.SlotCode == "A1" && sl.ProductID == testfixtures.DevProductCola.String() {
			hasA1Product = true
		}
	}
	require.True(t, hasA1Product)
}
