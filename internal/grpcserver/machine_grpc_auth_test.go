package grpcserver

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func testMachineGRPCConfig() *config.Config {
	return &config.Config{
		AppEnv:    config.AppEnvDevelopment,
		LogLevel:  "info",
		LogFormat: "json",
		HTTPAuth: config.HTTPAuthConfig{
			Mode:            "hs256",
			JWTSecret:       bytes.Repeat([]byte("y"), 32),
			JWTLeeway:       45 * time.Second,
			AccessTokenTTL:  15 * time.Minute,
			RefreshTokenTTL: 720 * time.Hour,
		},
		GRPC: config.GRPCConfig{
			Enabled:                true,
			Addr:                   "127.0.0.1:0",
			ShutdownTimeout:        3 * time.Second,
			HealthEnabled:          true,
			ReflectionEnabled:      false,
			RequireMachineJWT:      true,
			RequireGRPCIdempotency: true,
			UnaryHandlerTimeout:    30 * time.Second,
		},
		Commerce: config.CommerceHTTPConfig{
			PaymentOutboxTopic:          "commerce.payments",
			PaymentOutboxEventType:      "payment.session_started",
			PaymentOutboxAggregateType:  "payment",
			PaymentWebhookTimestampSkew: 120 * time.Second,
			MachineOrderCheckoutMaxAge:  30 * time.Minute,
			DefaultPaymentProvider:      "psp_grpc_int",
		},
	}
}

type recordingBootstrapServer struct {
	machinev1.UnimplementedMachineBootstrapServiceServer
	claimsOK bool
}

func (r *recordingBootstrapServer) GetBootstrap(ctx context.Context, _ *machinev1.GetBootstrapRequest) (*machinev1.GetBootstrapResponse, error) {
	_, r.claimsOK = plauth.MachineAccessClaimsFromContext(ctx)
	return &machinev1.GetBootstrapResponse{}, nil
}

type stubMachineTokenChecker struct {
	err error
}

func (s stubMachineTokenChecker) ValidateMachineAccessClaims(context.Context, plauth.MachineAccessClaims) error {
	return s.err
}

func TestMachineGRPC_NamespaceDefaultsToMachineJWT(t *testing.T) {
	t.Parallel()

	info := &grpc.UnaryServerInfo{FullMethod: "/avf.machine.v1.FutureService/FutureMethod"}
	called := false
	handler := func(context.Context, any) (any, error) {
		called = true
		return "ok", nil
	}

	cfg := testMachineGRPCConfig()
	interceptor := unaryAuthChainInterceptor(cfg, zap.NewNop(), nil, nil, nil, nil)

	_, err := interceptor(context.Background(), nil, info, handler)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v: %v", status.Code(err), err)
	}
	if called {
		t.Fatal("future machine RPC should not fall through to user auth")
	}
}

func TestMachineGRPC_P01ServicesRegistered(t *testing.T) {
	t.Parallel()

	s := grpc.NewServer()
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

	info := s.GetServiceInfo()
	want := []string{
		"avf.machine.v1.MachineAuthService",
		"avf.machine.v1.MachineActivationService",
		"avf.machine.v1.MachineTokenService",
		"avf.machine.v1.MachineBootstrapService",
		"avf.machine.v1.MachineCatalogService",
		"avf.machine.v1.MachineMediaService",
		"avf.machine.v1.MachineCommerceService",
		"avf.machine.v1.MachineSaleService",
		"avf.machine.v1.MachineInventoryService",
		"avf.machine.v1.MachineTelemetryService",
		"avf.machine.v1.MachineOperatorService",
		"avf.machine.v1.MachineOfflineSyncService",
		"avf.machine.v1.MachineCommandService",
	}
	for _, name := range want {
		if _, ok := info[name]; !ok {
			t.Fatalf("expected service %s to be registered", name)
		}
	}
}

func TestMachineGRPC_GetBootstrap_MissingBearerRejected(t *testing.T) {
	t.Parallel()

	cfg := testMachineGRPCConfig()
	rec := &recordingBootstrapServer{}
	srv, err := NewServer(cfg, zap.NewNop(), nil, nil, nil, nil, nil, nil, func(s *grpc.Server) error {
		machinev1.RegisterMachineBootstrapServiceServer(s, rec)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe(ctx) }()
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
	t.Cleanup(func() { _ = conn.Close() })

	client := machinev1.NewMachineBootstrapServiceClient(conn)
	_, err = client.GetBootstrap(context.Background(), &machinev1.GetBootstrapRequest{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v: %v", status.Code(err), err)
	}
}

// TestP06_MachineGRPC_AdminUserJWTRejected documents the RBAC boundary: admin user JWTs must not authenticate machine gRPC.
func TestP06_MachineGRPC_AdminUserJWTRejected(t *testing.T) {
	t.Parallel()
	testMachineGRPCAdminUserJWTRejected(t)
}

func testMachineGRPCAdminUserJWTRejected(t *testing.T) {
	t.Helper()
	cfg := testMachineGRPCConfig()
	rec := &recordingBootstrapServer{}
	srv, err := NewServer(cfg, zap.NewNop(), nil, nil, nil, nil, nil, nil, func(s *grpc.Server) error {
		machinev1.RegisterMachineBootstrapServiceServer(s, rec)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe(ctx) }()
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
	t.Cleanup(func() { _ = conn.Close() })

	issuer, err := plauth.NewSessionIssuerFromHTTPAuth(cfg.HTTPAuth)
	if err != nil {
		t.Fatal(err)
	}
	orgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	userTok, _, err := issuer.IssueAccessJWT(uuid.New(), orgID, []string{plauth.RoleOrgAdmin}, "active")
	if err != nil {
		t.Fatal(err)
	}

	mdCtx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+userTok)
	client := machinev1.NewMachineBootstrapServiceClient(conn)
	_, err = client.GetBootstrap(mdCtx, &machinev1.GetBootstrapRequest{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v: %v", status.Code(err), err)
	}
}

func TestMachineGRPC_GetBootstrap_UserJWTRejected(t *testing.T) {
	t.Parallel()
	testMachineGRPCAdminUserJWTRejected(t)
}

func TestMachineGRPC_GetBootstrap_MachineJWTAccepted(t *testing.T) {
	t.Parallel()

	cfg := testMachineGRPCConfig()
	rec := &recordingBootstrapServer{}
	srv, err := NewServer(cfg, zap.NewNop(), nil, nil, nil, nil, nil, nil, func(s *grpc.Server) error {
		machinev1.RegisterMachineBootstrapServiceServer(s, rec)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe(ctx) }()
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
	t.Cleanup(func() { _ = conn.Close() })

	issuer, err := plauth.NewSessionIssuerFromHTTPAuth(cfg.HTTPAuth)
	if err != nil {
		t.Fatal(err)
	}
	machineID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	orgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	siteID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	mTok, _, err := issuer.IssueMachineAccessJWT(machineID, orgID, siteID, 1, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}

	mdCtx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+mTok)
	client := machinev1.NewMachineBootstrapServiceClient(conn)
	_, err = client.GetBootstrap(mdCtx, &machinev1.GetBootstrapRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !rec.claimsOK {
		t.Fatal("expected machine claims in handler context")
	}
}

func TestMachineGRPC_RequestMachineScopeMismatchRejected(t *testing.T) {
	t.Parallel()

	cfg := testMachineGRPCConfig()
	rec := &recordingBootstrapServer{}
	srv, err := NewServer(cfg, zap.NewNop(), nil, nil, nil, nil, nil, nil, func(s *grpc.Server) error {
		machinev1.RegisterMachineBootstrapServiceServer(s, rec)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe(ctx) }()
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
	t.Cleanup(func() { _ = conn.Close() })

	issuer, err := plauth.NewSessionIssuerFromHTTPAuth(cfg.HTTPAuth)
	if err != nil {
		t.Fatal(err)
	}
	machineID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	otherMachineID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	orgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	siteID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	mTok, _, err := issuer.IssueMachineAccessJWT(machineID, orgID, siteID, 1, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}

	mdCtx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+mTok)
	client := machinev1.NewMachineBootstrapServiceClient(conn)
	_, err = client.GetBootstrap(mdCtx, &machinev1.GetBootstrapRequest{
		Meta: &machinev1.MachineRequestMeta{MachineId: otherMachineID.String(), OrganizationId: orgID.String()},
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v: %v", status.Code(err), err)
	}
	if rec.claimsOK {
		t.Fatal("handler should not receive scope-mismatched request")
	}
}

func TestMachineGRPC_WrongAudienceRejected(t *testing.T) {
	t.Parallel()

	cfg := testMachineGRPCConfig()
	info := &grpc.UnaryServerInfo{FullMethod: machinev1.MachineBootstrapService_GetBootstrap_FullMethodName}
	token := signMachineGRPCTestJWT(t, cfg, "wrong-audience", time.Now().UTC().Add(10*time.Minute))
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token))
	interceptor := unaryAuthChainInterceptor(cfg, zap.NewNop(), nil, nil, nil, nil)
	_, err := interceptor(ctx, &machinev1.GetBootstrapRequest{}, info, func(context.Context, any) (any, error) {
		return &machinev1.GetBootstrapResponse{}, nil
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v: %v", status.Code(err), err)
	}
}

func TestMachineGRPC_ExpiredMachineTokenRejected(t *testing.T) {
	t.Parallel()

	cfg := testMachineGRPCConfig()
	info := &grpc.UnaryServerInfo{FullMethod: machinev1.MachineBootstrapService_GetBootstrap_FullMethodName}
	token := signMachineGRPCTestJWT(t, cfg, plauth.AudienceMachineGRPC, time.Now().UTC().Add(-2*time.Hour))
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token))
	interceptor := unaryAuthChainInterceptor(cfg, zap.NewNop(), nil, nil, nil, nil)
	_, err := interceptor(ctx, &machinev1.GetBootstrapRequest{}, info, func(context.Context, any) (any, error) {
		return &machinev1.GetBootstrapResponse{}, nil
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v: %v", status.Code(err), err)
	}
}

func TestMachineGRPC_RevokedMachineRejected(t *testing.T) {
	t.Parallel()

	cfg := testMachineGRPCConfig()
	info := &grpc.UnaryServerInfo{FullMethod: machinev1.MachineBootstrapService_GetBootstrap_FullMethodName}
	issuer, err := plauth.NewSessionIssuerFromHTTPAuth(cfg.HTTPAuth)
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := issuer.IssueMachineAccessJWT(
		uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		1,
		uuid.Nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token))
	interceptor := unaryAuthChainInterceptor(cfg, zap.NewNop(), nil, nil, stubMachineTokenChecker{
		err: status.Error(codes.Unauthenticated, "machine credentials revoked"),
	}, nil)
	_, err = interceptor(ctx, &machinev1.GetBootstrapRequest{}, info, func(context.Context, any) (any, error) {
		return &machinev1.GetBootstrapResponse{}, nil
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v: %v", status.Code(err), err)
	}
}

func signMachineGRPCTestJWT(t *testing.T, cfg *config.Config, audience string, expiresAt time.Time) string {
	t.Helper()
	machineID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	orgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	now := time.Now().UTC()
	raw, err := plauth.SignHS256JWT(cfg.HTTPAuth.JWTSecret, map[string]any{
		"sub":             "machine:" + machineID.String(),
		"typ":             plauth.JWTClaimTypeMachine,
		"iss":             "",
		"aud":             audience,
		"roles":           []string{plauth.RoleMachine},
		"organization_id": orgID.String(),
		"machine_id":      machineID.String(),
		"token_version":   1,
		"scopes":          plauth.DefaultMachineAccessScopes,
		"iat":             now.Unix(),
		"nbf":             now.Unix(),
		"exp":             expiresAt.Unix(),
		"token_use":       plauth.TokenUseMachineAccess,
		"jti":             uuid.NewString(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestMachineGRPC_ActivateMachine_NoBearer(t *testing.T) {
	t.Parallel()

	cfg := testMachineGRPCConfig()
	srv, err := NewServer(cfg, zap.NewNop(), nil, nil, nil, nil, nil, nil, func(s *grpc.Server) error {
		machinev1.RegisterMachineAuthServiceServer(s, &machinev1.UnimplementedMachineAuthServiceServer{})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe(ctx) }()
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
	t.Cleanup(func() { _ = conn.Close() })

	// Unimplemented server returns Unimplemented; auth must allow only ActivateMachine through.
	client := machinev1.NewMachineAuthServiceClient(conn)
	_, err = client.ActivateMachine(context.Background(), &machinev1.ActivateMachineRequest{})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("expected Unimplemented from stub, got %v: %v", status.Code(err), err)
	}
}

func TestMachineGRPC_ClaimActivationAndRefresh_NoBearerAllowed(t *testing.T) {
	t.Parallel()

	cfg := testMachineGRPCConfig()
	srv, err := NewServer(cfg, zap.NewNop(), nil, nil, nil, nil, nil, nil, func(s *grpc.Server) error {
		machinev1.RegisterMachineAuthServiceServer(s, &machinev1.UnimplementedMachineAuthServiceServer{})
		machinev1.RegisterMachineTokenServiceServer(s, &machinev1.UnimplementedMachineTokenServiceServer{})
		machinev1.RegisterMachineActivationServiceServer(s, &machinev1.UnimplementedMachineActivationServiceServer{})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe(ctx) }()
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
	t.Cleanup(func() { _ = conn.Close() })

	client := machinev1.NewMachineAuthServiceClient(conn)
	_, err = client.RefreshMachineToken(context.Background(), &machinev1.MachineAuthServiceRefreshMachineTokenRequest{})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("expected RefreshMachineToken Unimplemented from stub without bearer, got %v: %v", status.Code(err), err)
	}

	_, err = client.ClaimActivation(context.Background(), &machinev1.MachineAuthServiceClaimActivationRequest{})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("expected ClaimActivation Unimplemented from stub without bearer, got %v: %v", status.Code(err), err)
	}

	tokClient := machinev1.NewMachineTokenServiceClient(conn)
	_, err = tokClient.RefreshMachineToken(context.Background(), &machinev1.RefreshMachineTokenRequest{})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("expected MachineTokenService.RefreshMachineToken Unimplemented from stub without bearer, got %v: %v", status.Code(err), err)
	}

	actClient := machinev1.NewMachineActivationServiceClient(conn)
	_, err = actClient.ClaimActivation(context.Background(), &machinev1.ClaimActivationRequest{})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("expected MachineActivationService.ClaimActivation Unimplemented from stub without bearer, got %v: %v", status.Code(err), err)
	}
}
