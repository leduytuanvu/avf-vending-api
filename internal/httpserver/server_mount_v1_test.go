package httpserver

import (
	"context"
	"net/http"
	"strings"
	"testing"

	appactivation "github.com/avf/avf-vending-api/internal/app/activation"
	appadminops "github.com/avf/avf-vending-api/internal/app/adminops"
	appanomalies "github.com/avf/avf-vending-api/internal/app/anomalies"
	"github.com/avf/avf-vending-api/internal/app/api"
	appaudit "github.com/avf/avf-vending-api/internal/app/audit"
	appfleet "github.com/avf/avf-vending-api/internal/app/fleet"
	appmediaadmin "github.com/avf/avf-vending-api/internal/app/mediaadmin"
	apppayments "github.com/avf/avf-vending-api/internal/app/payments"
	approvisioning "github.com/avf/avf-vending-api/internal/app/provisioning"
	approllout "github.com/avf/avf-vending-api/internal/app/rollout"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type stubAccessTokenValidator struct{}

func (stubAccessTokenValidator) ValidateAccessToken(ctx context.Context, rawJWT string) (auth.Principal, error) {
	return auth.Principal{}, auth.ErrUnauthenticated
}

func legacyMachineHTTPTestConfig() *config.Config {
	return &config.Config{
		AppEnv:            config.AppEnvDevelopment,
		TransportBoundary: config.TransportBoundaryConfig{MachineRESTLegacyEnabled: true},
	}
}

func TestMountV1_noDuplicateAuthRoute(t *testing.T) {
	t.Parallel()
	r := chi.NewRouter()
	app := &api.HTTPApplication{}
	writeRL := func(h http.Handler) http.Handler { return h }
	mountV1(r, app, zap.NewNop(), legacyMachineHTTPTestConfig(), stubAccessTokenValidator{}, writeRL, nil)
}

func TestMountV1_machineSetupRoutesRegistered(t *testing.T) {
	t.Parallel()
	r := chi.NewRouter()
	var stubOps appadminops.Service
	var stubProvisioning approvisioning.Service
	var stubRollout approllout.Service
	app := &api.HTTPApplication{
		// Non-nil pointers register org-scoped routes (handlers still require real wiring at runtime).
		Activation:      new(appactivation.Service),
		EnterpriseAudit: new(appaudit.Service),
		MediaAdmin:      new(appmediaadmin.Service),
		Fleet:           new(appfleet.Service),
		AdminOps:        &stubOps,
		Anomalies:       new(appanomalies.Service),
		Provisioning:    &stubProvisioning,
		Rollout:         &stubRollout,
		// Non-nil stub registers payment ops routes (handlers still validate deps at runtime).
		PaymentOps: new(apppayments.AdminService),
	}
	cfg := legacyMachineHTTPTestConfig()
	writeRL := func(h http.Handler) http.Handler { return h }
	mountV1(r, app, zap.NewNop(), cfg, stubAccessTokenValidator{}, writeRL, nil)

	var routes []string
	if err := chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes = append(routes, method+" "+route)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	wantContains := []string{
		"GET /v1/setup/machines/{machineId}/bootstrap",
		"GET /v1/admin/machines/{machineId}",
		"GET /v1/admin/organizations/{organizationId}/machines",
		"POST /v1/admin/organizations/{organizationId}/machines/{machineId}/rotate-credentials",
		"POST /v1/admin/organizations/{organizationId}/machines/{machineId}/transfer-site",
		"GET /v1/admin/organizations/{organizationId}/activation-codes",
		"POST /v1/admin/organizations/{organizationId}/machines/{machineId}/revoke-sessions",
		"DELETE /v1/admin/organizations/{organizationId}/machines/{machineId}/technicians/{userId}",
		"POST /v1/admin/media/uploads",
		"GET /v1/admin/media",
		"POST /v1/admin/organizations/{organizationId}/media/uploads/init",
		"POST /v1/admin/organizations/{organizationId}/media/product-images",
		"GET /v1/admin/organizations/{organizationId}/media/assets",
		"DELETE /v1/admin/organizations/{organizationId}/media/assets/{assetId}",
		"POST /v1/admin/organizations/{organizationId}/products/{productId}/media",
		"DELETE /v1/admin/organizations/{organizationId}/products/{productId}/media/{mediaId}",
		"POST /v1/admin/organizations/{organizationId}/products/{productId}/images",
		"GET /v1/admin/organizations/{organizationId}/products/{productId}/images",
		// Device HTTP bridge (registered under /v1 when HTTPApplication is non-nil; handlers guard nil deps).
		"POST /v1/device/machines/{machineId}/vend-results",
		"POST /v1/device/machines/{machineId}/commands/poll",
		// P1.2 admin operations (mounted when AdminOps non-nil).
		"GET /v1/admin/organizations/{organizationId}/operations/machines/health",
		"GET /v1/admin/organizations/{organizationId}/payments/webhook-events",
		"GET /v1/admin/organizations/{organizationId}/payments/settlements",
		"POST /v1/admin/organizations/{organizationId}/payments/settlements/import",
		"GET /v1/admin/organizations/{organizationId}/payments/disputes",
		"POST /v1/admin/organizations/{organizationId}/payments/disputes/{disputeId}/resolve",
		"GET /v1/admin/organizations/{organizationId}/payments/export",
		"GET /v1/admin/organizations/{organizationId}/machines/{machineId}/health",
		"GET /v1/admin/organizations/{organizationId}/machines/{machineId}/timeline",
		"GET /v1/admin/organizations/{organizationId}/commands",
		"GET /v1/admin/organizations/{organizationId}/commands/{commandId}",
		"POST /v1/admin/organizations/{organizationId}/commands/{commandId}/retry",
		"POST /v1/admin/organizations/{organizationId}/commands/{commandId}/cancel",
		"POST /v1/admin/organizations/{organizationId}/provisioning/machines/bulk",
		"GET /v1/admin/organizations/{organizationId}/provisioning/batches/{batchId}",
		"POST /v1/admin/organizations/{organizationId}/rollouts",
		"GET /v1/admin/organizations/{organizationId}/rollouts",
		"GET /v1/admin/organizations/{organizationId}/rollouts/{rolloutId}",
		"POST /v1/admin/organizations/{organizationId}/rollouts/{rolloutId}/start",
		"POST /v1/admin/organizations/{organizationId}/rollouts/{rolloutId}/pause",
		"POST /v1/admin/organizations/{organizationId}/rollouts/{rolloutId}/resume",
		"POST /v1/admin/organizations/{organizationId}/rollouts/{rolloutId}/cancel",
		"POST /v1/admin/organizations/{organizationId}/rollouts/{rolloutId}/rollback",
		"GET /v1/admin/organizations/{organizationId}/anomalies",
		"GET /v1/admin/organizations/{organizationId}/anomalies/{anomalyId}",
		"POST /v1/admin/organizations/{organizationId}/anomalies/{anomalyId}/resolve",
		"POST /v1/admin/organizations/{organizationId}/anomalies/{anomalyId}/ignore",
		"GET /v1/admin/organizations/{organizationId}/restock/suggestions",
		"GET /v1/admin/audit/events",
		"GET /v1/admin/organizations/{organizationId}/audit-events",
		"GET /v1/admin/organizations/{organizationId}/audit-events/{auditEventId}",
	}
	for _, w := range wantContains {
		var found bool
		for _, got := range routes {
			if strings.Contains(got, w) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing route pattern %q in:\n%s", w, strings.Join(routes, "\n"))
		}
	}
}

func TestMountV1_productionLegacyMachineHTTPOff_skipsLegacyRuntimeRoutes(t *testing.T) {
	t.Parallel()
	r := chi.NewRouter()
	var stubOps appadminops.Service
	app := &api.HTTPApplication{
		Activation:      new(appactivation.Service),
		EnterpriseAudit: new(appaudit.Service),
		MediaAdmin:      new(appmediaadmin.Service),
		Fleet:           new(appfleet.Service),
		AdminOps:        &stubOps,
		Anomalies:       new(appanomalies.Service),
		Provisioning:    new(approvisioning.Service),
		Rollout:         new(approllout.Service),
		PaymentOps:      new(apppayments.AdminService),
	}
	cfg := &config.Config{
		AppEnv:            config.AppEnvProduction,
		TransportBoundary: config.TransportBoundaryConfig{MachineRESTLegacyEnabled: false},
	}
	writeRL := func(h http.Handler) http.Handler { return h }
	mountV1(r, app, zap.NewNop(), cfg, stubAccessTokenValidator{}, writeRL, nil)

	var routes []string
	if err := chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes = append(routes, method+" "+route)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	forbidden := []string{
		"/v1/setup/machines/",
		"/v1/device/machines/",
		"/v1/machines/{machineId}/telemetry/",
		"/v1/machines/{machineId}/check-ins",
	}
	for _, line := range routes {
		for _, sub := range forbidden {
			if strings.Contains(line, sub) {
				t.Fatalf("legacy machine HTTP should not be registered, got route %q (matched %q)", line, sub)
			}
		}
		if strings.HasPrefix(line, "POST /v1/commerce/orders") && !strings.Contains(line, "/webhooks") {
			t.Fatalf("legacy commerce REST should not be registered, got %q", line)
		}
	}

	// Fleet command dispatch remains available (not legacy vending runtime).
	foundDispatch := false
	for _, line := range routes {
		if strings.Contains(line, "POST /v1/machines/{machineId}/commands/dispatch") {
			foundDispatch = true
			break
		}
	}
	if !foundDispatch {
		t.Fatalf("expected command dispatch route, routes:\n%s", strings.Join(routes, "\n"))
	}
}
