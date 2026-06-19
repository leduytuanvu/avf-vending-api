package grpcserver

import (
	"context"
	"testing"

	"github.com/avf/avf-vending-api/internal/testfixtures"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func testVendHardwareEvidenceProto() *machinev1.VendHardwareEvidence {
	vendAttempt := uuid.New()
	corr := uuid.New()
	digest := "tcn-digest-hex"
	return &machinev1.VendHardwareEvidence{
		VendAttemptId: vendAttempt.String(),
		CorrelationId: corr.String(),
		Command: &machinev1.HardwareCommandRef{
			CommandId:  uuid.NewString(),
			TxRxDigest: "txrx-sha256-abc",
		},
		BillFinal: &machinev1.BillFinalRecord{
			EventId:     "bill-final-1",
			AmountMinor: 150,
			Currency:    "USD",
		},
		TcnDispense: &machinev1.TcnDispenseRecord{
			Slot:    "A1",
			Result:  "ok",
			Dropped: true,
			Digest:  &digest,
		},
	}
}

func cashCheckoutThroughStartVend(t *testing.T, cli machinev1.MachineCommerceServiceClient, md context.Context, idem string) *machinev1.CreateOrderResponse {
	t.Helper()
	co, err := cli.CreateOrder(md, &machinev1.CreateOrderRequest{
		Context:   testCommerceIdemCtx(idem, "evt-co"),
		ProductId: testfixtures.DevProductCola.String(),
		Currency:  "USD",
		Slot:      &machinev1.SlotSelection{SlotIndex: ptrInt32(0)},
	})
	require.NoError(t, err)
	_, err = cli.ConfirmCashPayment(md, &machinev1.ConfirmCashPaymentRequest{
		Context: testCommerceIdemCtx(idem+":cash", "evt-cash"),
		OrderId: co.GetOrderId(),
	})
	require.NoError(t, err)
	_, err = cli.StartVend(md, &machinev1.StartVendRequest{
		Context:   testCommerceIdemCtx(idem+":vstart", "evt-vstart"),
		OrderId:   co.GetOrderId(),
		SlotIndex: 0,
	})
	require.NoError(t, err)
	return co
}

func TestMachineGRPC_Commerce_ConfirmVendSuccess_RequireEvidenceFlag(t *testing.T) {
	pool := machineGRPCTestPool(t)
	cfg := testMachineGRPCConfig()
	cfg.Commerce.RequireVendHardwareEvidence = true
	srv, issuer := machineCommerceTestServer(t, pool, cfg)
	conn := dialMachineCommerceServer(t, srv)
	md := machineAccessMD(t, pool, issuer, testfixtures.DevMachineID, testfixtures.DevSiteID)
	cli := machinev1.NewMachineCommerceServiceClient(conn)

	idem := "evidence-required-" + uuid.NewString()
	co := cashCheckoutThroughStartVend(t, cli, md, idem)

	_, err := cli.ConfirmVendSuccess(md, &machinev1.ConfirmVendSuccessRequest{
		Context:   testCommerceIdemCtx(idem+":vsucc", "evt-vsucc"),
		OrderId:   co.GetOrderId(),
		SlotIndex: 0,
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Contains(t, status.Convert(err).Message(), vendHardwareEvidenceRequiredMsg)
}

func TestMachineGRPC_Commerce_ConfirmVendSuccess_WithEvidence_Verified(t *testing.T) {
	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	cfg := testMachineGRPCConfig()
	cfg.Commerce.RequireVendHardwareEvidence = true
	srv, issuer := machineCommerceTestServer(t, pool, cfg)
	conn := dialMachineCommerceServer(t, srv)
	md := machineAccessMD(t, pool, issuer, testfixtures.DevMachineID, testfixtures.DevSiteID)
	cli := machinev1.NewMachineCommerceServiceClient(conn)

	idem := "evidence-verified-" + uuid.NewString()
	co := cashCheckoutThroughStartVend(t, cli, md, idem)
	evidence := testVendHardwareEvidenceProto()

	succ, err := cli.ConfirmVendSuccess(md, &machinev1.ConfirmVendSuccessRequest{
		Context:   testCommerceIdemCtx(idem+":vsucc", "evt-vsucc"),
		OrderId:   co.GetOrderId(),
		SlotIndex: 0,
		Evidence:  evidence,
	})
	require.NoError(t, err)
	require.Equal(t, "completed", succ.GetOrderStatus())

	var verificationStatus string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT verification_status FROM vend_sessions WHERE order_id = $1 AND slot_index = 0`,
		co.GetOrderId(),
	).Scan(&verificationStatus))
	require.Equal(t, "verified", verificationStatus)

	var evidenceCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM vend_hardware_evidence WHERE order_id = $1`,
		co.GetOrderId(),
	).Scan(&evidenceCount))
	require.Equal(t, 1, evidenceCount)
}

func TestMachineGRPC_Commerce_ConfirmVendSuccess_NoEvidence_HardwareUnverifiedWhenFlagOff(t *testing.T) {
	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	cfg := testMachineGRPCConfig()
	cfg.Commerce.RequireVendHardwareEvidence = false
	srv, issuer := machineCommerceTestServer(t, pool, cfg)
	conn := dialMachineCommerceServer(t, srv)
	md := machineAccessMD(t, pool, issuer, testfixtures.DevMachineID, testfixtures.DevSiteID)
	cli := machinev1.NewMachineCommerceServiceClient(conn)

	idem := "evidence-soft-" + uuid.NewString()
	co := cashCheckoutThroughStartVend(t, cli, md, idem)

	succ, err := cli.ConfirmVendSuccess(md, &machinev1.ConfirmVendSuccessRequest{
		Context:   testCommerceIdemCtx(idem+":vsucc", "evt-vsucc"),
		OrderId:   co.GetOrderId(),
		SlotIndex: 0,
	})
	require.NoError(t, err)
	require.Equal(t, "completed", succ.GetOrderStatus())

	var verificationStatus string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT verification_status FROM vend_sessions WHERE order_id = $1 AND slot_index = 0`,
		co.GetOrderId(),
	).Scan(&verificationStatus))
	require.Equal(t, "hardware_unverified", verificationStatus)
}

func TestMachineGRPC_Commerce_ConfirmVendSuccess_SameKeyDifferentEvidence_Conflict(t *testing.T) {
	pool := machineGRPCTestPool(t)
	cfg := testMachineGRPCConfig()
	cfg.Commerce.RequireVendHardwareEvidence = true
	srv, issuer := machineCommerceTestServer(t, pool, cfg)
	conn := dialMachineCommerceServer(t, srv)
	md := machineAccessMD(t, pool, issuer, testfixtures.DevMachineID, testfixtures.DevSiteID)
	cli := machinev1.NewMachineCommerceServiceClient(conn)

	idem := "evidence-conflict-" + uuid.NewString()
	co := cashCheckoutThroughStartVend(t, cli, md, idem)
	vsuccCtx := testCommerceIdemCtx(idem+":vsucc", "evt-vsucc")
	evidence1 := testVendHardwareEvidenceProto()

	_, err := cli.ConfirmVendSuccess(md, &machinev1.ConfirmVendSuccessRequest{
		Context:   vsuccCtx,
		OrderId:   co.GetOrderId(),
		SlotIndex: 0,
		Evidence:  evidence1,
	})
	require.NoError(t, err)

	evidence2 := testVendHardwareEvidenceProto()
	_, err = cli.ConfirmVendSuccess(md, &machinev1.ConfirmVendSuccessRequest{
		Context:   vsuccCtx,
		OrderId:   co.GetOrderId(),
		SlotIndex: 0,
		Evidence:  evidence2,
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Contains(t, status.Convert(err).Message(), "idempotency_payload_mismatch")
}
