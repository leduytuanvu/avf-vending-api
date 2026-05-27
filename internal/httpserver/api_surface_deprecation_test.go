package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestLegacySuccessorForRequest_adminUsersAlias(t *testing.T) {
	t.Parallel()
	r := chi.NewRouter()
	var inner *http.Request
	r.Get("/v1/admin/users", func(w http.ResponseWriter, r *http.Request) {
		inner = r
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/users", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if inner == nil {
		t.Fatal("handler not reached")
	}
	canon, ok := legacySuccessorForRequest(inner)
	if !ok {
		t.Fatal("expected legacy successor")
	}
	if canon != "/v1/admin/auth/users" {
		t.Fatalf("canon: got %q", canon)
	}
}

func TestApiSurfaceDeprecationMiddleware_setsHeaders(t *testing.T) {
	t.Parallel()
	r := chi.NewRouter()
	r.Use(apiSurfaceDeprecationMiddleware())
	r.Get("/v1/admin/media", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/media", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Header().Get("Deprecation") != "true" {
		t.Fatalf("Deprecation header: %q", rec.Header().Get("Deprecation"))
	}
	if rec.Header().Get("Link") == "" {
		t.Fatal("expected Link header")
	}
}
