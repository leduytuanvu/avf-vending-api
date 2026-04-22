package grpcserver

import (
	"context"
	"testing"
	"time"

	appapi "github.com/avf/avf-vending-api/internal/app/api"
	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/app/setupapp"
	"github.com/avf/avf-vending-api/internal/config"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	domainfleet "github.com/avf/avf-vending-api/internal/domain/fleet"
	platformauth "github.com/avf/avf-vending-api/internal/platform/auth"
	avfv1 "github.com/avf/avf-vending-api/proto/avf/v1"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
)

type stubMachineQueries struct {
	bootstrap setupapp.MachineBootstrap
	slotView  setupapp.MachineSlotView
	shadow    *appapi.ShadowView
}

func (s stubMachineQueries) GetMachineBootstrap(context.Context, uuid.UUID) (setupapp.MachineBootstrap, error) {
	return s.bootstrap, nil
}

func (s stubMachineQueries) GetMachineSlotView(context.Context, uuid.UUID) (setupapp.MachineSlotView, error) {
	return s.slotView, nil
}

func (s stubMachineQueries) GetShadow(context.Context, uuid.UUID) (*appapi.ShadowView, error) {
	return s.shadow, nil
}

type stubTelemetryQueries struct {
	snapshot  appapi.TelemetrySnapshotView
	incidents []appapi.MachineIncidentView
}

func (s stubTelemetryQueries) GetTelemetrySnapshot(context.Context, uuid.UUID) (appapi.TelemetrySnapshotView, error) {
	return s.snapshot, nil
}

func (s stubTelemetryQueries) ListMachineIncidentsRecent(context.Context, uuid.UUID, int32) ([]appapi.MachineIncidentView, error) {
	return s.incidents, nil
}

type stubCommerceQueries struct {
	out appcommerce.CheckoutStatusView
}

func (s stubCommerceQueries) GetCheckoutStatus(context.Context, uuid.UUID, uuid.UUID, int32) (appcommerce.CheckoutStatusView, error) {
	return s.out, nil
}

func TestNewServer_InternalQueryServices(t *testing.T) {
	t.Parallel()

	orgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	siteID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	machineID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	orderID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	vendID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	productID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	paymentID := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	incidentID := uuid.MustParse("88888888-8888-8888-8888-888888888888")

	cfg := &config.Config{
		AppEnv:    config.AppEnvDevelopment,
		LogLevel:  "info",
		LogFormat: "json",
		HTTPAuth: config.HTTPAuthConfig{
			Mode:            "hs256",
			JWTSecret:       []byte("test-secret-at-least-32-bytes-long-for-jwt!!"),
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
		Machine: stubMachineQueries{
			bootstrap: setupapp.MachineBootstrap{
				Machine: domainfleet.Machine{
					ID:              machineID,
					OrganizationID:  orgID,
					SiteID:          siteID,
					SerialNumber:    "SN-1",
					Name:            "Machine One",
					Status:          "active",
					CommandSequence: 42,
					CreatedAt:       time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
					UpdatedAt:       time.Date(2026, 1, 3, 3, 4, 5, 0, time.UTC),
				},
			},
			slotView: setupapp.MachineSlotView{
				LegacySlots: []setupapp.LegacySlotRow{{
					PlanogramID:       uuid.MustParse("99999999-9999-9999-9999-999999999999"),
					PlanogramName:     "Main",
					SlotIndex:         1,
					CurrentQuantity:   5,
					MaxQuantity:       10,
					PriceMinor:        250,
					ProductID:         &productID,
					ProductSKU:        "SKU-1",
					ProductName:       "Soda",
					PlanogramRevision: 2,
				}},
			},
			shadow: &appapi.ShadowView{
				MachineID: machineID,
				DesiredState: map[string]any{
					"desiredConfigVersion": 9,
				},
				ReportedState: map[string]any{
					"motor": "ok",
				},
				Version: 3,
			},
		},
		Telemetry: stubTelemetryQueries{
			snapshot: appapi.TelemetrySnapshotView{
				MachineID:         machineID,
				OrganizationID:    orgID,
				SiteID:            siteID,
				ReportedState:     []byte(`{"temperature":"cold"}`),
				MetricsState:      []byte(`{"power_watts":42.5}`),
				LastHeartbeatAt:   timePtr(time.Date(2026, 1, 4, 3, 4, 5, 0, time.UTC)),
				UpdatedAt:         time.Date(2026, 1, 4, 4, 4, 5, 0, time.UTC),
				EffectiveTimezone: "Asia/Bangkok",
			},
			incidents: []appapi.MachineIncidentView{{
				ID:        incidentID,
				MachineID: machineID,
				Severity:  "high",
				Code:      "door_open",
				Detail:    []byte(`{"state":"open"}`),
				OpenedAt:  time.Date(2026, 1, 4, 5, 4, 5, 0, time.UTC),
				UpdatedAt: time.Date(2026, 1, 4, 5, 5, 5, 0, time.UTC),
			}},
		},
		Commerce: stubCommerceQueries{
			out: appcommerce.CheckoutStatusView{
				Order: domaincommerce.Order{
					ID:             orderID,
					OrganizationID: orgID,
					MachineID:      machineID,
					Status:         "paid",
					Currency:       "THB",
					SubtotalMinor:  250,
					TaxMinor:       0,
					TotalMinor:     250,
					CreatedAt:      time.Date(2026, 1, 5, 1, 2, 3, 0, time.UTC),
					UpdatedAt:      time.Date(2026, 1, 5, 1, 3, 3, 0, time.UTC),
				},
				Vend: domaincommerce.VendSession{
					ID:        vendID,
					OrderID:   orderID,
					MachineID: machineID,
					SlotIndex: 1,
					ProductID: productID,
					State:     "in_progress",
					CreatedAt: time.Date(2026, 1, 5, 1, 4, 3, 0, time.UTC),
				},
				Payment: domaincommerce.Payment{
					ID:                   paymentID,
					OrderID:              orderID,
					Provider:             "cash",
					State:                "captured",
					AmountMinor:          250,
					Currency:             "THB",
					ReconciliationStatus: "matched",
					SettlementStatus:     "settled",
					CreatedAt:            time.Date(2026, 1, 5, 1, 2, 30, 0, time.UTC),
				},
				PaymentPresent: true,
			},
		},
	}

	srv, err := NewServer(cfg, zap.NewNop(), RegisterInternalQueryServices(services))
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
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
	defer conn.Close()

	healthClient := grpc_health_v1.NewHealthClient(conn)
	healthResp, err := healthClient.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if healthResp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		t.Fatalf("health status=%v", healthResp.Status)
	}

	token := issueTestAccessToken(t, cfg, orgID, []string{platformauth.RoleService})
	authCtx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token, "x-request-id", "grpc-test-1")

	machineClient := avfv1.NewInternalMachineQueryServiceClient(conn)
	machineResp, err := machineClient.GetMachineSummary(authCtx, &avfv1.GetMachineRequest{MachineId: machineID.String()})
	if err != nil {
		t.Fatal(err)
	}
	if got := machineResp.GetMachine().GetMachineId(); got != machineID.String() {
		t.Fatalf("machine id=%q", got)
	}

	telemetryClient := avfv1.NewInternalTelemetryQueryServiceClient(conn)
	telemetryResp, err := telemetryClient.GetLatestMachineTelemetry(authCtx, &avfv1.GetMachineRequest{MachineId: machineID.String()})
	if err != nil {
		t.Fatal(err)
	}
	if got := telemetryResp.GetSnapshot().GetEffectiveTimezone(); got != "Asia/Bangkok" {
		t.Fatalf("effective timezone=%q", got)
	}

	commerceClient := avfv1.NewInternalCommerceQueryServiceClient(conn)
	commerceResp, err := commerceClient.GetOrderPaymentVendState(authCtx, &avfv1.GetOrderPaymentVendStateRequest{
		OrganizationId: orgID.String(),
		OrderId:        orderID.String(),
		SlotIndex:      1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !commerceResp.GetPaymentPresent() {
		t.Fatal("expected payment present")
	}
	if got := commerceResp.GetPayment().GetId(); got != paymentID.String() {
		t.Fatalf("payment id=%q", got)
	}
}

func issueTestAccessToken(t *testing.T, cfg *config.Config, orgID uuid.UUID, roles []string) string {
	t.Helper()
	issuer, err := platformauth.NewSessionIssuerFromHTTPAuth(cfg.HTTPAuth)
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := issuer.IssueAccessJWT(uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), orgID, roles)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func timePtr(v time.Time) *time.Time {
	return &v
}
