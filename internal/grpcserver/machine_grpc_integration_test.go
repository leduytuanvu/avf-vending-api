package grpcserver

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/activation"
	"github.com/avf/avf-vending-api/internal/app/api"
	appaudit "github.com/avf/avf-vending-api/internal/app/audit"
	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/app/salecatalog"
	"github.com/avf/avf-vending-api/internal/app/workfloworch"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	platformpayments "github.com/avf/avf-vending-api/internal/platform/payments"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
)

func testMachineGRPCIntegrationDSN(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration tests in -short mode")
	}
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	return dsn
}

func machineGRPCTestModuleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func machineGRPCTestMigrate(t *testing.T, dsn string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	goBin := os.Getenv("GO_BIN")
	if goBin == "" {
		goBin = "go"
	}
	cmd := exec.CommandContext(ctx, goBin, "run", "github.com/pressly/goose/v3/cmd/goose@v3.27.0",
		"-dir", filepath.Join(machineGRPCTestModuleRoot(t), "migrations"),
		"postgres", dsn, "up",
	)
	cmd.Dir = machineGRPCTestModuleRoot(t)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", string(out))
}

func machineGRPCTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := testMachineGRPCIntegrationDSN(t)
	machineGRPCTestMigrate(t, dsn)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func TestMachineGRPC_GetBootstrap_RetiredMachineRejected(t *testing.T) {
	t.Parallel()

	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()

	_, err := pool.Exec(ctx, `INSERT INTO organizations (id, name, slug, status) VALUES ($1, 'grpc-int', 'grpc-int-org', 'active')`, orgID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO sites (id, organization_id, name, code, status) VALUES ($1, $2, 's', '', 'active')`, siteID, orgID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, organization_id, site_id, serial_number, status, credential_version)
VALUES ($1, $2, $3, $4, 'retired', 1)`, machineID, orgID, siteID, "sn-retired-grpc")
	require.NoError(t, err)

	cfg := testMachineGRPCConfig()
	issuer, err := plauth.NewSessionIssuerFromHTTPAuth(cfg.HTTPAuth)
	require.NoError(t, err)
	pepper := plauth.TrimSecret(cfg.HTTPAuth.JWTSecret)
	act := activation.NewService(pool, issuer, pepper, nil)
	store := postgres.NewStore(pool)
	auditSvc := appaudit.NewService(pool)
	commerceSvc := appcommerce.NewService(appcommerce.Deps{
		OrderVend:              store,
		PaymentOutbox:          store,
		Lifecycle:              store,
		WebhookPersist:         store,
		SaleLines:              store,
		WorkflowOrchestration:  workfloworch.NewDisabled(),
		EnterpriseAudit:        auditSvc,
		PaymentSessionRegistry: platformpayments.NewRegistry(cfg),
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

	tok, _, err := issuer.IssueMachineAccessJWT(machineID, orgID, siteID, 1, uuid.Nil)
	require.NoError(t, err)
	mdCtx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+tok)

	client := machinev1.NewMachineBootstrapServiceClient(conn)
	_, err = client.GetBootstrap(mdCtx, &machinev1.GetBootstrapRequest{})
	require.Equal(t, codes.PermissionDenied, status.Code(err), "got %v", err)
}

func TestMachineGRPC_GetInventorySnapshot_MaintenanceMachineRejected(t *testing.T) {
	t.Parallel()

	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()

	_, err := pool.Exec(ctx, `INSERT INTO organizations (id, name, slug, status) VALUES ($1, 'grpc-inv', 'grpc-inv-org', 'active')`, orgID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO sites (id, organization_id, name, code, status) VALUES ($1, $2, 's', '', 'active')`, siteID, orgID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, organization_id, site_id, serial_number, status, credential_version)
VALUES ($1, $2, $3, $4, 'maintenance', 1)`, machineID, orgID, siteID, "sn-maint-grpc")
	require.NoError(t, err)

	cfg := testMachineGRPCConfig()
	issuer, err := plauth.NewSessionIssuerFromHTTPAuth(cfg.HTTPAuth)
	require.NoError(t, err)
	pepper := plauth.TrimSecret(cfg.HTTPAuth.JWTSecret)
	act := activation.NewService(pool, issuer, pepper, nil)
	store := postgres.NewStore(pool)
	auditSvc := appaudit.NewService(pool)
	commerceSvc := appcommerce.NewService(appcommerce.Deps{
		OrderVend:              store,
		PaymentOutbox:          store,
		Lifecycle:              store,
		WebhookPersist:         store,
		SaleLines:              store,
		WorkflowOrchestration:  workfloworch.NewDisabled(),
		EnterpriseAudit:        auditSvc,
		PaymentSessionRegistry: platformpayments.NewRegistry(cfg),
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

	tok, _, err := issuer.IssueMachineAccessJWT(machineID, orgID, siteID, 1, uuid.Nil)
	require.NoError(t, err)
	mdCtx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+tok)

	inv := machinev1.NewMachineInventoryServiceClient(conn)
	_, err = inv.GetInventorySnapshot(mdCtx, &machinev1.GetInventorySnapshotRequest{})
	require.Equal(t, codes.PermissionDenied, status.Code(err), "got %v", err)
}
