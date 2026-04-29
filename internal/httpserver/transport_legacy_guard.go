package httpserver

import (
	"net/http"

	"github.com/avf/avf-vending-api/internal/config"
)

// machineLegacyRESTGuard rejects HTTP requests when ENABLE_LEGACY_MACHINE_HTTP / TransportBoundary.MachineRESTLegacyEnabled is false.
// Prefer conditional route registration in mountV1 so these paths are not exposed; this guard remains for defense-in-depth if routes are mounted.
func machineLegacyRESTGuard(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg != nil && cfg.TransportBoundary.MachineRESTLegacyEnabled {
				next.ServeHTTP(w, r)
				return
			}
			writeAPIError(w, r.Context(), http.StatusNotFound, "legacy_machine_rest_disabled",
				"machine REST runtime API is disabled; use native gRPC (see docs/architecture/transport-boundary.md and docs/api/machine-grpc.md)")
		})
	}
}
