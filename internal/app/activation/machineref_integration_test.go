package activation

import (
	"context"
	"testing"

	"github.com/avf/avf-vending-api/internal/platform/id"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func insertMachineWithCode(t *testing.T, pool *pgxpool.Pool, siteID, machineID uuid.UUID, code string) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx, `INSERT INTO sites (id, name, code, status) VALUES ($1, 's', '', 'active') ON CONFLICT (id) DO NOTHING`, siteID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, site_id, serial_number, code, status, credential_version)
VALUES ($1, $2, $3, $4, 'online', 0)`, machineID, siteID, "sn-mcref-"+uuid.NewString()[:8], code)
	require.NoError(t, err)
}

func TestResolveMachineRef_integration_knownCode(t *testing.T) {
	t.Parallel()
	pool := activationTestPool(t)
	ctx := context.Background()
	siteID := id.NewUUIDV7()
	machineID := id.NewUUIDV7()
	insertMachineWithCode(t, pool, siteID, machineID, "AVF000001")

	gotID, gotCode, err := ResolveMachineRef(ctx, pool, "AVF000001")
	require.NoError(t, err)
	require.Equal(t, machineID, gotID)
	require.Equal(t, "AVF000001", gotCode)

	gotID, gotCode, err = ResolveMachineRef(ctx, pool, machineID.String())
	require.NoError(t, err)
	require.Equal(t, machineID, gotID)
	require.Equal(t, "AVF000001", gotCode)
}

func TestResolveMachineRef_integration_unknownCode(t *testing.T) {
	t.Parallel()
	pool := activationTestPool(t)
	ctx := context.Background()

	_, _, err := ResolveMachineRef(ctx, pool, "AVF999999")
	require.ErrorIs(t, err, ErrMachineNotFound)
}

func TestResolveMachineBody_integration_conflict(t *testing.T) {
	t.Parallel()
	pool := activationTestPool(t)
	ctx := context.Background()
	siteID := id.NewUUIDV7()
	machineA := id.NewUUIDV7()
	machineB := id.NewUUIDV7()
	insertMachineWithCode(t, pool, siteID, machineA, "AVF000010")
	insertMachineWithCode(t, pool, siteID, machineB, "AVF000011")

	_, _, err := ResolveMachineBody(ctx, pool, machineA.String(), "", "AVF000011", "")
	require.ErrorIs(t, err, ErrMachineIdentifierConflict)
}

func TestResolveMachineBody_integration_byCode(t *testing.T) {
	t.Parallel()
	pool := activationTestPool(t)
	ctx := context.Background()
	siteID := id.NewUUIDV7()
	machineID := id.NewUUIDV7()
	insertMachineWithCode(t, pool, siteID, machineID, "AVF000020")

	gotID, gotCode, err := ResolveMachineBody(ctx, pool, "", "", "AVF000020", "")
	require.NoError(t, err)
	require.Equal(t, machineID, gotID)
	require.Equal(t, "AVF000020", gotCode)
}
