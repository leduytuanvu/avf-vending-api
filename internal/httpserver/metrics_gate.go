package httpserver

import (
	"net/http"

	"github.com/avf/avf-vending-api/internal/observability"
)

// metricsBearerGate requires Authorization: Bearer <token> (trimmed; constant-time compare).
func metricsBearerGate(token string, h http.Handler) http.Handler {
	return observability.ScrapeBearerGate(token, h)
}
