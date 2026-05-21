package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appinventoryadmin "github.com/avf/avf-vending-api/internal/app/inventoryadmin"
	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

func TestMountV1_productionAdminOperatorSessionStart_registeredWithoutLegacyREST(t *testing.T) {
	t.Parallel()
	r := chi.NewRouter()
	app := &api.HTTPApplication{
		InventoryAdmin: new(appinventoryadmin.Service),
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
	want := "POST /v1/admin/machines/{machineId}/operator-sessions/start"
	found := false
	legacyLogin := false
	for _, line := range routes {
		if line == want {
			found = true
		}
		if strings.Contains(line, "/v1/machines/{machineId}/operator-sessions/login") {
			legacyLogin = true
		}
	}
	if !found {
		t.Fatalf("missing admin operator session start route; routes:\n%s", strings.Join(routes, "\n"))
	}
	if legacyLogin {
		t.Fatal("legacy operator login must not register when MachineRESTLegacyEnabled=false")
	}
}

func TestParseOperatorSessionIDField_emptyRejected(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	_, ok := parseOperatorSessionIDField(rec, req, "")
	if ok {
		t.Fatal("expected false for empty operator_session_id")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}
