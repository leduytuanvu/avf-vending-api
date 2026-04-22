package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	appmw "github.com/avf/avf-vending-api/internal/middleware"
	"github.com/avf/avf-vending-api/internal/observability"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func TestRequestObservabilityMiddleware_PropagatesHeaderAndTraceFields(t *testing.T) {
	t.Parallel()

	r := chi.NewRouter()
	r.Use(appmw.RequestID)
	r.Use(traceMiddleware())
	r.Use(requestObservabilityMiddleware(zap.NewNop()))
	r.Get("/machines/{machineId}/orders/{orderId}", func(w http.ResponseWriter, r *http.Request) {
		sc := trace.SpanContextFromContext(r.Context())
		writeJSON(w, http.StatusOK, map[string]any{
			"request_id": appmw.RequestIDFromContext(r.Context()),
			"machine_id": observability.MachineIDFromContext(r.Context()),
			"order_id":   observability.OrderIDFromContext(r.Context()),
			"trace_id":   sc.TraceID().String(),
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/machines/m-1/orders/o-1", nil)
	req.Header.Set("X-Request-ID", "req-1")
	req.Header.Set("X-Machine-ID", "m-1")
	req.Header.Set("X-Order-ID", "o-1")
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["request_id"] != "req-1" {
		t.Fatalf("request_id=%v", got["request_id"])
	}
	if got["machine_id"] != "m-1" {
		t.Fatalf("machine_id=%v", got["machine_id"])
	}
	if got["order_id"] != "o-1" {
		t.Fatalf("order_id=%v", got["order_id"])
	}
	if got["trace_id"] == "" {
		t.Fatal("expected trace_id")
	}
}

func TestAuthObservabilityMiddleware_PropagatesPrincipalFields(t *testing.T) {
	t.Parallel()

	orgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	techID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	r := chi.NewRouter()
	r.With(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.WithPrincipal(r.Context(), auth.Principal{
				OrganizationID: orgID,
				TechnicianID:   techID,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}, authObservabilityMiddleware(zap.NewNop())).Get("/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"organization_id": observability.OrganizationIDFromContext(r.Context()),
			"operator_id":     observability.OperatorIDFromContext(r.Context()),
		})
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["organization_id"] != orgID.String() {
		t.Fatalf("organization_id=%v", got["organization_id"])
	}
	if got["operator_id"] != techID.String() {
		t.Fatalf("operator_id=%v", got["operator_id"])
	}
}
