package grpcserver

import (
	"context"
	"strings"
)

type ctxKey int

const skipCredentialUsedTouchKey ctxKey = 1

func withSkipCredentialUsedTouch(ctx context.Context) context.Context {
	return context.WithValue(ctx, skipCredentialUsedTouchKey, true)
}

func skipCredentialUsedTouchFromContext(ctx context.Context) bool {
	v, ok := ctx.Value(skipCredentialUsedTouchKey).(bool)
	return ok && v
}

func skipCredentialUsedTouchForMethod(fullMethod string) bool {
	m := strings.ToLower(fullMethod)
	switch {
	case strings.Contains(m, "heartbeat"):
		return true
	case strings.Contains(m, "checkin"):
		return true
	case strings.Contains(m, "telemetry"):
		return true
	case strings.Contains(m, "pushtelemetry"):
		return true
	default:
		return false
	}
}
