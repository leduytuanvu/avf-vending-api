package grpcserver

import (
	"context"
	"testing"

	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestPushOfflineEvents_rejectedCashSlotDoesNotAdvanceCommerceCursor(t *testing.T) {
	pool := machineGRPCTestPool(t)
	ctx := context.Background()

	var qtyRestore int32
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT current_quantity FROM machine_slot_state WHERE machine_id = $1 AND slot_index = 0`,
		testfixtures.DevMachineID).Scan(&qtyRestore))
	t.Cleanup(func() {
		ctx2 := context.Background()
		_, _ = pool.Exec(ctx2, `UPDATE machine_slot_state SET current_quantity = $1 WHERE machine_id = $2 AND slot_index = 0`,
			qtyRestore, testfixtures.DevMachineID)
		_, _ = pool.Exec(ctx2, `DELETE FROM machine_offline_events WHERE machine_id = $1`, testfixtures.DevMachineID)
		_, _ = pool.Exec(ctx2, `DELETE FROM machine_sync_cursors WHERE machine_id = $1`, testfixtures.DevMachineID)
	})

	_, err := pool.Exec(ctx, `DELETE FROM machine_offline_events WHERE machine_id = $1`, testfixtures.DevMachineID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM machine_sync_cursors WHERE machine_id = $1`, testfixtures.DevMachineID)
	require.NoError(t, err)

	deps := offlineSyncIntegrationDeps(t, pool)
	deps.Config.OfflineSync.GapTolerant = true
	srv := &machineOfflineSyncServer{deps: deps}
	var credVer int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT credential_version FROM machines WHERE id = $1`, testfixtures.DevMachineID).Scan(&credVer))
	claims := plauth.MachineAccessClaims{MachineID: testfixtures.DevMachineID, CredentialVersion: credVer}
	ctxClaims := plauth.WithMachineAccessClaims(ctx, claims)

	base := "slot-occ-" + uuid.NewString()
	push := func(idem, rid string, seq int64, typ string, payload proto.Message) *machinev1.SyncOfflineEventsResponse {
		t.Helper()
		out, pErr := srv.PushOfflineEvents(ctxClaims, &machinev1.SyncOfflineEventsRequest{
			Meta: &machinev1.MachineRequestMeta{IdempotencyKey: idem, RequestId: rid},
			Events: []*machinev1.OfflineEvent{{
				Meta: &machinev1.MachineRequestMeta{
					OfflineSequence: seq,
					IdempotencyKey:  idem + ":oe",
					RequestId:       rid,
					ClientEventId:   rid + ":cli",
					OccurredAt:      timestamppb.Now(),
				},
				EventType: typ,
				Payload:   offlineProtoPayload(t, payload),
			}},
		})
		require.NoError(t, pErr)
		return out
	}

	co := &machinev1.CreateOrderRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  base + ":co",
			ClientEventId:   base + ":ce:co",
			ClientCreatedAt: timestamppb.Now(),
		},
		ProductId: testfixtures.DevProductCola.String(),
		Currency:  "USD",
		Slot:      &machinev1.SlotSelection{SlotIndex: ptrInt32(0)},
	}
	r1 := push(base+"sync1", "s1", 1, "commerce.create_order", co)
	require.Equal(t, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED, r1.GetResults()[0].GetStatus(), r1.GetResults()[0].GetReason())

	var orderID string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id::text FROM orders WHERE machine_id = $1 ORDER BY created_at DESC LIMIT 1`,
		testfixtures.DevMachineID).Scan(&orderID))

	cash := &machinev1.ConfirmCashPaymentRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  base + ":cash",
			ClientEventId:   base + ":ce:cash",
			ClientCreatedAt: timestamppb.Now(),
		},
		OrderId: orderID,
	}
	r2 := push(base+"sync2", "s2", 2, "commerce.confirm_cash_payment", cash)
	require.Equal(t, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED, r2.GetResults()[0].GetStatus(), r2.GetResults()[0].GetReason())

	vstart := &machinev1.StartVendRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  base + ":vs",
			ClientEventId:   base + ":ce:vs",
			ClientCreatedAt: timestamppb.Now(),
		},
		OrderId:   orderID,
		SlotIndex: 0,
	}
	r3 := push(base+"sync3", "s3", 3, "commerce.start_vend", vstart)
	require.Equal(t, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED, r3.GetResults()[0].GetStatus(), r3.GetResults()[0].GetReason())

	_, err = pool.Exec(ctx, `
INSERT INTO machine_offline_events (
    machine_id, offline_sequence, event_type, event_id, client_event_id,
    occurred_at, payload, processing_status, processing_error, idempotency_key
) VALUES (
    $1, 4, 'commerce.report_cash_movements', $2, $2,
    now(), '{}'::jsonb, 'rejected', 'invalid cash movements payload', $3
)`, testfixtures.DevMachineID, "poison-cli-"+base, "cash_movement:poison:"+base)
	require.NoError(t, err)

	commerceCursor := func() int64 {
		t.Helper()
		var last int64
		err := pool.QueryRow(ctx,
			`SELECT last_sequence FROM machine_sync_cursors WHERE machine_id = $1 AND stream_name = 'offline:commerce'`,
			testfixtures.DevMachineID,
		).Scan(&last)
		if err != nil {
			return 0
		}
		return last
	}
	require.Equal(t, int64(3), commerceCursor())

	vsucc := &machinev1.ConfirmVendSuccessRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  base + ":vsucc",
			ClientEventId:   base + ":ce:vsucc",
			ClientCreatedAt: timestamppb.Now(),
		},
		OrderId:   orderID,
		SlotIndex: 0,
	}
	occupied := push(base+"sync4", "s4", 4, "commerce.confirm_vend_success", vsucc)
	require.Len(t, occupied.GetResults(), 1)
	require.Equal(t, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_REPLAYED, occupied.GetResults()[0].GetStatus(), occupied.GetResults()[0].GetReason())
	require.Contains(t, occupied.GetResults()[0].GetReason(), "offline sequence slot occupied")
	require.Equal(t, int64(3), commerceCursor(), "slot occupied must not advance commerce cursor")
	require.Equal(t, "3", occupied.GetNextSyncCursor())

	var orderStatus string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1::uuid`, orderID).Scan(&orderStatus))
	require.NotEqual(t, "completed", orderStatus)

	retry := push(base+"sync5", "s5", 5, "commerce.confirm_vend_success", vsucc)
	require.Len(t, retry.GetResults(), 1)
	require.Equal(t, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED, retry.GetResults()[0].GetStatus(), retry.GetResults()[0].GetReason())
	require.Equal(t, int64(5), commerceCursor())

	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1::uuid`, orderID).Scan(&orderStatus))
	require.Equal(t, "completed", orderStatus)
}
