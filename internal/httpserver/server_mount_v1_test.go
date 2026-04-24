package httpserver

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type stubAccessTokenValidator struct{}

func (stubAccessTokenValidator) ValidateAccessToken(ctx context.Context, rawJWT string) (auth.Principal, error) {
	return auth.Principal{}, auth.ErrUnauthenticated
}

func TestMountV1_noDuplicateAuthRoute(t *testing.T) {
	t.Parallel()
	r := chi.NewRouter()
	app := &api.HTTPApplication{}
	cfg := &config.Config{}
	writeRL := func(h http.Handler) http.Handler { return h }
	mountV1(r, app, zap.NewNop(), cfg, stubAccessTokenValidator{}, writeRL)
}

func TestMountV1_machineSetupRoutesRegistered(t *testing.T) {
	t.Parallel()
	r := chi.NewRouter()
	app := &api.HTTPApplication{}
	cfg := &config.Config{}
	writeRL := func(h http.Handler) http.Handler { return h }
	mountV1(r, app, zap.NewNop(), cfg, stubAccessTokenValidator{}, writeRL)

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
		// Device HTTP bridge (registered under /v1 when HTTPApplication is non-nil; handlers guard nil deps).
		"POST /v1/device/machines/{machineId}/vend-results",
		"POST /v1/device/machines/{machineId}/commands/poll",
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
