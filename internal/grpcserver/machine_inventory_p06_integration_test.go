package grpcserver

import (
	"context"
	"testing"

	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestP06_MachineGRPC_FillReport_AppliesExpectedStock(t *testing.T) {
	t.Parallel()

	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	srv, issuer := machineCommerceTestServer(t, pool, testMachineGRPCConfig())
	conn := dialMachineCommerceServer(t, srv)
	md := machineAccessMD(t, pool, issuer, testfixtures.DevMachineID, testfixtures.DevOrganizationID, testfixtures.DevSiteID)
	cli := machinev1.NewMachineInventoryServiceClient(conn)

	var qtyBefore int32
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT current_quantity FROM machine_slot_state WHERE machine_id = $1 AND slot_index = 0`,
		testfixtures.DevMachineID).Scan(&qtyBefore))

	idem := "p06-fill-" + uuid.NewString()
	_, err := cli.SubmitFillReport(md, &machinev1.SubmitFillResultRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  idem,
			ClientEventId:   "fill-evt-1",
			ClientCreatedAt: timestamppb.Now(),
		},
		Lines: []*machinev1.RestockLine{{
			PlanogramId:    testfixtures.DevPlanogramID.String(),
			SlotIndex:      0,
			ProductId:      ptrString(testfixtures.DevProductCola.String()),
			QuantityBefore: qtyBefore,
			QuantityAfter:  qtyBefore + 3,
		}},
	})
	require.NoError(t, err)

	var qtyAfter int32
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT current_quantity FROM machine_slot_state WHERE machine_id = $1 AND slot_index = 0`,
		testfixtures.DevMachineID).Scan(&qtyAfter))
	require.Equal(t, qtyBefore+3, qtyAfter)
}

func TestP06_MachineGRPC_StockAdjustment_IsAudited(t *testing.T) {
	t.Parallel()

	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	srv, issuer := machineCommerceTestServer(t, pool, testMachineGRPCConfig())
	conn := dialMachineCommerceServer(t, srv)
	md := machineAccessMD(t, pool, issuer, testfixtures.DevMachineID, testfixtures.DevOrganizationID, testfixtures.DevSiteID)
	cli := machinev1.NewMachineInventoryServiceClient(conn)

	var qtyBefore int32
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT current_quantity FROM machine_slot_state WHERE machine_id = $1 AND slot_index = 1`,
		testfixtures.DevMachineID).Scan(&qtyBefore))

	idem := "p06-adj-audit-" + uuid.NewString()
	_, err := cli.SubmitStockAdjustment(md, &machinev1.SubmitInventoryAdjustmentRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  idem,
			ClientEventId:   "adj-evt-1",
			ClientCreatedAt: timestamppb.Now(),
		},
		Reason: "manual_adjustment",
		Lines: []*machinev1.AdjustmentLine{{
			PlanogramId:    testfixtures.DevPlanogramID.String(),
			SlotIndex:      1,
			ProductId:      ptrString(testfixtures.DevProductWater.String()),
			QuantityBefore: qtyBefore,
			QuantityAfter:  qtyBefore - 1,
		}},
	})
	require.NoError(t, err)

	var n int
	require.NoError(t, pool.QueryRow(ctx, `
SELECT COUNT(*) FROM audit_events
WHERE organization_id = $1 AND action = $2 AND resource_id = $3`,
		testfixtures.DevOrganizationID,
		compliance.ActionInventoryAdjusted,
		testfixtures.DevMachineID.String(),
	).Scan(&n))
	require.GreaterOrEqual(t, n, 1)
}

func TestP06_MachineGRPC_StockAdjustment_IdempotentLedgerReplayNoDoubleDelta(t *testing.T) {
	t.Parallel()

	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	srv, issuer := machineCommerceTestServer(t, pool, testMachineGRPCConfig())
	conn := dialMachineCommerceServer(t, srv)
	md := machineAccessMD(t, pool, issuer, testfixtures.DevMachineID, testfixtures.DevOrganizationID, testfixtures.DevSiteID)
	cli := machinev1.NewMachineInventoryServiceClient(conn)

	var qtyBefore int32
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT current_quantity FROM machine_slot_state WHERE machine_id = $1 AND slot_index = 1`,
		testfixtures.DevMachineID).Scan(&qtyBefore))

	idem := "p06-adj-idem-" + uuid.NewString()
	req := &machinev1.SubmitInventoryAdjustmentRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  idem,
			ClientEventId:   "adj-stable",
			ClientCreatedAt: timestamppb.Now(),
		},
		Reason: "manual_adjustment",
		Lines: []*machinev1.AdjustmentLine{{
			PlanogramId:    testfixtures.DevPlanogramID.String(),
			SlotIndex:      1,
			ProductId:      ptrString(testfixtures.DevProductWater.String()),
			QuantityBefore: qtyBefore,
			QuantityAfter:  qtyBefore - 1,
		}},
	}
	r1, err := cli.SubmitStockAdjustment(md, req)
	require.NoError(t, err)
	require.False(t, r1.GetReplay())

	var qtyMid int32
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT current_quantity FROM machine_slot_state WHERE machine_id = $1 AND slot_index = 1`,
		testfixtures.DevMachineID).Scan(&qtyMid))
	require.Equal(t, qtyBefore-1, qtyMid)

	r2, err := cli.SubmitStockAdjustment(md, req)
	require.NoError(t, err)
	require.True(t, r2.GetReplay())

	var qtyAfter int32
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT current_quantity FROM machine_slot_state WHERE machine_id = $1 AND slot_index = 1`,
		testfixtures.DevMachineID).Scan(&qtyAfter))
	require.Equal(t, qtyMid, qtyAfter)
}

func ptrString(s string) *string { return &s }
