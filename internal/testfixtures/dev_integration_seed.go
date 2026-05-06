package testfixtures

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// DevAssortmentID is a stable id for integration seed data (dev org only).
var DevAssortmentID = uuid.MustParse("ffffffff-ffff-ffff-ffff-0000000000a1")

// DevMachineCabinetID / DevMachineSlotLayoutID match integration seeds for the dev machine cabinet model.
var (
	DevMachineCabinetID    = uuid.MustParse("ffffffff-ffff-ffff-ffff-0000000000c1")
	DevMachineSlotLayoutID = uuid.MustParse("ffffffff-ffff-ffff-ffff-0000000000c2")
)

// DevCommerceSeedAdvisoryLockKey serializes dev-machine commerce seeding across parallel test
// packages (pg_advisory_xact_lock / pg_advisory_lock on this integer).
const DevCommerceSeedAdvisoryLockKey int64 = 586771100

// AcquireDevCommerceSeedSessionLock acquires a session-level advisory lock on
// DevCommerceSeedAdvisoryLockKey for the lifetime of the test. Any other connection trying to
// run EnsureDevCommerceIntegrationData (which uses pg_advisory_xact_lock on the same key) blocks
// until this session releases the lock — use for tests that mutate dev machine inventory and
// cannot interleave with a concurrent seed refresh.
func AcquireDevCommerceSeedSessionLock(t *testing.T, pool *pgxpool.Pool) *pgxpool.Conn {
	t.Helper()
	ctx := context.Background()
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, DevCommerceSeedAdvisoryLockKey)
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx2 := context.Background()
		_, _ = conn.Exec(ctx2, `SELECT pg_advisory_unlock($1)`, DevCommerceSeedAdvisoryLockKey)
		conn.Release()
	})
	return conn
}

// EnsureDevCommerceIntegrationDataUsingLockedConn runs the dev commerce seed on the given pool
// connection after the caller has acquired DevCommerceSeedAdvisoryLockKey via
// AcquireDevCommerceSeedSessionLock (same connection as the lock). Skips the transaction-level
// advisory lock because the session lock already excludes concurrent seed runs.
func EnsureDevCommerceIntegrationDataUsingLockedConn(t *testing.T, conn *pgxpool.Conn) {
	t.Helper()
	ctx := context.Background()
	tx, err := conn.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	devCommerceSeedWork(t, ctx, tx)

	require.NoError(t, tx.Commit(ctx))
}

// EnsureDevCommerceIntegrationData installs published assortment + primary machine binding and
// current cabinet/slot config rows for the seeded dev machine so gRPC commerce/pricing and
// inventory slot resolution match CommerceIsProductInMachinePublishedAssortment and
// InventoryAdminListCurrentMachineSlotConfigsByMachine.
// Uses pg_advisory_xact_lock so parallel test packages sharing the same Postgres do not race.
func EnsureDevCommerceIntegrationData(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, DevCommerceSeedAdvisoryLockKey)
	require.NoError(t, err)

	devCommerceSeedWork(t, ctx, tx)

	require.NoError(t, tx.Commit(ctx))
}

func devCommerceSeedWork(t *testing.T, ctx context.Context, tx pgx.Tx) {
	t.Helper()

	var err error
	_, err = tx.Exec(ctx, `
INSERT INTO assortments (id, organization_id, name, status)
VALUES ($1, $2, 'dev-integration-assortment', 'published')
ON CONFLICT (id) DO UPDATE SET
	status = 'published',
	updated_at = now()
`, DevAssortmentID, DevOrganizationID)
	require.NoError(t, err)

	for _, row := range []struct {
		productID uuid.UUID
		sortOrder int
	}{
		{DevProductCola, 0},
		{DevProductWater, 1},
	} {
		_, err = tx.Exec(ctx, `
INSERT INTO assortment_items (organization_id, assortment_id, product_id, sort_order)
VALUES ($1, $2, $3, $4)
ON CONFLICT (assortment_id, product_id) DO UPDATE SET
	sort_order = EXCLUDED.sort_order
`, DevOrganizationID, DevAssortmentID, row.productID, row.sortOrder)
		require.NoError(t, err)
	}

	_, err = tx.Exec(ctx, `
INSERT INTO machine_assortment_bindings (organization_id, machine_id, assortment_id, is_primary, valid_from, valid_to)
SELECT $1, $2, $3, true, now(), NULL
WHERE NOT EXISTS (
	SELECT 1 FROM machine_assortment_bindings b
	WHERE b.machine_id = $2 AND b.is_primary AND b.valid_to IS NULL
)
`, DevOrganizationID, DevMachineID, DevAssortmentID)
	require.NoError(t, err)

	var cabID uuid.UUID
	err = tx.QueryRow(ctx, `
INSERT INTO machine_cabinets (id, machine_id, cabinet_code, title, sort_order)
VALUES ($1, $2, 'A', 'Alpha', 1)
ON CONFLICT (machine_id, cabinet_code) DO UPDATE SET
	title = EXCLUDED.title,
	sort_order = EXCLUDED.sort_order
RETURNING id
`, DevMachineCabinetID, DevMachineID).Scan(&cabID)
	require.NoError(t, err)

	var layoutID uuid.UUID
	err = tx.QueryRow(ctx, `
INSERT INTO machine_slot_layouts (id, organization_id, machine_id, machine_cabinet_id, layout_key, revision, status, layout_spec)
VALUES ($1, $2, $3, $4, 'default', 1, 'published', '{}'::jsonb)
ON CONFLICT (machine_id, machine_cabinet_id, layout_key, revision) DO UPDATE SET
	status = EXCLUDED.status,
	organization_id = EXCLUDED.organization_id,
	layout_spec = EXCLUDED.layout_spec
RETURNING id
`, DevMachineSlotLayoutID, DevOrganizationID, DevMachineID, cabID).Scan(&layoutID)
	require.NoError(t, err)

	_, err = tx.Exec(ctx, `
DELETE FROM machine_slot_configs
WHERE machine_id = $1 AND is_current = true AND slot_code IN ('0', '1')
`, DevMachineID)
	require.NoError(t, err)

	_, err = tx.Exec(ctx, `
INSERT INTO machine_slot_configs (
	organization_id, machine_id, machine_cabinet_id, machine_slot_layout_id,
	slot_code, slot_index, product_id, max_quantity, price_minor, effective_from, is_current, metadata
) VALUES
	($1, $2, $3, $4, '0', 0, $5, 10, 150, now(), true, '{}'),
	($1, $2, $3, $4, '1', 1, $6, 10, 120, now(), true, '{}')
`, DevOrganizationID, DevMachineID, cabID, layoutID, DevProductCola, DevProductWater)
	require.NoError(t, err)

	_, err = tx.Exec(ctx, `
UPDATE machine_slot_state
SET current_quantity = 5, price_minor = 150, planogram_revision_applied = 1
WHERE machine_id = $1 AND planogram_id = $2 AND slot_index = 0
`, DevMachineID, DevPlanogramID)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `
UPDATE machine_slot_state
SET current_quantity = 8, price_minor = 120, planogram_revision_applied = 1
WHERE machine_id = $1 AND planogram_id = $2 AND slot_index = 1
`, DevMachineID, DevPlanogramID)
	require.NoError(t, err)
}
