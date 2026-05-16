package httpserver

import (
	"net/http"
	"strings"
	"testing"

	"github.com/avf/avf-vending-api/internal/app/api"
	appauth "github.com/avf/avf-vending-api/internal/app/auth"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

func TestMountV1_adminAuthUsersRoutesRegistered(t *testing.T) {
	t.Parallel()
	r := chi.NewRouter()
	// Wire a non-nil Auth service so admin auth routes register (handlers are not invoked by chi.Walk).
	app := &api.HTTPApplication{Auth: &appauth.Service{}}
	cfg := &config.Config{}
	writeRL := func(h http.Handler) http.Handler { return h }
	mountV1(r, app, zap.NewNop(), cfg, stubAccessTokenValidator{}, writeRL, nil)

	var routes []string
	if err := chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes = append(routes, method+" "+route)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"GET /v1/admin/auth/users",
		"POST /v1/admin/auth/users",
		"GET /v1/admin/auth/users/{accountId}",
		"PATCH /v1/admin/auth/users/{accountId}",
		"PATCH /v1/admin/auth/users/{accountId}/status",
		"POST /v1/admin/auth/users/{accountId}/activate",
		"POST /v1/admin/auth/users/{accountId}/deactivate",
		"POST /v1/admin/auth/users/{accountId}/reset-password",
		"POST /v1/admin/auth/users/{accountId}/revoke-sessions",
		"POST /v1/admin/auth/users/{accountId}/roles",
		"PUT /v1/admin/auth/users/{accountId}/roles",
		"PATCH /v1/admin/auth/users/{accountId}/roles",
		"GET /v1/admin/users",
		"POST /v1/admin/users",
		"GET /v1/admin/users/{userId}",
		"PATCH /v1/admin/users/{userId}",
		"PATCH /v1/admin/users/{userId}/status",
		"POST /v1/admin/users/{userId}/roles",
		"PUT /v1/admin/users/{userId}/roles",
		"PATCH /v1/admin/users/{userId}/roles",
		"POST /v1/admin/users/{userId}/enable",
		"POST /v1/admin/users/{userId}/disable",
		"POST /v1/admin/users/{userId}/revoke-sessions",
		"POST /v1/admin/users/{userId}/reset-password",
		"POST /v1/auth/change-password",
		"POST /v1/auth/password/change",
		"POST /v1/auth/password/reset/request",
		"POST /v1/auth/password/reset/confirm",
	}
	for _, w := range want {
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
