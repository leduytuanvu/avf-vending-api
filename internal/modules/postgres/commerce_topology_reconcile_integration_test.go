package postgres_test

import (
	"context"
	"testing"

	appfleet "github.com/avf/avf-vending-api/internal/app/fleet"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestReconcileCommerceTopology_IdempotentWithLegacyCurrentConfigs(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	machineID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-00000000c132")
	layoutID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-00000000c133")
	legacyCabinetID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-00000000c134")
	legacyLayoutID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-00000000c135")

	_, err := pool.Exec(ctx, `
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
VALUES ($1, 'ITC132', 'commerce-reconcile-it', 'online', 'tcn', $2)
`, machineID, testfixtures.DevSiteID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
INSERT INTO machine_layouts (id, machine_id, name, status, grid_rows, grid_cols, layout_revision, fingerprint)
VALUES ($1, $2, 'reconcile-it-layout', 'READY', 6, 10, 1, 'it-fp')
`, layoutID, machineID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `UPDATE machines SET active_layout_id = $2 WHERE id = $1`, machineID, layoutID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
INSERT INTO machine_layout_slots (layout_id, slot_code, slot_ordinal, product_id, max_quantity, price_minor, current_inventory, enabled, operational_state)
VALUES
  ($1, 'A1', 1, $2, 10, 5000, 5, true, 'assigned'),
  ($1, 'A2', 2, $3, 10, 6000, 5, true, 'assigned')
`, layoutID, testfixtures.DevProductCola, testfixtures.DevProductWater)
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
DELETE FROM machine_slot_configs WHERE machine_id = $1;
DELETE FROM machine_slot_layouts WHERE machine_id = $1;
DELETE FROM machine_cabinets WHERE machine_id = $1;
DELETE FROM machine_layout_slots WHERE layout_id = $2;
DELETE FROM machine_layouts WHERE id = $2;
UPDATE machines SET active_layout_id = NULL WHERE id = $1;
DELETE FROM machines WHERE id = $1;
`, machineID, layoutID)
	})

	readinessBefore, err := appfleet.LoadCommerceReadiness(ctx, pool, machineID)
	require.NoError(t, err)
	require.True(t, readinessBefore.NeedsReconcile)

	_, _, err = appfleet.ReconcileCommerceTopology(ctx, pool, machineID, false)
	require.NoError(t, err)

	readinessAfter, err := appfleet.LoadCommerceReadiness(ctx, pool, machineID)
	require.NoError(t, err)
	require.False(t, readinessAfter.NeedsReconcile)
	require.Greater(t, readinessAfter.CurrentSlotConfigCount, int64(0))

	_, _, err = appfleet.ReconcileCommerceTopology(ctx, pool, machineID, false)
	require.NoError(t, err)

	readinessSecond, err := appfleet.LoadCommerceReadiness(ctx, pool, machineID)
	require.NoError(t, err)
	require.False(t, readinessSecond.NeedsReconcile)
}
