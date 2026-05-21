package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	appcatalogadmin "github.com/avf/avf-vending-api/internal/app/catalogadmin"
	appmediaadmin "github.com/avf/avf-vending-api/internal/app/mediaadmin"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func testMountV1WithMediaAdmin(t *testing.T, mediaAdmin *appmediaadmin.Service) http.Handler {
	t.Helper()
	secret := testJWTSecret(t)
	app := &api.HTTPApplication{
		CatalogAdmin: new(appcatalogadmin.Service),
		MediaAdmin:   mediaAdmin,
	}
	r := chi.NewRouter()
	cfg := &config.Config{}
	cfg.TransportBoundary.MachineRESTLegacyEnabled = true
	v := auth.NewHS256AccessTokenValidator(secret, 45*time.Second)
	writeRL := func(h http.Handler) http.Handler { return h }
	mountV1(r, app, zap.NewNop(), cfg, v, writeRL, nil)
	return r
}

func testAdminBearerToken(t *testing.T, roles ...string) string {
	t.Helper()
	secret := testJWTSecret(t)
	iss := testSessionIssuer(t, secret)
	org := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	tok, _, err := iss.IssueAccessJWT(uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), org, roles, "active")
	require.NoError(t, err)
	return tok
}

func TestMountV1_nilMediaAdmin_mediaUploadInitRouteRegistered(t *testing.T) {
	t.Parallel()
	r := chi.NewRouter()
	app := &api.HTTPApplication{CatalogAdmin: new(appcatalogadmin.Service)}
	cfg := &config.Config{}
	cfg.TransportBoundary.MachineRESTLegacyEnabled = true
	writeRL := func(h http.Handler) http.Handler { return h }
	mountV1(r, app, zap.NewNop(), cfg, auth.NewHS256AccessTokenValidator(testJWTSecret(t), 45*time.Second), writeRL, nil)

	var routes []string
	require.NoError(t, chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes = append(routes, method+" "+route)
		return nil
	}))

	want := "POST /v1/admin/media/uploads/init"
	var found bool
	for _, got := range routes {
		if strings.Contains(got, want) {
			found = true
			break
		}
	}
	require.True(t, found, "missing route %q in:\n%s", want, strings.Join(routes, "\n"))
}

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

func TestAdminMediaUploadInit_nilMediaAdmin_returns503CapabilityNotConfigured(t *testing.T) {
	t.Parallel()
	h := testMountV1WithMediaAdmin(t, nil)
	body := strings.NewReader(`{"filename":"coca-330ml.png","contentType":"image/png","purpose":"product_image"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/media/uploads/init", body)
	req.Header.Set("Authorization", "Bearer "+testAdminBearerToken(t, "admin"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Idempotency-Key", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "application/json")

	var env map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	errObj, ok := env["error"].(map[string]any)
	require.True(t, ok, "body: %s", rec.Body.String())
	require.Equal(t, "capability_not_configured", errObj["code"])
	details, ok := errObj["details"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "admin.media", details["capability"])
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

func TestWriteMediaAdminError_notConfigured_returns503CapabilityNotConfigured(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	writeMediaAdminError(rec, context.Background(), appmediaadmin.ErrNotConfigured)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	errObj := body["error"].(map[string]any)
	require.Equal(t, "capability_not_configured", errObj["code"])
}

func TestWithCloudinaryUpload_notConfiguredReturns503(t *testing.T) {
	t.Parallel()
	h := withCloudinaryUpload(&api.HTTPApplication{MediaAdmin: nil}, func(_ *api.HTTPApplication) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }
	})
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPost, "/v1/admin/product-images", nil))
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Contains(t, rec.Body.String(), "capability_not_configured")
}

func TestMountAdminMediaRoutes_registersProductImagesRoutes(t *testing.T) {
	t.Parallel()
	r := chi.NewRouter()
	mountAdminMediaRoutes(r, &api.HTTPApplication{MediaAdmin: nil}, nil)

	var routes []string
	require.NoError(t, chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes = append(routes, method+" "+route)
		return nil
	}))
	for _, want := range []string{"POST /product-images", "POST /media/product-images"} {
		found := false
		for _, got := range routes {
			if got == want {
				found = true
				break
			}
		}
		require.True(t, found, "missing %q in:\n%s", want, strings.Join(routes, "\n"))
	}
}
