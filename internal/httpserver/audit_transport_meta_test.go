package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/avf/avf-vending-api/internal/domain/compliance"
	appmw "github.com/avf/avf-vending-api/internal/middleware"
	"github.com/stretchr/testify/require"
)

func TestAuditTransportMetaMiddlewareAttachesRequestMeta(t *testing.T) {
	t.Parallel()

	var got compliance.TransportMeta
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = compliance.TransportMetaFromContext(r.Context())
	})
	handler := appmw.RequestID(auditTransportMetaMiddleware(next))

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/audit/events", nil)
	req.Header.Set("X-Request-ID", "req-123")
	req.Header.Set("X-Correlation-ID", "trace-456")
	req.Header.Set("User-Agent", "audit-agent")
	req.RemoteAddr = "203.0.113.20:54321"
	handler.ServeHTTP(httptest.NewRecorder(), req)

	require.Equal(t, "req-123", got.RequestID)
	require.Equal(t, "trace-456", got.TraceID)
	require.Equal(t, "203.0.113.20", got.IP)
	require.Equal(t, "audit-agent", got.UserAgent)
}
