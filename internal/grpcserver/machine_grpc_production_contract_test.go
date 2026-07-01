package grpcserver

import (
	"context"
	"strings"
	"testing"

	"github.com/avf/avf-vending-api/internal/testfixtures"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

// machineGRPCUnauthenticatedMethods mirrors isUnauthenticatedGRPCMethod in interceptors.go.
func machineGRPCUnauthenticatedMethods() map[string]struct{} {
	return map[string]struct{}{
		machinev1.MachineAuthService_ActivateMachine_FullMethodName:       {},
		machinev1.MachineAuthService_ClaimActivation_FullMethodName:       {},
		machinev1.MachineAuthService_RefreshMachineToken_FullMethodName:   {},
		machinev1.MachineActivationService_ClaimActivation_FullMethodName: {},
		machinev1.MachineTokenService_RefreshMachineToken_FullMethodName:  {},
	}
}

func registerAllMachineGRPCServices(s *grpc.Server) {
	machinev1.RegisterMachineAuthServiceServer(s, &machinev1.UnimplementedMachineAuthServiceServer{})
	machinev1.RegisterMachineActivationServiceServer(s, &machinev1.UnimplementedMachineActivationServiceServer{})
	machinev1.RegisterMachineTokenServiceServer(s, &machinev1.UnimplementedMachineTokenServiceServer{})
	machinev1.RegisterMachineBootstrapServiceServer(s, &machinev1.UnimplementedMachineBootstrapServiceServer{})
	machinev1.RegisterMachineCatalogServiceServer(s, &machinev1.UnimplementedMachineCatalogServiceServer{})
	machinev1.RegisterMachineMediaServiceServer(s, &machinev1.UnimplementedMachineMediaServiceServer{})
	machinev1.RegisterMachineCommerceServiceServer(s, &machinev1.UnimplementedMachineCommerceServiceServer{})
	machinev1.RegisterMachineSaleServiceServer(s, &machinev1.UnimplementedMachineSaleServiceServer{})
	machinev1.RegisterMachineInventoryServiceServer(s, &machinev1.UnimplementedMachineInventoryServiceServer{})
	machinev1.RegisterMachineTelemetryServiceServer(s, &machinev1.UnimplementedMachineTelemetryServiceServer{})
	machinev1.RegisterMachineOperatorServiceServer(s, &machinev1.UnimplementedMachineOperatorServiceServer{})
	machinev1.RegisterMachineOfflineSyncServiceServer(s, &machinev1.UnimplementedMachineOfflineSyncServiceServer{})
	machinev1.RegisterMachineCommandServiceServer(s, &machinev1.UnimplementedMachineCommandServiceServer{})
}

func TestMachineProductionContract_AllRegisteredMethodsAreMachineNamespace(t *testing.T) {
	t.Parallel()

	s := grpc.NewServer()
	registerAllMachineGRPCServices(s)
	unauth := machineGRPCUnauthenticatedMethods()

	for svcName, info := range s.GetServiceInfo() {
		if !strings.HasPrefix(svcName, "avf.machine.v1.") {
			continue
		}
		for _, m := range info.Methods {
			full := "/" + svcName + "/" + m.Name
			if !isMachineGRPCMethod(full) {
				t.Fatalf("%s is not recognized as machine gRPC", full)
			}
			if _, ok := unauth[full]; ok {
				continue
			}
			if !requiresMachineAccessJWT(full) {
				t.Fatalf("%s must be listed in requiresMachineAccessJWT (production contract)", full)
			}
		}
	}
}

func TestMachineProductionContract_OnlyActivationAndRefreshSkipJWT(t *testing.T) {
	t.Parallel()

	want := machineGRPCUnauthenticatedMethods()
	if len(want) != 5 {
		t.Fatalf("expected 5 unauthenticated machine methods, got %d", len(want))
	}
	for method := range want {
		if !isMachineGRPCMethod(method) {
			t.Fatalf("%s must remain under avf.machine.v1", method)
		}
		if requiresMachineAccessJWT(method) {
			t.Fatalf("%s must not require machine JWT at interceptor allowlist", method)
		}
	}
}

func TestMachineProductionContract_NoTokenRejected(t *testing.T) {
	t.Parallel()
	testMachineGRPCMissingBearerRejected(t)
}

func TestMachineProductionContract_AdminUserJWTRejectedOnRuntimeRPC(t *testing.T) {
	t.Parallel()
	testMachineGRPCAdminUserJWTRejected(t)
}

func TestMachineProductionContract_ValidMachineTokenAccepted(t *testing.T) {
	t.Parallel()
	testMachineGRPCMachineJWTAccepted(t)
}

// TestMachineProductionContract_IdempotencyLedger enforces replay + payload mismatch (production contract).
func TestMachineProductionContract_IdempotencyLedger(t *testing.T) {
	TestMachineReplayLedger_ReplayAndConflict(t)
}

func TestMachineProductionContract_IdempotentRetryReturnsSameResult(t *testing.T) {
	TestMachineGRPC_Commerce_CreateOrder_IdempotentReplay(t)
}

func TestMachineProductionContract_IdempotencyPayloadMismatchRejected(t *testing.T) {
	TestMachineGRPC_Commerce_CreateOrder_IdempotentReplay(t)
}

func TestMachineProductionContract_RequiredMutatingRPCsAreIdempotent(t *testing.T) {
	t.Parallel()

	required := []string{
		machinev1.MachineBootstrapService_AckConfigVersion_FullMethodName,
		machinev1.MachineCatalogService_AckCatalogVersion_FullMethodName,
		machinev1.MachineMediaService_AckMediaVersion_FullMethodName,
		machinev1.MachineInventoryService_AckInventorySync_FullMethodName,
		machinev1.MachineCommerceService_CreateQuote_FullMethodName,
		machinev1.MachineCommerceService_CreateOrderFromQuote_FullMethodName,
		machinev1.MachineCommerceService_CreateOrder_FullMethodName,
		machinev1.MachineCommerceService_CreatePaymentSession_FullMethodName,
		machinev1.MachineCommerceService_ConfirmCashPayment_FullMethodName,
		machinev1.MachineCommerceService_CreateCashCheckout_FullMethodName,
		machinev1.MachineCommerceService_StartVend_FullMethodName,
		machinev1.MachineCommerceService_ReportVendSuccess_FullMethodName,
		machinev1.MachineCommerceService_ReportVendFailure_FullMethodName,
		machinev1.MachineOfflineSyncService_PushOfflineEvents_FullMethodName,
		machinev1.MachineOperatorService_SubmitStockAdjustment_FullMethodName,
		machinev1.MachineOperatorService_SubmitFillReport_FullMethodName,
	}
	for _, method := range required {
		if !isMachineIdempotentMutation(method) {
			t.Fatalf("%s must be ledger-idempotent for production Android runtime", method)
		}
	}
}

func TestMachineProductionContract_SuspendedMachineCannotCreateOrder(t *testing.T) {
	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	cfg := testMachineGRPCConfig()
	srv, issuer := machineCommerceTestServer(t, pool, cfg)
	conn := dialMachineCommerceServer(t, srv)
	md := machineAccessMD(t, pool, issuer, testfixtures.DevMachineID, testfixtures.DevSiteID)
	cli := machinev1.NewMachineCommerceServiceClient(conn)

	_, err := pool.Exec(ctx, `UPDATE machines SET status = 'suspended' WHERE id = $1`, testfixtures.DevMachineID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `UPDATE machines SET status = 'online', last_seen_at = now() WHERE id = $1`, testfixtures.DevMachineID)
	})

	_, err = cli.CreateOrder(md, &machinev1.CreateOrderRequest{
		Context:   testCommerceIdemCtx("suspend-order-"+uuid.NewString(), "evt-1"),
		ProductId: testfixtures.DevProductCola.String(),
		Currency:  "USD",
		Slot:      &machinev1.SlotSelection{SlotIndex: ptrInt32(0)},
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestMachineProductionContract_DisabledMachineCannotSell(t *testing.T) {
	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	cfg := testMachineGRPCConfig()
	srv, issuer := machineCommerceTestServer(t, pool, cfg)
	conn := dialMachineCommerceServer(t, srv)
	md := machineAccessMD(t, pool, issuer, testfixtures.DevMachineID, testfixtures.DevSiteID)
	cli := machinev1.NewMachineCommerceServiceClient(conn)

	_, err := pool.Exec(ctx, `UPDATE machines SET status = 'maintenance' WHERE id = $1`, testfixtures.DevMachineID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `UPDATE machines SET status = 'online', last_seen_at = now() WHERE id = $1`, testfixtures.DevMachineID)
	})

	_, err = cli.CreateOrder(md, &machinev1.CreateOrderRequest{
		Context:   testCommerceIdemCtx("maint-order-"+uuid.NewString(), "evt-1"),
		ProductId: testfixtures.DevProductCola.String(),
		Currency:  "USD",
		Slot:      &machinev1.SlotSelection{SlotIndex: ptrInt32(0)},
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestMachineProductionContract_VendSuccessRejectedBeforeStartVend(t *testing.T) {
	pool := machineGRPCTestPool(t)
	cfg := testMachineGRPCConfig()
	srv, issuer := machineCommerceTestServer(t, pool, cfg)
	conn := dialMachineCommerceServer(t, srv)
	md := machineAccessMD(t, pool, issuer, testfixtures.DevMachineID, testfixtures.DevSiteID)
	cli := machinev1.NewMachineCommerceServiceClient(conn)

	idem := "vend-before-start-" + uuid.NewString()
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

	_, err = cli.ConfirmVendSuccess(md, &machinev1.ConfirmVendSuccessRequest{
		Context:   testCommerceIdemCtx(idem+":vsucc", "evt-vsucc"),
		OrderId:   co.GetOrderId(),
		SlotIndex: 0,
	})
	require.Error(t, err)
	require.NotEqual(t, codes.OK, status.Code(err))
}

func TestMachineProductionContract_VendFailureRejectedBeforeStartVend(t *testing.T) {
	pool := machineGRPCTestPool(t)
	cfg := testMachineGRPCConfig()
	srv, issuer := machineCommerceTestServer(t, pool, cfg)
	conn := dialMachineCommerceServer(t, srv)
	md := machineAccessMD(t, pool, issuer, testfixtures.DevMachineID, testfixtures.DevSiteID)
	cli := machinev1.NewMachineCommerceServiceClient(conn)

	idem := "fail-before-start-" + uuid.NewString()
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

	_, err = cli.ReportVendFailure(md, &machinev1.ReportVendFailureRequest{
		Context:       testCommerceIdemCtx(idem+":vfail", "evt-vfail"),
		OrderId:       co.GetOrderId(),
		SlotIndex:     0,
		FailureReason: "jam",
	})
	require.Error(t, err)
	require.NotEqual(t, codes.OK, status.Code(err))
}

func TestMachineProductionContract_BootstrapExposesSellReadiness(t *testing.T) {
	pool := machineGRPCTestPool(t)
	cfg := testMachineGRPCConfig()
	srv, issuer := machineCommerceTestServer(t, pool, cfg)
	conn := dialMachineCommerceServer(t, srv)
	md := machineAccessMD(t, pool, issuer, testfixtures.DevMachineID, testfixtures.DevSiteID)
	cli := machinev1.NewMachineBootstrapServiceClient(conn)

	resp, err := cli.GetBootstrap(md, &machinev1.GetBootstrapRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp.GetRuntimeHints())
	require.NotNil(t, resp.GetRuntimeHints().GetSellReadiness())
}

func TestMachineProductionContract_BootstrapTopologyIncludesCabinetMetadata(t *testing.T) {
	pool := machineGRPCTestPool(t)
	cfg := testMachineGRPCConfig()
	srv, issuer := machineCommerceTestServer(t, pool, cfg)
	conn := dialMachineCommerceServer(t, srv)
	md := machineAccessMD(t, pool, issuer, testfixtures.DevMachineID, testfixtures.DevSiteID)
	cli := machinev1.NewMachineBootstrapServiceClient(conn)

	resp, err := cli.GetBootstrap(md, &machinev1.GetBootstrapRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp.GetTopology())
	require.NotEmpty(t, resp.GetTopology().GetCabinets())
	first := resp.GetTopology().GetCabinets()[0]
	require.NotNil(t, first.GetMetadata(), "cabinet metadata struct must be present (possibly empty before repair)")
}

func TestBootstrapCabinetMetadata_StructRoundTrip(t *testing.T) {
	meta := map[string]any{
		"machine_type":      "tcn",
		"board_protocol":    "tcn",
		"bill_protocol":     "ict_bc_v1",
		"cash_topology":     "direct_bill",
		"payment_authority": "local",
		"transport_type":    "serial",
		"baud_rate":         float64(9600),
		"driver_options": map[string]any{
			"billBusKey":               "/dev/ttyS1",
			"billSharesBoardSerialBus": "false",
		},
	}
	s, err := structpb.NewStruct(meta)
	require.NoError(t, err)
	require.Equal(t, "tcn", s.GetFields()["board_protocol"].GetStringValue())
	require.Equal(t, "local", s.GetFields()["payment_authority"].GetStringValue())
	require.NotNil(t, s.GetFields()["driver_options"].GetStructValue())
}
