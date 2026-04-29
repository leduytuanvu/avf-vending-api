package compliance

import "context"

type transportMetaKey struct{}

// TransportMeta carries non-domain correlation fields from the HTTP edge (set by handlers/middleware).
type TransportMeta struct {
	RequestID string
	TraceID   string
	IP        string
	UserAgent string
}

// WithTransportMeta attaches TransportMeta for downstream audit recording (auth, commerce, etc.).
func WithTransportMeta(ctx context.Context, m TransportMeta) context.Context {
	return context.WithValue(ctx, transportMetaKey{}, m)
}

// TransportMetaFromContext returns meta previously attached, or zero values.
func TransportMetaFromContext(ctx context.Context) TransportMeta {
	v, _ := ctx.Value(transportMetaKey{}).(TransportMeta)
	return v
}
