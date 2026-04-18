package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type requestIDCtxKey struct{}

type correlationIDCtxKey struct{}

type idempotencyKeyCtxKey struct{}

type actorIDCtxKey struct{}

type machineIDCtxKey struct{}

// RequestID ensures every request has an X-Request-ID (generated if absent) and
// propagates optional correlation, idempotency, actor, and machine headers into context.
// Mount early in the chain; correlation defaults to the request ID when not supplied.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if id == "" {
			id = uuid.NewString()
		}

		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDCtxKey{}, id)

		corr := strings.TrimSpace(firstHeader(r, "X-Correlation-ID", "X-Correlation-Id"))
		if corr == "" {
			corr = id
		}
		ctx = context.WithValue(ctx, correlationIDCtxKey{}, corr)
		w.Header().Set("X-Correlation-ID", corr)

		if v := strings.TrimSpace(r.Header.Get("Idempotency-Key")); v != "" {
			ctx = context.WithValue(ctx, idempotencyKeyCtxKey{}, v)
		}
		if v := strings.TrimSpace(firstHeader(r, "X-Actor-Id", "X-Actor-ID")); v != "" {
			ctx = context.WithValue(ctx, actorIDCtxKey{}, v)
		}
		if v := strings.TrimSpace(firstHeader(r, "X-Machine-Id", "X-Machine-ID")); v != "" {
			ctx = context.WithValue(ctx, machineIDCtxKey{}, v)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func firstHeader(r *http.Request, names ...string) string {
	for _, n := range names {
		if v := r.Header.Get(n); v != "" {
			return v
		}
	}
	return ""
}

// RequestIDFromContext returns the request ID if present.
func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(requestIDCtxKey{}).(string)
	return v
}

// CorrelationIDFromContext returns the correlation ID (falls back to empty if unset).
func CorrelationIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(correlationIDCtxKey{}).(string)
	return v
}

// IdempotencyKeyFromContext returns the idempotency key when the client sent one.
func IdempotencyKeyFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(idempotencyKeyCtxKey{}).(string)
	return v, ok && v != ""
}

// ActorIDFromContext returns a caller actor identifier when provided (e.g. staff subject).
func ActorIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(actorIDCtxKey{}).(string)
	return v, ok && v != ""
}

// MachineIDFromContext returns a machine scope hint when provided (raw header string).
func MachineIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(machineIDCtxKey{}).(string)
	return v, ok && v != ""
}
