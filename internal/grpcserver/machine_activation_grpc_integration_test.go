package grpcserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/activation"
	"github.com/avf/avf-vending-api/internal/app/api"
	appaudit "github.com/avf/avf-vending-api/internal/app/audit"
	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/app/machineruntime"
	"github.com/avf/avf-vending-api/internal/app/salecatalog"
	"github.com/avf/avf-vending-api/internal/app/workfloworch"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/emqxadmin"
	"github.com/avf/avf-vending-api/internal/platform/id"
	platformpayments "github.com/avf/avf-vending-api/internal/platform/payments"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
)

func mockEMQXClient(t *testing.T) *emqxadmin.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut:
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	t.Cleanup(srv.Close)
	client, err := emqxadmin.NewClient(srv.URL, "test-key", "test-secret")
	require.NoError(t, err)
	client.HTTPClient = srv.Client()
	return client
}

func TestMachineGRPC_ClaimActivation_ReturnsDeviceAttachmentAndMqttCredentials(t *testing.T) {
	t.Parallel()

	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	siteID := id.NewUUIDV7()
	machineID := id.NewUUIDV7()

	_, err := pool.Exec(ctx, `INSERT INTO sites (id, name, code, status) VALUES ($1, 's', '', 'active')`, siteID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, site_id, serial_number, status, credential_version)
VALUES ($1, $2, $3, 'online', 0)`, machineID, siteID, "sn-grpc-act-"+uuid.NewString()[:8])
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

	client := machinev1.NewMachineActivationServiceClient(conn)
	resp, err := client.ClaimActivation(ctx, &machinev1.ClaimActivationRequest{
		ActivationCode: create.PlaintextCode,
		DeviceFingerprint: &machinev1.DeviceFingerprint{
			AndroidId:    "grpc-aid",
			BoardSerial:  "grpc-board",
			SerialNumber: "grpc-serial",
			PackageName:  "com.avf.vending",
			VersionName:  "1.0.0",
			VersionCode:  100,
			SimIccid:     "grpc-iccid",
			AppBuildSha:  "grpc-sha",
			BootId:       "grpc-boot",
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.GetDeviceAttachmentId())
	_, parseErr := uuid.Parse(resp.GetDeviceAttachmentId())
	require.NoError(t, parseErr)
	require.Equal(t, machineID.String(), resp.GetMachineId())
	require.NotEmpty(t, resp.GetAccessToken())
	require.NotEmpty(t, resp.GetMqttUsername())
	require.NotEmpty(t, resp.GetMqttPassword())
	require.Equal(t, machineID.String(), resp.GetMqttUsername())
}

func TestMachineGRPC_ActivateMachineAlias_ReturnsDeviceAttachmentId(t *testing.T) {
	t.Parallel()

	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	siteID := id.NewUUIDV7()
	machineID := id.NewUUIDV7()

	_, err := pool.Exec(ctx, `INSERT INTO sites (id, name, code, status) VALUES ($1, 's', '', 'active')`, siteID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, site_id, serial_number, status, credential_version)
VALUES ($1, $2, $3, 'online', 0)`, machineID, siteID, "sn-grpc-alias-"+uuid.NewString()[:8])
	require.NoError(t, err)

	cfg := testMachineGRPCConfig()
	issuer, err := plauth.NewSessionIssuerFromHTTPAuth(cfg.HTTPAuth)
	require.NoError(t, err)
	pepper := plauth.TrimSecret(cfg.HTTPAuth.JWTSecret)
	act := activation.NewService(pool, issuer, pepper, nil)
	rt, err := machineruntime.NewService(machineruntime.Deps{Pool: pool})
	require.NoError(t, err)
	act.SetMachineRuntime(rt)

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

	conn := dialMachineCommerceServer(t, srv)
	authClient := machinev1.NewMachineAuthServiceClient(conn)
	aliasResp, err := authClient.ActivateMachine(ctx, &machinev1.ActivateMachineRequest{
		Claim: &machinev1.ClaimActivationRequest{
			ActivationCode: create.PlaintextCode,
			DeviceFingerprint: &machinev1.DeviceFingerprint{
				AndroidId:   "alias-aid",
				BoardSerial: "alias-board",
			},
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, aliasResp.GetClaim().GetDeviceAttachmentId())
}
