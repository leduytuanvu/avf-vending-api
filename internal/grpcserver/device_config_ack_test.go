package grpcserver

import (
	"context"
	"testing"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func ensureDevMachineSnapshot(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
INSERT INTO machine_current_snapshot (machine_id, site_id, reported_state, metrics_state)
VALUES ($1, $2, '{}'::jsonb, '{}'::jsonb)
ON CONFLICT (machine_id) DO NOTHING
`, testfixtures.DevMachineID, testfixtures.DevSiteID)
	require.NoError(t, err)
}

func TestAckConfigVersion_persistsEffectiveDeviceConfigAndFieldAck(t *testing.T) {
	pool := machineGRPCTestPool(t)
	ensureDevMachineSnapshot(t, pool)
	ctx := context.Background()

	srv, issuer := machineCommerceTestServer(t, pool, testMachineGRPCConfig())
	conn := dialMachineCommerceServer(t, srv)
	md := machineAccessMD(t, pool, issuer, testfixtures.DevMachineID, testfixtures.DevSiteID)
	cli := machinev1.NewMachineBootstrapServiceClient(conn)

	effective, err := structpb.NewStruct(map[string]any{
		"schemaVersion": float64(1),
		"tcn": map[string]any{
			"laneMode": float64(10),
		},
	})
	require.NoError(t, err)

	_, err = cli.AckConfigVersion(md, &machinev1.AckConfigVersionRequest{
		AcknowledgedConfigVersion: 3,
		EffectiveDeviceConfig:     effective,
		FieldAck: map[string]string{
			"tcn.laneMode": "applied",
		},
	})
	require.NoError(t, err)

	row, err := db.New(pool).FleetAdminGetMachineDetail(ctx, testfixtures.DevMachineID)
	require.NoError(t, err)
	require.Contains(t, string(row.EffectiveDeviceConfig), `"laneMode"`)
	require.Contains(t, string(row.DeviceConfigFieldAck), `"tcn.laneMode"`)
}
