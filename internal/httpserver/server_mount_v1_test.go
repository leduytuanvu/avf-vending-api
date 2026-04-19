package httpserver

import (
	"context"
	"net/http"
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
