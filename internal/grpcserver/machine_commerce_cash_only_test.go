package grpcserver

import (
	"testing"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func testMachineGRPCCashOnlyConfig() *config.Config {
	cfg := testMachineGRPCConfig()
	cfg.AppEnv = config.AppEnvProduction
	cfg.PaymentEnv = config.PaymentEnvCashOnly
	cfg.Commerce.DefaultPaymentProvider = ""
	return cfg
}

func TestMachineGRPC_Commerce_CreatePaymentSession_CashOnlyRejected(t *testing.T) {
	pool := machineGRPCTestPool(t)
	cfg := testMachineGRPCCashOnlyConfig()
	srv, issuer := machineCommerceTestServer(t, pool, cfg)
	conn := dialMachineCommerceServer(t, srv)
	md := machineAccessMD(t, pool, issuer, testfixtures.DevMachineID, testfixtures.DevSiteID)
	cli := machinev1.NewMachineCommerceServiceClient(conn)

	idem := "cashonly-pay-" + uuid.NewString()
	co, err := cli.CreateOrder(md, &machinev1.CreateOrderRequest{
		Context:   testCommerceIdemCtx(idem, "evt-co"),
		ProductId: testfixtures.DevProductCola.String(),
		Currency:  "USD",
		Slot:      &machinev1.SlotSelection{SlotIndex: ptrInt32(0)},
	})
	require.NoError(t, err)

	_, err = cli.CreatePaymentSession(md, &machinev1.CreatePaymentSessionRequest{
		Context:      testCommerceIdemCtx(idem+":pay", "evt-pay"),
		OrderId:      co.GetOrderId(),
		Provider:     "stripe",
		PaymentState: "created",
		AmountMinor:  co.GetTotalMinor(),
		Currency:     "USD",
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Contains(t, status.Convert(err).Message(), "provider_unavailable")
}

func TestMachineGRPC_Commerce_CashOnly_CashCheckoutStillWorks(t *testing.T) {
	pool := machineGRPCTestPool(t)
	cfg := testMachineGRPCCashOnlyConfig()
	srv, issuer := machineCommerceTestServer(t, pool, cfg)
	conn := dialMachineCommerceServer(t, srv)
	md := machineAccessMD(t, pool, issuer, testfixtures.DevMachineID, testfixtures.DevSiteID)
	cli := machinev1.NewMachineCommerceServiceClient(conn)

	idem := "cashonly-cash-" + uuid.NewString()
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

	got, err := cli.GetOrderStatus(md, &machinev1.GetOrderStatusRequest{OrderId: co.GetOrderId()})
	require.NoError(t, err)
	require.Equal(t, "paid", got.GetOrderStatus())
}

func TestMachineGRPC_Bootstrap_CashOnlyPaymentMethods(t *testing.T) {
	pool := machineGRPCTestPool(t)
	cfg := testMachineGRPCCashOnlyConfig()
	srv, issuer := machineCommerceTestServer(t, pool, cfg)
	conn := dialMachineCommerceServer(t, srv)
	md := machineAccessMD(t, pool, issuer, testfixtures.DevMachineID, testfixtures.DevSiteID)
	cli := machinev1.NewMachineBootstrapServiceClient(conn)

	resp, err := cli.GetBootstrap(md, &machinev1.GetBootstrapRequest{})
	require.NoError(t, err)
	pm := resp.GetPaymentMethods()
	require.NotNil(t, pm)
	require.True(t, pm.GetCashEnabled())
	require.False(t, pm.GetQrCardEnabled())
	require.Equal(t, "cash_only", pm.GetPaymentMode())
	require.Equal(t, "provider_unavailable", pm.GetQrCardUnavailableReason())
	require.Equal(t, "unavailable", pm.GetCardQrProviderStatus())
}
