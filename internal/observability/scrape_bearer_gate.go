package observability

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// ScrapeBearerGate requires Authorization: Bearer <token> (trimmed; constant-time compare).
// Used for Prometheus /metrics when a scrape token is configured (public or private listener).
func ScrapeBearerGate(token string, h http.Handler) http.Handler {
	want := []byte(strings.TrimSpace(token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := strings.TrimSpace(r.Header.Get("Authorization"))
		const pfx = "Bearer "
		if !strings.HasPrefix(raw, pfx) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="prometheus"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		got := []byte(strings.TrimSpace(strings.TrimPrefix(raw, pfx)))
		if len(got) != len(want) || subtle.ConstantTimeCompare(got, want) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="prometheus"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
}
