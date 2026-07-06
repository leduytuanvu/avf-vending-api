package grpcserver

import (
	"context"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/activation"
	"github.com/avf/avf-vending-api/internal/app/api"
	appaudit "github.com/avf/avf-vending-api/internal/app/audit"
	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/app/machineruntime"
	"github.com/avf/avf-vending-api/internal/app/salecatalog"
	"github.com/avf/avf-vending-api/internal/app/workfloworch"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/id"
	platformpayments "github.com/avf/avf-vending-api/internal/platform/payments"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
)

type machineCodeGRPCTestEnv struct {
	pool      *pgxpool.Pool
	srv       *Server
	conn      *grpc.ClientConn
	cfg       *config.Config
	pepper    []byte
	machineID uuid.UUID
	siteID    uuid.UUID
	humanCode string
	actCode   activation.CreateResult
}

func setupMachineCodeGRPCTest(t *testing.T, humanCode string) *machineCodeGRPCTestEnv {
	t.Helper()

	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	siteID := id.NewUUIDV7()
	machineID := id.NewUUIDV7()

	_, err := pool.Exec(ctx, `INSERT INTO sites (id, name, code, status) VALUES ($1, 's', '', 'active')`, siteID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, site_id, serial_number, code, status, credential_version)
VALUES ($1, $2, $3, $4, 'online', 0)`, machineID, siteID, "sn-grpc-mcode-"+uuid.NewString()[:8], humanCode)
	require.NoError(t, err)

	cfg := testMachineGRPCConfig()
	issuer, err := plauth.NewSessionIssuerFromHTTPAuth(cfg.HTTPAuth)
	require.NoError(t, err)
	pepper := plauth.TrimSecret(cfg.HTTPAuth.JWTSecret)
	act := activation.NewService(pool, issuer, pepper, nil)
	rt, err := machineruntime.NewService(machineruntime.Deps{Pool: pool})
	require.NoError(t, err)
	act.SetMachineRuntime(rt)
	act.SetEMQXClient(mockEMQXClient(t))

	create, err := act.CreateCode(ctx, activation.CreateInput{
		MachineID:        machineID,
		ExpiresInMinutes: 60,
		MaxUses:          1,
	})
	require.NoError(t, err)

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
		MQTTBrokerURL:   "tcp://mqtt.example.invalid:1883",
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

	return &machineCodeGRPCTestEnv{
		pool:      pool,
		srv:       srv,
		conn:      conn,
		cfg:       cfg,
		pepper:    pepper,
		machineID: machineID,
		siteID:    siteID,
		humanCode: humanCode,
		actCode:   create,
	}
}

func (e *machineCodeGRPCTestEnv) claim(t *testing.T) *machinev1.ClaimActivationResponse {
	t.Helper()
	client := machinev1.NewMachineActivationServiceClient(e.conn)
	resp, err := client.ClaimActivation(context.Background(), &machinev1.ClaimActivationRequest{
		ActivationCode: e.actCode.PlaintextCode,
		DeviceFingerprint: &machinev1.DeviceFingerprint{
			AndroidId:    "mcode-aid",
			BoardSerial:  "mcode-board",
			SerialNumber: "mcode-serial",
			PackageName:  "com.avf.vending",
			VersionName:  "1.0.0",
			VersionCode:  100,
		},
	})
	require.NoError(t, err)
	return resp
}

func TestMachineGRPC_MachineAuthClaimActivationAlias_ReturnsMachineCode(t *testing.T) {
	t.Parallel()

	env := setupMachineCodeGRPCTest(t, "AVF000002")
	authClient := machinev1.NewMachineAuthServiceClient(env.conn)
	resp, err := authClient.ClaimActivation(context.Background(), &machinev1.MachineAuthServiceClaimActivationRequest{
		Claim: &machinev1.ClaimActivationRequest{
			ActivationCode: env.actCode.PlaintextCode,
			DeviceFingerprint: &machinev1.DeviceFingerprint{
				AndroidId:   "auth-alias-aid",
				BoardSerial: "auth-alias-board",
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, env.machineID.String(), resp.GetClaim().GetMachineId())
	require.Equal(t, "AVF000002", resp.GetClaim().GetMachineCode())
}

func TestMachineGRPC_RefreshMachineToken_ReturnsMachineCode(t *testing.T) {
	t.Parallel()

	env := setupMachineCodeGRPCTest(t, "AVF000003")
	claim := env.claim(t)
	require.Equal(t, "AVF000003", claim.GetMachineCode())

	tokClient := machinev1.NewMachineTokenServiceClient(env.conn)
	refreshResp, err := tokClient.RefreshMachineToken(context.Background(), &machinev1.RefreshMachineTokenRequest{
		RefreshToken: claim.GetRefreshToken(),
	})
	require.NoError(t, err)
	require.Equal(t, env.machineID.String(), refreshResp.GetMachineId())
	require.Equal(t, "AVF000003", refreshResp.GetMachineCode())
	require.NotEmpty(t, refreshResp.GetAccessToken())
	require.NotEmpty(t, refreshResp.GetRefreshToken())

	authClient := machinev1.NewMachineAuthServiceClient(env.conn)
	authRefresh, err := authClient.RefreshMachineToken(context.Background(), &machinev1.MachineAuthServiceRefreshMachineTokenRequest{
		Refresh: &machinev1.RefreshMachineTokenRequest{RefreshToken: claim.GetRefreshToken()},
	})
	require.NoError(t, err)
	require.Equal(t, "AVF000003", authRefresh.GetRefresh().GetMachineCode())
	require.Equal(t, env.machineID.String(), authRefresh.GetRefresh().GetMachineId())
}

func TestMachineGRPC_GetBootstrap_ReturnsMachineCode(t *testing.T) {
	t.Parallel()

	env := setupMachineCodeGRPCTest(t, "AVF000004")
	claim := env.claim(t)
	md := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+claim.GetAccessToken())
	bootstrapClient := machinev1.NewMachineBootstrapServiceClient(env.conn)
	resp, err := bootstrapClient.GetBootstrap(md, &machinev1.GetBootstrapRequest{})
	require.NoError(t, err)
	require.Equal(t, env.machineID.String(), resp.GetMachine().GetMachineId())
	require.Equal(t, "AVF000004", resp.GetMachine().GetMachineCode())
}

func TestMachineGRPC_ClaimActivation_JWTMachineIDRemainsUUID(t *testing.T) {
	t.Parallel()

	env := setupMachineCodeGRPCTest(t, "AVF000005")
	resp := env.claim(t)
	require.Equal(t, "AVF000005", resp.GetMachineCode())
	require.Equal(t, env.machineID.String(), resp.GetMqttUsername())

	claims, err := plauth.ValidateMachineAccessJWT(resp.GetAccessToken(), [][]byte{env.pepper}, 0, "")
	require.NoError(t, err)
	require.Equal(t, env.machineID, claims.MachineID)
}
