package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/ratelimit"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func testAbuseConfig(limit int) config.AbuseRateLimitConfig {
	return config.AbuseRateLimitConfig{
		Enabled:                  true,
		LoginPerMinute:           limit,
		RefreshPerMinute:         limit,
		AdminMutationPerMinute:   limit,
		MachinePerMinute:         limit,
		WebhookPerMinute:         limit,
		PublicPerMinute:          limit,
		CommandDispatchPerMinute: limit,
		ReportsReadPerMinute:     limit,
		LockoutWindow:            time.Minute,
	}
}

func TestAbuseProtection_LoginPOST_rateLimited429WithRetryAfter(t *testing.T) {
	t.Parallel()
	mem := ratelimit.NewMemoryBackend()
	cfg := testAbuseConfig(2)
	a := NewAbuseProtection(cfg, mem, zap.NewNop())
	h := a.LoginPOST()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	body := func() *strings.Reader {
		return strings.NewReader(`{"organizationId":"11111111-1111-1111-1111-111111111111","email":"u@test.example.com","password":"x"}`)
	}
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", body())
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, "iteration %d", i)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", body())
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.NotEmpty(t, rec.Header().Get("Retry-After"))
	var env map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	errObj := env["error"].(map[string]any)
	require.Equal(t, "rate_limited", errObj["code"])
	details := errObj["details"].(map[string]any)
	require.NotNil(t, details["retry_after_seconds"])
}

func TestAbuseProtection_LoginPOST_disabledBypasses(t *testing.T) {
	t.Parallel()
	mem := ratelimit.NewMemoryBackend()
	cfg := testAbuseConfig(1)
	cfg.Enabled = false
	a := NewAbuseProtection(cfg, mem, zap.NewNop())
	h := a.LoginPOST()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{"email":"z@test.example.com","password":"p","organizationId":"11111111-1111-1111-1111-111111111111"}`))
		req.RemoteAddr = "127.0.0.1:9"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		require.Equal(t, http.StatusTeapot, rec.Code)
	}
}

func TestAbuseProtection_AdminMutation_GETNotLimited(t *testing.T) {
	t.Parallel()
	mem := ratelimit.NewMemoryBackend()
	cfg := testAbuseConfig(1)
	a := NewAbuseProtection(cfg, mem, zap.NewNop())
	h := a.AdminMutation()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	org := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/products", nil)
		req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{Subject: "acc", OrganizationID: org}))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
	}
}

func TestAbuseProtection_AdminMutation_POSTLimitedByAccountOrg(t *testing.T) {
	t.Parallel()
	mem := ratelimit.NewMemoryBackend()
	cfg := testAbuseConfig(2)
	a := NewAbuseProtection(cfg, mem, zap.NewNop())
	h := a.AdminMutation()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	org := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/admin/products", nil)
		req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{Subject: "acc", OrganizationID: org}))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, "iter %d", i)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/products", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{Subject: "acc", OrganizationID: org}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
}

func TestAbuseProtection_MachineScoped_keyedByMachineID(t *testing.T) {
	t.Parallel()
	mem := ratelimit.NewMemoryBackend()
	cfg := testAbuseConfig(2)
	a := NewAbuseProtection(cfg, mem, zap.NewNop())
	h := a.MachineScoped()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	mid := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	path := "/v1/machines/" + mid.String() + "/telemetry/snapshot"
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)

	other := uuid.New()
	path2 := "/v1/machines/" + other.String() + "/telemetry/incidents"
	req2 := httptest.NewRequest(http.MethodGet, path2, nil)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code, "different machine id should have independent counter")
}

func TestAbuseProtection_ActivationClaimPOST_rateLimited429WithRetryAfter(t *testing.T) {
	t.Parallel()
	mem := ratelimit.NewMemoryBackend()
	cfg := testAbuseConfig(2)
	a := NewAbuseProtection(cfg, mem, zap.NewNop())
	h := a.ActivationClaimPOST()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	body := func() *strings.Reader {
		return strings.NewReader(`{"activationCode":"CODE-ABC","deviceFingerprint":{"androidId":"a"}}`)
	}
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/setup/activation-codes/claim", body())
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, "iteration %d", i)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/setup/activation-codes/claim", body())
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.NotEmpty(t, rec.Header().Get("Retry-After"))
}

func TestAbuseProtection_ActivationClaimPOST_differentCodesIndependent(t *testing.T) {
	t.Parallel()
	mem := ratelimit.NewMemoryBackend()
	cfg := testAbuseConfig(1)
	a := NewAbuseProtection(cfg, mem, zap.NewNop())
	h := a.ActivationClaimPOST()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req1 := httptest.NewRequest(http.MethodPost, "/v1/setup/activation-codes/claim", strings.NewReader(`{"activationCode":"CODE-ONE"}`))
	req1.RemoteAddr = "192.168.1.9:1"
	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, req1)
	require.Equal(t, http.StatusOK, rec1.Code)

	req2 := httptest.NewRequest(http.MethodPost, "/v1/setup/activation-codes/claim", strings.NewReader(`{"activationCode":"CODE-TWO"}`))
	req2.RemoteAddr = "192.168.1.9:1"
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code, "different activation code should use a separate bucket")
}

func TestAbuseProtection_CommandDispatchPOST_rateLimited(t *testing.T) {
	t.Parallel()
	mem := ratelimit.NewMemoryBackend()
	cfg := testAbuseConfig(2)
	a := NewAbuseProtection(cfg, mem, zap.NewNop())
	h := a.CommandDispatchPOST()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	mid := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	path := "/v1/machines/" + mid.String() + "/commands/dispatch"
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.RemoteAddr = "127.0.0.1:4444"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, "iter %d", i)
	}
	req := httptest.NewRequest(http.MethodPost, path, nil)
	req.RemoteAddr = "127.0.0.1:4444"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
}

func TestAbuseProtection_nilLimiterSafe(t *testing.T) {
	t.Parallel()
	a := &AbuseProtection{}
	require.NotPanics(t, func() {
		_ = a.LoginPOST()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	})
}
