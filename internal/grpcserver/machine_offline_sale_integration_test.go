package grpcserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/avf/avf-vending-api/internal/testfixtures"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestP06_OfflineSync_offlineSaleReplayCreatesOrderAndCashPayment(t *testing.T) {
	pool := machineGRPCTestPool(t)
	ctx := context.Background()

	_, err := pool.Exec(ctx, `DELETE FROM machine_offline_events WHERE machine_id = $1`, testfixtures.DevMachineID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM machine_sync_cursors WHERE machine_id = $1 AND stream_name = 'offline'`,
		testfixtures.DevMachineID)
	require.NoError(t, err)

	deps := offlineSyncIntegrationDeps(t, pool)
	srv := &machineOfflineSyncServer{deps: deps}
	var credVer int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT credential_version FROM machines WHERE id = $1`, testfixtures.DevMachineID).Scan(&credVer))
	claims := plauth.MachineAccessClaims{MachineID: testfixtures.DevMachineID, CredentialVersion: credVer}
	ctxClaims := plauth.WithMachineAccessClaims(ctx, claims)

	base := "p06-offline-sale-" + uuid.NewString()
	snapshot := map[string]any{
		"snapshotId":           base + ":snap",
		"machineId":            testfixtures.DevMachineID.String(),
		"payableTotalMinor":    100,
		"currency":             "USD",
		"localPricingRevision": 1,
		"slotConfigVersion":    1,
		"capturedAtEpochMs":    1_700_000_000_000,
		"lines": []map[string]any{
			{
				"slotCode":       "0",
				"productId":      testfixtures.DevProductCola.String(),
				"unitPriceMinor": 100,
				"quantity":       1,
			},
		},
	}
	payloadMap := map[string]any{
		"orderId":            base + ":order",
		"machineId":          testfixtures.DevMachineID.String(),
		"transactionId":      base + ":tx",
		"cashReceivedMinor":  100,
		"payableTotalMinor":  100,
		"currency":           "USD",
		"snapshotId":         base + ":snap",
		"pricingSnapshot":    snapshot,
		"canonical_operation": "commerce.offline_sale",
	}
	payloadJSON, err := json.Marshal(payloadMap)
	require.NoError(t, err)
	var payloadStruct structpb.Struct
	require.NoError(t, protojson.Unmarshal(payloadJSON, &payloadStruct))

	out, err := srv.PushOfflineEvents(ctxClaims, &machinev1.SyncOfflineEventsRequest{
		Meta: &machinev1.MachineRequestMeta{IdempotencyKey: base + ":sync", RequestId: base + ":sync"},
		Events: []*machinev1.OfflineEvent{{
			Meta: &machinev1.MachineRequestMeta{
				OfflineSequence: 1,
				IdempotencyKey:  base + ":offline-sale",
				RequestId:       base + ":req",
				ClientEventId:   base + ":cli",
				OccurredAt:      timestamppb.Now(),
			},
			EventType: "commerce.offline_sale",
			Payload:   &payloadStruct,
		}},
	})
	require.NoError(t, err)
	require.Len(t, out.GetResults(), 1)
	require.Equal(t, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED, out.GetResults()[0].GetStatus())

	var orderCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM orders WHERE machine_id = $1 AND total_minor = 100 AND created_at > NOW() - INTERVAL '5 minutes'`,
		testfixtures.DevMachineID).Scan(&orderCount))
	require.GreaterOrEqual(t, orderCount, 1)

	var paidCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM payments p JOIN orders o ON o.id = p.order_id
		 WHERE o.machine_id = $1 AND p.provider = 'cash' AND p.state = 'captured'
		   AND o.created_at > NOW() - INTERVAL '5 minutes'`,
		testfixtures.DevMachineID).Scan(&paidCount))
	require.GreaterOrEqual(t, paidCount, 1)

	// Idempotent replay at same sequence returns REPLAYED.
	out2, err := srv.PushOfflineEvents(ctxClaims, &machinev1.SyncOfflineEventsRequest{
		Meta: &machinev1.MachineRequestMeta{IdempotencyKey: base + ":sync2", RequestId: base + ":sync2"},
		Events: []*machinev1.OfflineEvent{{
			Meta: &machinev1.MachineRequestMeta{
				OfflineSequence: 1,
				IdempotencyKey:  base + ":offline-sale",
				RequestId:       base + ":req2",
				ClientEventId:   base + ":cli",
				OccurredAt:      timestamppb.Now(),
			},
			EventType: "commerce.offline_sale",
			Payload:   &payloadStruct,
		}},
	})
	require.NoError(t, err)
	require.Len(t, out2.GetResults(), 1)
	require.Equal(t, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_REPLAYED, out2.GetResults()[0].GetStatus())
}
