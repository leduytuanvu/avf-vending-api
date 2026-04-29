package grpcserver

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	appapi "github.com/avf/avf-vending-api/internal/app/api"
	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/app/listscope"
	appreporting "github.com/avf/avf-vending-api/internal/app/reporting"
	appsalecatalog "github.com/avf/avf-vending-api/internal/app/salecatalog"
	"github.com/avf/avf-vending-api/internal/app/setupapp"
	"github.com/avf/avf-vending-api/internal/config"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	domainfleet "github.com/avf/avf-vending-api/internal/domain/fleet"
	internalv1 "github.com/avf/avf-vending-api/internal/gen/avfinternalv1"
	platformauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/ratelimit"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
)

func TestNewMachineHotRateLimitBackend_IndependentOfHTTPAbuseFlag(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		GRPC: config.GRPCConfig{Enabled: true},
		RedisRuntime: config.RedisRuntimeFeatures{
			GRPCMachineHotPerMinute: 12,
		},
	}
	cfg.HTTPRateLimit.Abuse.Enabled = false

	backend, err := newMachineHotRateLimitBackend(cfg, zap.NewNop(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := backend.(*ratelimit.MemoryBackend); !ok {
		t.Fatalf("expected memory fallback backend without Redis, got %T", backend)
	}
}

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

type stubPaymentQueries struct {
	payment domaincommerce.Payment
}

func (s stubPaymentQueries) GetPaymentByID(context.Context, uuid.UUID, uuid.UUID) (domaincommerce.Payment, error) {
	return s.payment, nil
}

func (s stubPaymentQueries) GetLatestPaymentForOrder(context.Context, uuid.UUID, uuid.UUID) (domaincommerce.Payment, error) {
	return s.payment, nil
}

type stubInternalQuerySaleCatalog struct {
	orgID uuid.UUID
}

func (s stubInternalQuerySaleCatalog) BuildSnapshot(ctx context.Context, machineID uuid.UUID, opts appsalecatalog.Options) (appsalecatalog.Snapshot, error) {
	_ = ctx
	_ = opts
	return appsalecatalog.Snapshot{
		MachineID:      machineID,
		OrganizationID: s.orgID,
		SiteID:         uuid.Nil,
		ConfigVersion:  1,
		CatalogVersion: "v1",
		Currency:       "THB",
		GeneratedAt:    time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC),
		Items:          nil,
	}, nil
}

type stubReportingForInternal struct{}

func (stubReportingForInternal) SalesSummary(_ context.Context, q listscope.ReportingQuery) (*appreporting.SalesSummaryResponse, error) {
	return &appreporting.SalesSummaryResponse{
		OrganizationID: q.OrganizationID.String(),
		From:           q.From.UTC().Format(time.RFC3339),
		To:             q.To.UTC().Format(time.RFC3339),
		GroupBy:        q.GroupBy,
	}, nil
}

func (stubReportingForInternal) PaymentsSummary(_ context.Context, q listscope.ReportingQuery) (*appreporting.PaymentsSummaryResponse, error) {
	return &appreporting.PaymentsSummaryResponse{
		OrganizationID: q.OrganizationID.String(),
		From:           q.From.UTC().Format(time.RFC3339),
		To:             q.To.UTC().Format(time.RFC3339),
		GroupBy:        q.GroupBy,
	}, nil
}

func (stubReportingForInternal) FleetHealth(_ context.Context, q listscope.ReportingQuery) (*appreporting.FleetHealthResponse, error) {
	return &appreporting.FleetHealthResponse{
		OrganizationID: q.OrganizationID.String(),
		From:           q.From.UTC().Format(time.RFC3339),
		To:             q.To.UTC().Format(time.RFC3339),
	}, nil
}

func (stubReportingForInternal) InventoryExceptions(_ context.Context, q listscope.ReportingQuery) (*appreporting.InventoryExceptionsResponse, error) {
	return &appreporting.InventoryExceptionsResponse{
		OrganizationID: q.OrganizationID.String(),
		From:           q.From.UTC().Format(time.RFC3339),
		To:             q.To.UTC().Format(time.RFC3339),
	}, nil
}

func (stubReportingForInternal) CashCollectionsExport(_ context.Context, q listscope.ReportingQuery) ([]appreporting.CashCollectionExportRow, error) {
	_ = q
	return nil, nil
}

func (stubReportingForInternal) PaymentSettlement(_ context.Context, q listscope.ReportingQuery) (*appreporting.PaymentSettlementResponse, error) {
	return &appreporting.PaymentSettlementResponse{OrganizationID: q.OrganizationID.String()}, nil
}

func (stubReportingForInternal) Refunds(_ context.Context, q listscope.ReportingQuery) (*appreporting.RefundReportResponse, error) {
	return &appreporting.RefundReportResponse{OrganizationID: q.OrganizationID.String()}, nil
}

func (stubReportingForInternal) CashCollectionsReport(_ context.Context, q listscope.ReportingQuery) (*appreporting.CashCollectionReportResponse, error) {
	return &appreporting.CashCollectionReportResponse{OrganizationID: q.OrganizationID.String()}, nil
}

func (stubReportingForInternal) MachineHealth(_ context.Context, q listscope.ReportingQuery) (*appreporting.MachineHealthReportResponse, error) {
	return &appreporting.MachineHealthReportResponse{OrganizationID: q.OrganizationID.String()}, nil
}

func (stubReportingForInternal) FailedVends(_ context.Context, q listscope.ReportingQuery) (*appreporting.FailedVendReportResponse, error) {
	return &appreporting.FailedVendReportResponse{OrganizationID: q.OrganizationID.String()}, nil
}

func (stubReportingForInternal) ReconciliationQueue(_ context.Context, q listscope.ReportingQuery) (*appreporting.ReconciliationQueueReportResponse, error) {
	return &appreporting.ReconciliationQueueReportResponse{OrganizationID: q.OrganizationID.String()}, nil
}

func (stubReportingForInternal) VendSummary(_ context.Context, q listscope.ReportingQuery) (*appreporting.VendSummaryResponse, error) {
	return &appreporting.VendSummaryResponse{OrganizationID: q.OrganizationID.String()}, nil
}

func (stubReportingForInternal) StockMovement(_ context.Context, q listscope.ReportingQuery) (*appreporting.StockMovementReportResponse, error) {
	return &appreporting.StockMovementReportResponse{OrganizationID: q.OrganizationID.String()}, nil
}

func (stubReportingForInternal) CommandFailures(_ context.Context, q listscope.ReportingQuery) (*appreporting.CommandFailuresReportResponse, error) {
	return &appreporting.CommandFailuresReportResponse{OrganizationID: q.OrganizationID.String()}, nil
}

func (stubReportingForInternal) ReconciliationBI(_ context.Context, q listscope.ReportingQuery) (*appreporting.ReconciliationBIReportResponse, error) {
	return &appreporting.ReconciliationBIReportResponse{OrganizationID: q.OrganizationID.String()}, nil
}

func (stubReportingForInternal) ProductPerformance(_ context.Context, q listscope.ReportingQuery) (*appreporting.ProductPerformanceResponse, error) {
	return &appreporting.ProductPerformanceResponse{OrganizationID: q.OrganizationID.String()}, nil
}

func (stubReportingForInternal) TechnicianFillOperations(_ context.Context, q listscope.ReportingQuery) (*appreporting.TechnicianFillReportResponse, error) {
	return &appreporting.TechnicianFillReportResponse{OrganizationID: q.OrganizationID.String()}, nil
}

func TestInternalGRPCServer_ReadOnlyQueries(t *testing.T) {
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
		},
	}

	machineStub := stubMachineQueries{
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
	}

	payment := domaincommerce.Payment{
		ID:                   paymentID,
		OrderID:              orderID,
		Provider:             "cash",
		State:                "captured",
		AmountMinor:          250,
		Currency:             "THB",
		ReconciliationStatus: "matched",
		SettlementStatus:     "settled",
		CreatedAt:            time.Date(2026, 1, 5, 1, 2, 30, 0, time.UTC),
	}

	services := InternalQueryServices{
		Machine: machineStub,
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
				Payment:        payment,
				PaymentPresent: true,
			},
		},
		Payment:   stubPaymentQueries{payment: payment},
		Catalog:   stubInternalQuerySaleCatalog{orgID: orgID},
		Inventory: appapi.NewInternalInventoryQueryService(machineStub),
		Reporting: stubReportingForInternal{},
	}

	srv, err := NewInternalGRPCServer(cfg, zap.NewNop(), nil, RegisterInternalQueryServices(services))
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

	token := issueTestInternalServiceToken(t, cfg.HTTPAuth.JWTSecret, orgID)
	authCtx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token, "x-request-id", "grpc-test-1")

	machineClient := internalv1.NewInternalMachineQueryServiceClient(conn)
	machineResp, err := machineClient.GetMachineSummary(authCtx, &internalv1.GetMachineSummaryRequest{MachineId: machineID.String()})
	if err != nil {
		t.Fatal(err)
	}
	if got := machineResp.GetMachine().GetMachineId(); got != machineID.String() {
		t.Fatalf("machine id=%q", got)
	}

	telemetryClient := internalv1.NewInternalTelemetryQueryServiceClient(conn)
	telemetryResp, err := telemetryClient.GetLatestMachineTelemetry(authCtx, &internalv1.GetLatestMachineTelemetryRequest{MachineId: machineID.String()})
	if err != nil {
		t.Fatal(err)
	}
	if got := telemetryResp.GetSnapshot().GetEffectiveTimezone(); got != "Asia/Bangkok" {
		t.Fatalf("effective timezone=%q", got)
	}

	commerceClient := internalv1.NewInternalCommerceQueryServiceClient(conn)
	commerceResp, err := commerceClient.GetOrderPaymentVendState(authCtx, &internalv1.GetOrderPaymentVendStateRequest{
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

	paymentClient := internalv1.NewInternalPaymentQueryServiceClient(conn)
	paymentResp, err := paymentClient.GetPaymentById(authCtx, &internalv1.GetPaymentByIdRequest{
		OrganizationId: orgID.String(),
		PaymentId:      paymentID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := paymentResp.GetPayment().GetOrderId(); got != orderID.String() {
		t.Fatalf("payment order id=%q", got)
	}
	latestPaymentResp, err := paymentClient.GetLatestPaymentForOrder(authCtx, &internalv1.GetLatestPaymentForOrderRequest{
		OrganizationId: orgID.String(),
		OrderId:        orderID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := latestPaymentResp.GetPayment().GetId(); got != paymentID.String() {
		t.Fatalf("latest payment id=%q", got)
	}

	catalogClient := internalv1.NewInternalCatalogQueryServiceClient(conn)
	catResp, err := catalogClient.GetSaleCatalogSnapshot(authCtx, &internalv1.GetSaleCatalogSnapshotRequest{MachineId: machineID.String()})
	if err != nil {
		t.Fatal(err)
	}
	var catPayload map[string]any
	if err := json.Unmarshal([]byte(catResp.GetCatalogJson()), &catPayload); err != nil {
		t.Fatal(err)
	}
	gotID, _ := catPayload["MachineID"].(string)
	if gotID == "" {
		gotID, _ = catPayload["machineId"].(string)
	}
	if !strings.EqualFold(strings.TrimSpace(gotID), machineID.String()) {
		t.Fatalf("catalog machine id=%v", gotID)
	}

	invClient := internalv1.NewInternalInventoryQueryServiceClient(conn)
	invResp, err := invClient.GetMachineSlotInventory(authCtx, &internalv1.GetMachineSlotInventoryRequest{MachineId: machineID.String()})
	if err != nil {
		t.Fatal(err)
	}
	if len(invResp.GetLegacySlots()) != 1 {
		t.Fatalf("legacy slots=%d", len(invResp.GetLegacySlots()))
	}

	reportClient := internalv1.NewInternalReportingQueryServiceClient(conn)
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	to := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	repResp, err := reportClient.GetSalesSummary(authCtx, &internalv1.GetSalesSummaryRequest{
		OrganizationId: orgID.String(),
		FromRfc3339:    from,
		ToRfc3339:      to,
		GroupBy:        "day",
	})
	if err != nil {
		t.Fatal(err)
	}
	var repPayload map[string]any
	if err := json.Unmarshal([]byte(repResp.GetSummaryJson()), &repPayload); err != nil {
		t.Fatal(err)
	}
	if got, _ := repPayload["organizationId"].(string); got != orgID.String() {
		t.Fatalf("report org=%v", got)
	}
}

func issueTestAccessToken(t *testing.T, cfg *config.Config, orgID uuid.UUID, roles []string) string {
	t.Helper()
	issuer, err := platformauth.NewSessionIssuerFromHTTPAuth(cfg.HTTPAuth)
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := issuer.IssueAccessJWT(uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), orgID, roles, "active")
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func timePtr(v time.Time) *time.Time {
	return &v
}
