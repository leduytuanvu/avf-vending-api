package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	appmw "github.com/avf/avf-vending-api/internal/middleware"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestEnrichLogger_IncludesCorrelationFields(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "req-123")

	var ctx context.Context
	appmw.RequestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		ctx = r.Context()
	})).ServeHTTP(httptest.NewRecorder(), req)

	ctx = WithMachineID(ctx, "machine-1")
	ctx = WithOrderID(ctx, "order-1")
	ctx = WithPaymentID(ctx, "payment-1")
	ctx = WithCommandID(ctx, "command-1")
	ctx = auth.WithPrincipal(ctx, auth.Principal{
		OrganizationID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		TechnicianID:   uuid.MustParse("22222222-2222-2222-2222-222222222222"),
	})
	ctx = trace.ContextWithSpanContext(ctx, trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:     trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8},
		TraceFlags: trace.FlagsSampled,
	}))

	core, observed := observer.New(zap.InfoLevel)
	EnrichLogger(zap.New(core), ctx).Info("hello")

	entry := observed.All()[0]
	fields := entry.ContextMap()
	if got := fields["request_id"]; got != "req-123" {
		t.Fatalf("request_id=%v", got)
	}
	if got := fields["machine_id"]; got != "machine-1" {
		t.Fatalf("machine_id=%v", got)
	}
	if got := fields["organization_id"]; got != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("organization_id=%v", got)
	}
	if got := fields["operator_id"]; got != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("operator_id=%v", got)
	}
	if got := fields["order_id"]; got != "order-1" {
		t.Fatalf("order_id=%v", got)
	}
	if got := fields["payment_id"]; got != "payment-1" {
		t.Fatalf("payment_id=%v", got)
	}
	if got := fields["command_id"]; got != "command-1" {
		t.Fatalf("command_id=%v", got)
	}
	if got := fields["trace_id"]; got == "" {
		t.Fatal("expected trace_id")
	}
}
