package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/avf/avf-vending-api/internal/config"
	"go.uber.org/zap"
)

func TestSensitiveWriteRateLimit_disabledNoOp(t *testing.T) {
	cfg := config.HTTPRateLimitConfig{SensitiveWritesEnabled: false}
	mw := SensitiveWriteRateLimitIfEnabled(cfg, zap.NewNop())
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/x", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("iteration %d: status %d", i, rr.Code)
		}
	}
}

func TestSensitiveWriteRateLimit_POST_exhaustsBurst(t *testing.T) {
	cfg := config.HTTPRateLimitConfig{
		SensitiveWritesEnabled: true,
		SensitiveWritesRPS:     0.0001,
		SensitiveWritesBurst:   1,
	}
	mw := SensitiveWriteRateLimitIfEnabled(cfg, zap.NewNop())
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/v1/commerce/orders", nil)
		r.RemoteAddr = "192.0.2.1:1234"
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, r)
		return rr
	}

	if req().Code != http.StatusOK {
		t.Fatal("first POST should succeed")
	}
	if req().Code != http.StatusTooManyRequests {
		t.Fatalf("second POST should be rate limited, got %d", req().Code)
	}

	rrGet := httptest.NewRecorder()
	rGet := httptest.NewRequest(http.MethodGet, "/v1/commerce/orders", nil)
	rGet.RemoteAddr = "192.0.2.1:1234"
	h.ServeHTTP(rrGet, rGet)
	if rrGet.Code != http.StatusOK {
		t.Fatalf("GET should bypass limiter, got %d", rrGet.Code)
	}
}
