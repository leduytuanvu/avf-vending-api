package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

func TestMountSwaggerUI_servesOpenAPIJSON(t *testing.T) {
	t.Parallel()
	r := chi.NewRouter()
	MountSwaggerUI(r, zap.NewNop())
	for _, path := range []string{"/swagger/doc.json", "/swagger/index.html"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status=%d body=%s", path, rec.Code, rec.Body.String())
		}
		if path == "/swagger/doc.json" {
			ct := rec.Header().Get("Content-Type")
			if !strings.Contains(ct, "application/json") && !strings.Contains(ct, "charset=utf-8") {
				if !strings.Contains(ct, "json") {
					t.Fatalf("unexpected content-type: %q", ct)
				}
			}
			body := rec.Body.String()
			if !strings.Contains(body, `"swagger"`) && !strings.Contains(body, `"openapi"`) {
				t.Fatalf("expected swagger or openapi key in body prefix: %s", body[:min(80, len(body))])
			}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
