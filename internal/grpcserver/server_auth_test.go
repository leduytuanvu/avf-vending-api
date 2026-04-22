package grpcserver

import (
	"bytes"
	"context"
	"testing"
	"time"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/config"
	platformauth "github.com/avf/avf-vending-api/internal/platform/auth"
	avfv1 "github.com/avf/avf-vending-api/proto/avf/v1"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestInternalQueryServices_RejectMissingBearerToken(t *testing.T) {
	t.Parallel()

	_, conn := startInternalQueryTestServer(t)
	client := avfv1.NewInternalMachineQueryServiceClient(conn)

	_, err := client.GetMachineSummary(context.Background(), &avfv1.GetMachineRequest{MachineId: uuid.NewString()})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("code=%v err=%v", status.Code(err), err)
	}
}

func TestInternalQueryServices_RejectOrganizationScopeMismatch(t *testing.T) {
	t.Parallel()

	cfg, conn := startInternalQueryTestServer(t)
	requestOrgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	tokenOrgID := uuid.MustParse("aaaaaaaa-1111-1111-1111-111111111111")
	orderID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	token := issueTestAccessToken(t, cfg, tokenOrgID, []string{platformauth.RoleOrgAdmin})
	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)

	client := avfv1.NewInternalCommerceQueryServiceClient(conn)
	_, err := client.GetOrderPaymentVendState(ctx, &avfv1.GetOrderPaymentVendStateRequest{
		OrganizationId: requestOrgID.String(),
		OrderId:        orderID.String(),
		SlotIndex:      1,
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("code=%v err=%v", status.Code(err), err)
	}
}

func startInternalQueryTestServer(t *testing.T) (*config.Config, *grpc.ClientConn) {
	t.Helper()

	cfg := &config.Config{
		AppEnv:    config.AppEnvDevelopment,
		LogLevel:  "info",
		LogFormat: "json",
		HTTPAuth: config.HTTPAuthConfig{
			Mode:            "hs256",
			JWTSecret:       bytes.Repeat([]byte("x"), 32),
			JWTLeeway:       45 * time.Second,
			AccessTokenTTL:  15 * time.Minute,
			RefreshTokenTTL: 720 * time.Hour,
		},
		GRPC: config.GRPCConfig{
			Enabled:         true,
			Addr:            "127.0.0.1:0",
			ShutdownTimeout: 3 * time.Second,
		},
	}

	services := InternalQueryServices{
		Machine:   stubMachineQueries{},
		Telemetry: stubTelemetryQueries{},
		Commerce:  stubCommerceQueries{out: appcommerce.CheckoutStatusView{}},
	}
	srv, err := NewServer(cfg, zap.NewNop(), RegisterInternalQueryServices(services))
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		<-errCh
	})

	connCtx, connCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer connCancel()
	conn, err := grpc.DialContext(connCtx, srv.ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
	})

	return cfg, conn
}
