package activation

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/avf/avf-vending-api/internal/platform/id"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestCreateAndListCodes_includeMachineCode(t *testing.T) {
	t.Parallel()
	pool := activationTestPool(t)
	ctx := context.Background()
	siteID := id.NewUUIDV7()
	machineID := id.NewUUIDV7()
	insertMachineWithCode(t, pool, siteID, machineID, "AVF000101")

	svc := newActivationTestService(t, pool)
	create, err := svc.CreateCode(ctx, CreateInput{MachineID: machineID, ExpiresInMinutes: 60, MaxUses: 1})
	require.NoError(t, err)
	require.Equal(t, "AVF000101", create.MachineCode)
	require.NotEmpty(t, create.PlaintextCode)

	rows, err := svc.ListCodes(ctx, machineID)
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	require.Equal(t, "AVF000101", rows[0].MachineCode)

	raw, err := json.Marshal(rows)
	require.NoError(t, err)
	body := string(raw)
	require.NotContains(t, body, "codeHash")
	require.NotContains(t, body, create.PlaintextCode)
}

func TestListAllCodes_includeMachineCode_noPlaintext(t *testing.T) {
	t.Parallel()
	pool := activationTestPool(t)
	ctx := context.Background()
	siteID := id.NewUUIDV7()
	machineID := id.NewUUIDV7()
	insertMachineWithCode(t, pool, siteID, machineID, "AVF000102")

	svc := newActivationTestService(t, pool)
	create, err := svc.CreateCode(ctx, CreateInput{MachineID: machineID, ExpiresInMinutes: 60, MaxUses: 1})
	require.NoError(t, err)

	rows, total, err := svc.ListAllCodes(ctx, 50, 0)
	require.NoError(t, err)
	require.Positive(t, total)
	found := false
	for _, row := range rows {
		if row.ID == create.ID {
			found = true
			require.Equal(t, "AVF000102", row.MachineCode)
		}
	}
	require.True(t, found)

	raw, err := json.Marshal(rows)
	require.NoError(t, err)
	body := string(raw)
	require.NotContains(t, body, "codeHash")
	require.NotContains(t, body, create.PlaintextCode)
}

func newActivationTestService(t *testing.T, pool *pgxpool.Pool) *Service {
	t.Helper()
	svc, _ := activationServiceWithRuntime(t, pool)
	return svc
}
