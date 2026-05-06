package grpcserver

import (
	"context"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/activation"
	"github.com/avf/avf-vending-api/internal/app/api"
	appaudit "github.com/avf/avf-vending-api/internal/app/audit"
	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/app/salecatalog"
	"github.com/avf/avf-vending-api/internal/app/workfloworch"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	platformpayments "github.com/avf/avf-vending-api/internal/platform/payments"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
)

func testCommerceIdemCtx(key, clientEvt string) *machinev1.IdempotencyContext {
	return &machinev1.IdempotencyContext{
		IdempotencyKey:  key,
		ClientEventId:   clientEvt,
		ClientCreatedAt: timestamppb.Now(),
	}
}

func machineCommerceTestServer(t *testing.T, pool *pgxpool.Pool, cfg *config.Config) (*Server, *plauth.SessionIssuer) {
	t.Helper()
	if cfg == nil {
		cfg = testMachineGRPCConfig()
	}
	issuer, err := plauth.NewSessionIssuerFromHTTPAuth(cfg.HTTPAuth)
	require.NoError(t, err)
	pepper := plauth.TrimSecret(cfg.HTTPAuth.JWTSecret)
	act := activation.NewService(pool, issuer, pepper, nil)
	store := postgres.NewStore(pool)
	auditSvc := appaudit.NewService(pool)
	payReg := platformpayments.NewRegistry(cfg)
	commerceSvc := appcommerce.NewService(appcommerce.Deps{
		OrderVend:              store,
		PaymentOutbox:          store,
		Lifecycle:              store,
		WebhookPersist:         store,
		SaleLines:              store,
		WorkflowOrchestration:  workfloworch.NewDisabled(),
		EnterpriseAudit:        auditSvc,
		PaymentSessionRegistry: payReg,
	})
	machineQueries := api.NewInternalMachineQueryService(store, api.NewSQLMachineShadow(pool))
	replayLedger := NewMachineReplayLedger(pool, auditSvc)
	srv, err := NewServer(cfg, zap.NewNop(), nil, nil, nil, replayLedger, nil, nil, RegisterMachineGRPCServices(MachineGRPCServicesDeps{
		Activation:      act,
		MachineQueries:  machineQueries,
		FeatureFlags:    nil,
		SaleCatalog:     salecatalog.NewService(pool),
		Pool:            pool,
		MQTTBrokerURL:   "tcp://example.invalid",
		MQTTTopicPrefix: "avf/devices",
		Config:          cfg,
		InventoryLedger: postgres.NewInventoryRepository(pool),
		Commerce:        commerceSvc,
		TelemetryStore:  store,
		EnterpriseAudit: auditSvc,
	}))
	require.NoError(t, err)
	return srv, issuer
}

func machineAccessMD(t *testing.T, pool *pgxpool.Pool, issuer *plauth.SessionIssuer, machineID, orgID, siteID uuid.UUID) context.Context {
	t.Helper()
	ctx := context.Background()
	var credVer int64
	err := pool.QueryRow(ctx, `SELECT credential_version FROM machines WHERE id = $1`, machineID).Scan(&credVer)
	require.NoError(t, err)
	tok, _, err := issuer.IssueMachineAccessJWT(machineID, orgID, siteID, credVer, uuid.Nil)
	require.NoError(t, err)
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+tok)
}

func dialMachineCommerceServer(t *testing.T, srv *Server) *grpc.ClientConn {
	t.Helper()
	srvCtx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe(srvCtx) }()
	t.Cleanup(func() {
		cancel()
		<-errCh
	})
	connCtx, connCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer connCancel()
	conn, err := grpc.DialContext(connCtx, srv.ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// Cola on planogram slot_index 0, price 150 per dev seed.
func TestMachineGRPC_Commerce_CashSale_EndToEnd(t *testing.T) {
	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	cfg := testMachineGRPCConfig()
	srv, issuer := machineCommerceTestServer(t, pool, cfg)
	conn := dialMachineCommerceServer(t, srv)
	md := machineAccessMD(t, pool, issuer, testfixtures.DevMachineID, testfixtures.DevOrganizationID, testfixtures.DevSiteID)
	cli := machinev1.NewMachineCommerceServiceClient(conn)

	idem := "cash-sale-" + uuid.NewString()
	co, err := cli.CreateOrder(md, &machinev1.CreateOrderRequest{
		Context:   testCommerceIdemCtx(idem, "evt-co-1"),
		ProductId: testfixtures.DevProductCola.String(),
		Currency:  "USD",
		Slot:      &machinev1.SlotSelection{SlotIndex: ptrInt32(0)},
	})
	require.NoError(t, err)
	require.Equal(t, int64(150), co.GetTotalMinor())

	_, err = cli.ConfirmCashPayment(md, &machinev1.ConfirmCashPaymentRequest{
		Context: testCommerceIdemCtx(idem+":cash", "evt-cash-1"),
		OrderId: co.GetOrderId(),
	})
	require.NoError(t, err)

	_, err = cli.StartVend(md, &machinev1.StartVendRequest{
		Context:   testCommerceIdemCtx(idem+":vend", "evt-vstart-1"),
		OrderId:   co.GetOrderId(),
		SlotIndex: 0,
	})
	require.NoError(t, err)

	var qtyBefore int32
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT current_quantity FROM machine_slot_state WHERE machine_id = $1 AND slot_index = 0`,
		testfixtures.DevMachineID,
	).Scan(&qtyBefore))

	vsuccCtx := testCommerceIdemCtx(idem+":vsucc", "evt-vsucc-1")
	succ, err := cli.ConfirmVendSuccess(md, &machinev1.ConfirmVendSuccessRequest{
		Context:   vsuccCtx,
		OrderId:   co.GetOrderId(),
		SlotIndex: 0,
	})
	require.NoError(t, err)
	require.False(t, succ.GetReplay())
	require.Equal(t, "completed", succ.GetOrderStatus())
	require.Equal(t, "success", succ.GetVendState())

	var qtyAfter int32
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT current_quantity FROM machine_slot_state WHERE machine_id = $1 AND slot_index = 0`,
		testfixtures.DevMachineID,
	).Scan(&qtyAfter))
	require.Equal(t, qtyBefore-1, qtyAfter)

	succ2, err := cli.ConfirmVendSuccess(md, &machinev1.ConfirmVendSuccessRequest{
		Context:   vsuccCtx,
		OrderId:   co.GetOrderId(),
		SlotIndex: 0,
	})
	require.NoError(t, err)
	require.True(t, succ2.GetReplay())

	var qtyAfter2 int32
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT current_quantity FROM machine_slot_state WHERE machine_id = $1 AND slot_index = 0`,
		testfixtures.DevMachineID,
	).Scan(&qtyAfter2))
	require.Equal(t, qtyAfter, qtyAfter2, "replay must not decrement again")
}

func TestMachineGRPC_Commerce_QRFlow_WebhookThenVend(t *testing.T) {
	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	cfg := testMachineGRPCConfig()
	srv, issuer := machineCommerceTestServer(t, pool, cfg)
	store := postgres.NewStore(pool)
	commerceSvc := appcommerce.NewService(appcommerce.Deps{
		OrderVend:              store,
		PaymentOutbox:          store,
		Lifecycle:              store,
		WebhookPersist:         store,
		SaleLines:              store,
		WorkflowOrchestration:  workfloworch.NewDisabled(),
		EnterpriseAudit:        appaudit.NewService(pool),
		PaymentSessionRegistry: platformpayments.NewRegistry(cfg),
	})
	conn := dialMachineCommerceServer(t, srv)
	md := machineAccessMD(t, pool, issuer, testfixtures.DevMachineID, testfixtures.DevOrganizationID, testfixtures.DevSiteID)
	cli := machinev1.NewMachineCommerceServiceClient(conn)

	idem := "qr-sale-" + uuid.NewString()
	co, err := cli.CreateOrder(md, &machinev1.CreateOrderRequest{
		Context:   testCommerceIdemCtx(idem, "evt-q-co"),
		ProductId: testfixtures.DevProductWater.String(),
		Currency:  "USD",
		Slot:      &machinev1.SlotSelection{SlotIndex: ptrInt32(1)},
	})
	require.NoError(t, err)
	require.Equal(t, int64(120), co.GetTotalMinor())

	payOut, err := cli.CreatePaymentSession(md, &machinev1.CreatePaymentSessionRequest{
		Context:      testCommerceIdemCtx(idem+":pay", "evt-pay"),
		OrderId:      co.GetOrderId(),
		Provider:     "psp_grpc_int",
		PaymentState: "created",
		AmountMinor:  co.GetTotalMinor(),
		Currency:     "USD",
	})
	require.NoError(t, err)
	require.NotEmpty(t, payOut.GetQrPayloadOrUrl())

	payID := uuid.MustParse(payOut.GetPaymentId())
	providerRef := "sb_" + payID.String()
	_, err = commerceSvc.ApplyPaymentProviderWebhook(ctx, appcommerce.ApplyPaymentProviderWebhookInput{
		OrganizationID:          testfixtures.DevOrganizationID,
		OrderID:                 uuid.MustParse(co.GetOrderId()),
		PaymentID:               uuid.MustParse(payOut.GetPaymentId()),
		Provider:                "psp_grpc_int",
		ProviderReference:       providerRef,
		WebhookEventID:          "wh-" + idem,
		EventType:               "payment.captured",
		NormalizedPaymentState:  "captured",
		Payload:                 []byte(`{"ok":true}`),
		WebhookValidationStatus: "unsigned_development",
	})
	require.NoError(t, err)

	got, err := cli.GetOrder(md, &machinev1.GetOrderRequest{OrderId: co.GetOrderId(), SlotIndex: 1})
	require.NoError(t, err)
	require.Equal(t, "paid", got.GetOrderStatus())
	require.Equal(t, "captured", got.GetPaymentState())

	_, err = cli.StartVend(md, &machinev1.StartVendRequest{
		Context:   testCommerceIdemCtx(idem+":vend", "evt-vstart"),
		OrderId:   co.GetOrderId(),
		SlotIndex: 1,
	})
	require.NoError(t, err)

	succ, err := cli.ConfirmVendSuccess(md, &machinev1.ConfirmVendSuccessRequest{
		Context:   testCommerceIdemCtx(idem+":vsucc", "evt-vsucc"),
		OrderId:   co.GetOrderId(),
		SlotIndex: 1,
	})
	require.NoError(t, err)
	require.Equal(t, "completed", succ.GetOrderStatus())
}

func TestMachineGRPC_Commerce_StartVend_BlockedBeforePayment(t *testing.T) {
	pool := machineGRPCTestPool(t)
	srv, issuer := machineCommerceTestServer(t, pool, testMachineGRPCConfig())
	conn := dialMachineCommerceServer(t, srv)
	md := machineAccessMD(t, pool, issuer, testfixtures.DevMachineID, testfixtures.DevOrganizationID, testfixtures.DevSiteID)
	cli := machinev1.NewMachineCommerceServiceClient(conn)

	idem := "block-vend-" + uuid.NewString()
	co, err := cli.CreateOrder(md, &machinev1.CreateOrderRequest{
		Context:   testCommerceIdemCtx(idem, "evt-1"),
		ProductId: testfixtures.DevProductCola.String(),
		Currency:  "USD",
		Slot:      &machinev1.SlotSelection{SlotIndex: ptrInt32(0)},
	})
	require.NoError(t, err)

	_, err = cli.StartVend(md, &machinev1.StartVendRequest{
		Context:   testCommerceIdemCtx(idem+":v", "evt-v"),
		OrderId:   co.GetOrderId(),
		SlotIndex: 0,
	})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestMachineGRPC_Commerce_CreateOrder_IdempotentReplay(t *testing.T) {
	pool := machineGRPCTestPool(t)
	srv, issuer := machineCommerceTestServer(t, pool, testMachineGRPCConfig())
	conn := dialMachineCommerceServer(t, srv)
	md := machineAccessMD(t, pool, issuer, testfixtures.DevMachineID, testfixtures.DevOrganizationID, testfixtures.DevSiteID)
	cli := machinev1.NewMachineCommerceServiceClient(conn)

	idem := "idem-co-" + uuid.NewString()
	ts := timestamppb.Now()
	sharedCtx := &machinev1.IdempotencyContext{
		IdempotencyKey:  idem,
		ClientEventId:   "evt-a",
		ClientCreatedAt: ts,
	}
	r1, err := cli.CreateOrder(md, &machinev1.CreateOrderRequest{
		Context:   sharedCtx,
		ProductId: testfixtures.DevProductCola.String(),
		Currency:  "USD",
		Slot:      &machinev1.SlotSelection{SlotIndex: ptrInt32(0)},
	})
	require.NoError(t, err)
	require.False(t, r1.GetReplay())

	r2, err := cli.CreateOrder(md, &machinev1.CreateOrderRequest{
		Context:   sharedCtx,
		ProductId: testfixtures.DevProductCola.String(),
		Currency:  "USD",
		Slot:      &machinev1.SlotSelection{SlotIndex: ptrInt32(0)},
	})
	require.NoError(t, err)
	require.True(t, r2.GetReplay())
	require.Equal(t, r1.GetOrderId(), r2.GetOrderId())

	_, err = cli.CreateOrder(md, &machinev1.CreateOrderRequest{
		Context:   testCommerceIdemCtx(idem, "evt-b"),
		ProductId: testfixtures.DevProductWater.String(),
		Currency:  "USD",
		Slot:      &machinev1.SlotSelection{SlotIndex: ptrInt32(1)},
	})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Contains(t, status.Convert(err).Message(), "idempotency_payload_mismatch")
}

func TestMachineGRPC_Commerce_CreatePaymentSession_IdempotentReplay_NoDuplicatePaymentRow(t *testing.T) {
	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	srv, issuer := machineCommerceTestServer(t, pool, testMachineGRPCConfig())
	conn := dialMachineCommerceServer(t, srv)
	md := machineAccessMD(t, pool, issuer, testfixtures.DevMachineID, testfixtures.DevOrganizationID, testfixtures.DevSiteID)
	cli := machinev1.NewMachineCommerceServiceClient(conn)

	idem := "idem-pay-" + uuid.NewString()
	co, err := cli.CreateOrder(md, &machinev1.CreateOrderRequest{
		Context:   testCommerceIdemCtx(idem, "evt-co"),
		ProductId: testfixtures.DevProductWater.String(),
		Currency:  "USD",
		Slot:      &machinev1.SlotSelection{SlotIndex: ptrInt32(1)},
	})
	require.NoError(t, err)

	payCtx := testCommerceIdemCtx(idem+":pay", "evt-pay-stable")
	req := &machinev1.CreatePaymentSessionRequest{
		Context:      payCtx,
		OrderId:      co.GetOrderId(),
		Provider:     "psp_grpc_int",
		PaymentState: "created",
		AmountMinor:  co.GetTotalMinor(),
		Currency:     "USD",
	}
	p1, err := cli.CreatePaymentSession(md, req)
	require.NoError(t, err)
	require.False(t, p1.GetReplay())
	require.NotEmpty(t, p1.GetPaymentId())

	p2, err := cli.CreatePaymentSession(md, req)
	require.NoError(t, err)
	require.True(t, p2.GetReplay())
	require.Equal(t, p1.GetPaymentId(), p2.GetPaymentId())

	var n int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM payments WHERE id = $1::uuid`, p1.GetPaymentId()).Scan(&n)
	require.NoError(t, err)
	require.Equal(t, 1, n)
}

func TestMachineGRPC_Commerce_GetOrder_WrongMachineDenied(t *testing.T) {
	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	otherID := uuid.New()
	hw := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	_, err := pool.Exec(ctx, `
INSERT INTO machines (id, organization_id, site_id, hardware_profile_id, serial_number, name, status, command_sequence, credential_version)
VALUES ($1, $2, $3, $4, $5, $6, 'online', 0, 0)`,
		otherID, testfixtures.DevOrganizationID, testfixtures.DevSiteID, hw, "sn-other-grpc", "other")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM machines WHERE id = $1`, otherID)
	})

	srv, issuer := machineCommerceTestServer(t, pool, testMachineGRPCConfig())
	conn := dialMachineCommerceServer(t, srv)
	mdDev := machineAccessMD(t, pool, issuer, testfixtures.DevMachineID, testfixtures.DevOrganizationID, testfixtures.DevSiteID)
	cli := machinev1.NewMachineCommerceServiceClient(conn)

	idem := "xmachine-" + uuid.NewString()
	co, err := cli.CreateOrder(mdDev, &machinev1.CreateOrderRequest{
		Context:   testCommerceIdemCtx(idem, "evt-1"),
		ProductId: testfixtures.DevProductCola.String(),
		Currency:  "USD",
		Slot:      &machinev1.SlotSelection{SlotIndex: ptrInt32(0)},
	})
	require.NoError(t, err)

	mdOther := machineAccessMD(t, pool, issuer, otherID, testfixtures.DevOrganizationID, testfixtures.DevSiteID)
	_, err = cli.GetOrder(mdOther, &machinev1.GetOrderRequest{OrderId: co.GetOrderId(), SlotIndex: 0})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestMachineGRPC_Commerce_ExpiredCheckoutWindow_Blocked(t *testing.T) {
	pool := machineGRPCTestPool(t)
	cfg := testMachineGRPCConfig()
	cfg.Commerce.MachineOrderCheckoutMaxAge = 50 * time.Millisecond
	srv, issuer := machineCommerceTestServer(t, pool, cfg)
	conn := dialMachineCommerceServer(t, srv)
	md := machineAccessMD(t, pool, issuer, testfixtures.DevMachineID, testfixtures.DevOrganizationID, testfixtures.DevSiteID)
	cli := machinev1.NewMachineCommerceServiceClient(conn)

	idem := "exp-" + uuid.NewString()
	co, err := cli.CreateOrder(md, &machinev1.CreateOrderRequest{
		Context:   testCommerceIdemCtx(idem, "evt-1"),
		ProductId: testfixtures.DevProductCola.String(),
		Currency:  "USD",
		Slot:      &machinev1.SlotSelection{SlotIndex: ptrInt32(0)},
	})
	require.NoError(t, err)

	time.Sleep(80 * time.Millisecond)

	_, err = cli.ConfirmCashPayment(md, &machinev1.ConfirmCashPaymentRequest{
		Context: testCommerceIdemCtx(idem+":c", "evt-c"),
		OrderId: co.GetOrderId(),
	})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestP06_MachineGRPC_CreateOrder_MissingIdempotencyKeyRejected(t *testing.T) {
	pool := machineGRPCTestPool(t)
	srv, issuer := machineCommerceTestServer(t, pool, testMachineGRPCConfig())
	conn := dialMachineCommerceServer(t, srv)
	md := machineAccessMD(t, pool, issuer, testfixtures.DevMachineID, testfixtures.DevOrganizationID, testfixtures.DevSiteID)
	cli := machinev1.NewMachineCommerceServiceClient(conn)

	_, err := cli.CreateOrder(md, &machinev1.CreateOrderRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  "",
			ClientEventId:   "evt-no-idem",
			ClientCreatedAt: timestamppb.Now(),
		},
		ProductId: testfixtures.DevProductCola.String(),
		Currency:  "USD",
		Slot:      &machinev1.SlotSelection{SlotIndex: ptrInt32(0)},
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func ptrInt32(v int32) *int32 { return &v }
