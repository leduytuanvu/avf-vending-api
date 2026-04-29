package httpserver

import (
	"context"
	"net/http"
	"strings"
	"time"

	appmw "github.com/avf/avf-vending-api/internal/middleware"
	"github.com/avf/avf-vending-api/internal/observability"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func traceMiddleware() func(http.Handler) http.Handler {
	tracer := otel.Tracer("github.com/avf/avf-vending-api/internal/httpserver")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagationHeaderCarrier{header: r.Header})
			spanName := r.Method + " " + r.URL.Path
			ctx, span := tracer.Start(ctx, spanName)
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			defer func() {
				route := routeLabel(r.WithContext(ctx))
				span.SetAttributes(
					attribute.String("http.request.method", r.Method),
					attribute.String("http.route", route),
					attribute.String("url.path", r.URL.Path),
					attribute.Int("http.response.status_code", ww.Status()),
				)
				span.SetName(r.Method + " " + route)
				if ww.Status() >= http.StatusInternalServerError {
					span.SetStatus(codes.Error, http.StatusText(ww.Status()))
				} else {
					span.SetStatus(codes.Ok, http.StatusText(ww.Status()))
				}
				span.End()
			}()
			next.ServeHTTP(ww, r.WithContext(ctx))
		})
	}
}

type propagationHeaderCarrier struct {
	header http.Header
}

func (c propagationHeaderCarrier) Get(key string) string {
	return c.header.Get(key)
}

func (c propagationHeaderCarrier) Set(key string, value string) {
	c.header.Set(key, value)
}

func (c propagationHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c.header))
	for k := range c.header {
		keys = append(keys, k)
	}
	return keys
}

func requestObservabilityMiddleware(base *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := attachRequestMetadata(r.Context(), r)
			log := observability.EnrichLogger(base, ctx)
			next.ServeHTTP(w, r.WithContext(observability.WithLogger(ctx, log)))
		})
	}
}

func authObservabilityMiddleware(base *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if p, ok := auth.PrincipalFromContext(ctx); ok {
				if p.HasOrganization() {
					ctx = observability.WithOrganizationID(ctx, p.OrganizationID.String())
				}
				if p.TechnicianID != uuid.Nil {
					ctx = observability.WithOperatorID(ctx, p.TechnicianID.String())
				}
			}
			log := observability.EnrichLogger(base, ctx)
			next.ServeHTTP(w, r.WithContext(observability.WithLogger(ctx, log)))
		})
	}
}

func requestLoggingMiddleware(base *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			ctx := attachRequestMetadata(r.Context(), r)
			log := observability.EnrichLogger(observability.LoggerFromContext(r.Context(), base), ctx)
			fields := observability.ContextFields(ctx)
			if cid := correlationIDFromContext(ctx); cid != "" {
				fields = append(fields, zap.String("correlation_id", cid))
			}
			fields = append(fields,
				zap.String("method", r.Method),
				zap.String("route", routeLabel(r)),
				zap.String("path", r.URL.Path),
				zap.Int("status", ww.Status()),
				zap.Duration("duration", time.Since(start)),
				zap.Int("bytes_written", ww.BytesWritten()),
			)
			switch {
			case ww.Status() >= http.StatusInternalServerError:
				log.Error("http_request", fields...)
			default:
				log.Debug("http_request", fields...)
			}
		})
	}
}

func attachRequestMetadata(ctx context.Context, r *http.Request) context.Context {
	for _, pair := range []struct {
		param string
		set   func(context.Context, string) context.Context
	}{
		{param: "machineId", set: observability.WithMachineID},
		{param: "orgId", set: observability.WithOrganizationID},
		{param: "organizationId", set: observability.WithOrganizationID},
		{param: "orderId", set: observability.WithOrderID},
		{param: "paymentId", set: observability.WithPaymentID},
		{param: "vendId", set: observability.WithVendID},
		{param: "commandId", set: observability.WithCommandID},
	} {
		if v := strings.TrimSpace(chi.URLParam(r, pair.param)); v != "" {
			ctx = pair.set(ctx, v)
		}
	}

	for _, header := range []struct {
		names []string
		set   func(context.Context, string) context.Context
	}{
		{names: []string{"X-Machine-ID", "X-Machine-Id"}, set: observability.WithMachineID},
		{names: []string{"X-Organization-ID", "X-Organization-Id"}, set: observability.WithOrganizationID},
		{names: []string{"X-Operator-ID", "X-Operator-Id"}, set: observability.WithOperatorID},
		{names: []string{"X-Order-ID", "X-Order-Id"}, set: observability.WithOrderID},
		{names: []string{"X-Payment-ID", "X-Payment-Id"}, set: observability.WithPaymentID},
		{names: []string{"X-Vend-ID", "X-Vend-Id"}, set: observability.WithVendID},
		{names: []string{"X-Command-ID", "X-Command-Id"}, set: observability.WithCommandID},
	} {
		if v := firstRequestHeader(r, header.names...); v != "" {
			ctx = header.set(ctx, v)
		}
	}

	if v := strings.TrimSpace(r.URL.Query().Get("organization_id")); v != "" {
		ctx = observability.WithOrganizationID(ctx, v)
	}
	return ctx
}

func firstRequestHeader(r *http.Request, names ...string) string {
	for _, name := range names {
		if v := strings.TrimSpace(r.Header.Get(name)); v != "" {
			return v
		}
	}
	return ""
}

func correlationIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		id := sc.TraceID().String()
		if id != "" && id != "00000000000000000000000000000000" {
			return id
		}
	}
	return appmw.RequestIDFromContext(ctx)
}

func logAPIError(ctx context.Context, status int, code, message string, details map[string]any) {
	log := observability.LoggerFromContext(ctx, zap.NewNop())
	fields := observability.ContextFields(ctx)
	if cid := correlationIDFromContext(ctx); cid != "" {
		fields = append(fields, zap.String("correlation_id", cid))
	}
	fields = append(fields,
		zap.Int("status", status),
		zap.String("error_code", code),
		zap.String("error_message", message),
	)
	if len(details) > 0 {
		fields = append(fields, zap.Any("details", details))
	}
	switch {
	case status >= http.StatusInternalServerError:
		log.Error("api_error_response", fields...)
	case status >= http.StatusBadRequest:
		log.Debug("api_error_response", fields...)
	default:
		log.Debug("api_error_response", fields...)
	}
}
