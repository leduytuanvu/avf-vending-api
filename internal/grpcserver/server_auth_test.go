package grpcserver

import (
	"bytes"
	"context"
	"testing"
	"time"

	appapi "github.com/avf/avf-vending-api/internal/app/api"
	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/config"
	internalv1 "github.com/avf/avf-vending-api/internal/gen/avfinternalv1"
	platformauth "github.com/avf/avf-vending-api/internal/platform/auth"
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
	client := internalv1.NewInternalMachineQueryServiceClient(conn)

	_, err := client.GetMachineSummary(context.Background(), &internalv1.GetMachineSummaryRequest{MachineId: uuid.NewString()})
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

	token := issueTestInternalServiceToken(t, cfg.HTTPAuth.JWTSecret, tokenOrgID)
	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)

	client := internalv1.NewInternalCommerceQueryServiceClient(conn)
	_, err := client.GetOrderPaymentVendState(ctx, &internalv1.GetOrderPaymentVendStateRequest{
		OrganizationId: requestOrgID.String(),
		OrderId:        orderID.String(),
		SlotIndex:      1,
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("code=%v err=%v", status.Code(err), err)
	}
}

func TestInternalQueryServices_RejectUserAccessJWT(t *testing.T) {
	t.Parallel()

	cfg, conn := startInternalQueryTestServer(t)
	orgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	machineID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	token := issueTestAccessToken(t, cfg, orgID, []string{platformauth.RoleOrgAdmin})
	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)

	client := internalv1.NewInternalMachineQueryServiceClient(conn)
	_, err := client.GetMachineSummary(ctx, &internalv1.GetMachineSummaryRequest{MachineId: machineID.String()})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("code=%v err=%v", status.Code(err), err)
	}
}

func TestInternalQueryServices_RejectMachineAccessJWT(t *testing.T) {
	t.Parallel()

	cfg, conn := startInternalQueryTestServer(t)
	orgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	siteID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	machineID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	issuer, err := platformauth.NewSessionIssuerFromHTTPAuth(cfg.HTTPAuth)
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := issuer.IssueMachineAccessJWT(machineID, orgID, siteID, 1, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)

	client := internalv1.NewInternalMachineQueryServiceClient(conn)
	_, err = client.GetMachineSummary(ctx, &internalv1.GetMachineSummaryRequest{MachineId: machineID.String()})
	if status.Code(err) != codes.Unauthenticated {
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
		InternalGRPC: config.InternalGRPCConfig{
			Enabled:             true,
			Addr:                "127.0.0.1:0",
			ShutdownTimeout:     3 * time.Second,
			HealthEnabled:       true,
			ReflectionEnabled:   false,
			UnaryHandlerTimeout: 30 * time.Second,
			ServiceTokenSecret:  nil, // dev fallback to HTTPAuth.JWTSecret
		},
	}

	orgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	services := InternalQueryServices{
		Machine:   stubMachineQueries{},
		Telemetry: stubTelemetryQueries{},
		Commerce:  stubCommerceQueries{out: appcommerce.CheckoutStatusView{}},
		Payment:   stubPaymentQueries{},
		Catalog:   stubInternalQuerySaleCatalog{orgID: orgID},
		Inventory: appapi.NewInternalInventoryQueryService(stubMachineQueries{}),
		Reporting: stubReportingForInternal{},
	}
	srv, err := NewInternalGRPCServer(cfg, zap.NewNop(), nil, RegisterInternalQueryServices(services))
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

func issueTestInternalServiceToken(t *testing.T, secret []byte, orgID uuid.UUID) string {
	t.Helper()
	token, _, err := platformauth.IssueInternalServiceAccessJWT(secret, "grpcserver-test", orgID, time.Minute, "test")
	if err != nil {
		t.Fatal(err)
	}
	return token
}
