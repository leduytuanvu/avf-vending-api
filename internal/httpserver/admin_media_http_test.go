package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/avf/avf-vending-api/internal/app/api"
	appmediaadmin "github.com/avf/avf-vending-api/internal/app/mediaadmin"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMountAdminMediaRoutes_alwaysRegistersExternalImagesRoute(t *testing.T) {
	t.Parallel()
	r := chi.NewRouter()
	mountAdminMediaRoutes(r, &api.HTTPApplication{MediaAdmin: nil}, nil)

	var routes []string
	require.NoError(t, chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes = append(routes, method+" "+route)
		return nil
	}))
	found := false
	for _, got := range routes {
		if got == "POST /media/external-images" {
			found = true
			break
		}
	}
	require.True(t, found, "routes: %s", strings.Join(routes, "\n"))
}

func TestWithMediaAdmin_nilServiceReturns503JSON(t *testing.T) {
	t.Parallel()
	h := withMediaAdmin(&api.HTTPApplication{MediaAdmin: nil}, func(_ *appmediaadmin.Service) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}
	})
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/media", nil))
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Contains(t, rec.Body.String(), "capability_not_configured")
}

func TestWithMediaUpload_nilStoreReturns503JSON(t *testing.T) {
	t.Parallel()
	h := withMediaUpload(&api.HTTPApplication{MediaAdmin: nil}, func(_ *appmediaadmin.Service) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}
	})
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPost, "/media/uploads/init", nil))
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Contains(t, rec.Body.String(), "capability_not_configured")
}

func TestPostAdminExternalProductImage_featureDisabledReturns503(t *testing.T) {
	t.Parallel()
	r := chi.NewRouter()
	r.Post("/v1/admin/media/external-images", postAdminExternalProductImage(&api.HTTPApplication{MediaAdmin: nil}))
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/media/external-images", strings.NewReader(`{"url":"https://adm.avf.vn/x.png"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", uuid.NewString())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Contains(t, rec.Body.String(), "capability_not_configured")
}
