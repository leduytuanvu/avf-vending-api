package httpserver

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// legacyRouteSuccessor maps Chi route patterns to canonical successor path templates.
// Patterns use Chi param names; Link header uses the template (clients resolve IDs).
var legacyRouteSuccessor = map[string]string{
	"/v1/admin/users":                            "/v1/admin/auth/users",
	"/v1/admin/users/{userId}":                   "/v1/admin/auth/users/{accountId}",
	"/v1/admin/users/{userId}/sessions":          "/v1/admin/auth/users/{accountId}/sessions",
	"/v1/admin/users/{userId}/roles":             "/v1/admin/auth/users/{accountId}/roles",
	"/v1/admin/users/{userId}/roles/{role}":      "/v1/admin/auth/users/{accountId}/roles/{role}",
	"/v1/admin/media":                            "/v1/admin/media/assets",
	"/v1/admin/media/{mediaId}":                  "/v1/admin/media/assets/{mediaId}",
	"/v1/admin/media/uploads":                    "/v1/admin/media/uploads/init",
	"/v1/admin/media/{mediaId}/complete":         "/v1/admin/media/uploads/{mediaId}/complete",
	"/v1/admin/products/{productId}/image":       "/v1/admin/products/{productId}/media",
	"/v1/machines/{machineId}/commands/dispatch": "/v1/admin/machines/{machineId}/commands",
}

func apiSurfaceDeprecationMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
			if canon, ok := legacySuccessorForRequest(r); ok {
				w.Header().Set("Deprecation", "true")
				w.Header().Set("Link", "<"+canon+">; rel=\"successor-version\"")
			}
		})
	}
}

func legacySuccessorForRequest(r *http.Request) (string, bool) {
	rc := chi.RouteContext(r.Context())
	if rc == nil {
		return "", false
	}
	pattern := strings.TrimSpace(rc.RoutePattern())
	if pattern == "" {
		return "", false
	}
	canon, ok := legacyRouteSuccessor[pattern]
	if !ok {
		return "", false
	}
	// Role mutation aliases share PUT handler; mark POST/PATCH as deprecated.
	if strings.HasSuffix(pattern, "/roles") && (r.Method == http.MethodPost || r.Method == http.MethodPatch) {
		return canon, true
	}
	if strings.HasSuffix(pattern, "/roles") && r.Method == http.MethodPut {
		return "", false
	}
	return canon, true
}
