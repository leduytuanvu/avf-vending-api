package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestRequestMetricsMiddleware_RecordsStatus(t *testing.T) {
	t.Parallel()
	r := chi.NewRouter()
	r.Use(requestMetricsMiddleware())
	r.Get("/health/live", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code %d", rec.Code)
	}
}

func TestRouteLabel_Unmatched(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/nowhere", nil)
	if g := routeLabel(req); g != "unknown" {
		t.Fatalf("got %q want unknown", g)
	}
}
