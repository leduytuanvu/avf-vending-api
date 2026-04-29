package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	appcatalogadmin "github.com/avf/avf-vending-api/internal/app/catalogadmin"
	"github.com/avf/avf-vending-api/internal/config"
	auth "github.com/avf/avf-vending-api/internal/platform/auth"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func testJWTSecret(t *testing.T) []byte {
	t.Helper()
	return bytes.Repeat([]byte("z"), 32)
}

func testSessionIssuer(t *testing.T, secret []byte) *auth.SessionIssuer {
	t.Helper()
	iss, err := auth.NewSessionIssuerFromHTTPAuth(config.HTTPAuthConfig{
		JWTSecret:        secret,
		JWTLeeway:        45 * time.Second,
		ExpectedIssuer:   "test-issuer",
		ExpectedAudience: "test-audience",
		AccessTokenTTL:   30 * time.Minute,
		RefreshTokenTTL:  720 * time.Hour,
	})
	require.NoError(t, err)
	require.NotNil(t, iss)
	return iss
}

func testMountV1ForAdminREST(t *testing.T) http.Handler {
	t.Helper()
	secret := testJWTSecret(t)
	_ = testSessionIssuer(t, secret)

	app := &api.HTTPApplication{
		CatalogAdmin: new(appcatalogadmin.Service),
		ListPaymentProviders: func() []api.PaymentProviderRegistryInfo {
			return nil
		},
	}

	r := chi.NewRouter()
	cfg := &config.Config{}
	cfg.TransportBoundary.MachineRESTLegacyEnabled = true

	v := auth.NewHS256AccessTokenValidator(secret, 45*time.Second)
	writeRL := func(h http.Handler) http.Handler { return h }

	mountV1(r, app, zap.NewNop(), cfg, v, writeRL, nil)

	return r
}

func TestAdminREST_noBearer_unauthorizedPaymentProvidersListing(t *testing.T) {
	t.Parallel()
	h := testMountV1ForAdminREST(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/payment/providers", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	var env map[string]any
	require.NoError(t, json.NewDecoder(strings.NewReader(rec.Body.String())).Decode(&env))
	if code, ok := env["code"].(string); ok {
		require.Equal(t, "unauthenticated", code)
	}
}

func TestAdminREST_viewer_forbiddenCatalogMutationPost(t *testing.T) {
	t.Parallel()

	secret := testJWTSecret(t)
	org := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	app := &api.HTTPApplication{CatalogAdmin: new(appcatalogadmin.Service)}
	r := chi.NewRouter()
	cfg := &config.Config{}
	cfg.TransportBoundary.MachineRESTLegacyEnabled = true
	v := auth.NewHS256AccessTokenValidator(secret, 45*time.Second)
	writeRL := func(h http.Handler) http.Handler { return h }
	mountV1(r, app, zap.NewNop(), cfg, v, writeRL, nil)

	iss := testSessionIssuer(t, secret)
	tok, _, err := iss.IssueAccessJWT(uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		org, []string{"viewer"}, "active")
	require.NoError(t, err)

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/products?organization_id="+org.String(), body)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
}
