package observability

import (
	"context"

	appmw "github.com/avf/avf-vending-api/internal/middleware"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type loggerCtxKey struct{}

type stringCtxKey string

const (
	ctxMachineID      stringCtxKey = "machine_id"
	ctxOrganizationID stringCtxKey = "organization_id"
	ctxOperatorID     stringCtxKey = "operator_id"
	ctxOrderID        stringCtxKey = "order_id"
	ctxPaymentID      stringCtxKey = "payment_id"
	ctxVendID         stringCtxKey = "vend_id"
	ctxCommandID      stringCtxKey = "command_id"
)

func withStringValue(ctx context.Context, key stringCtxKey, value string) context.Context {
	if ctx == nil || value == "" {
		return ctx
	}
	return context.WithValue(ctx, key, value)
}

func stringValueFromContext(ctx context.Context, key stringCtxKey) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(key).(string)
	return v
}

// WithLogger attaches a request or operation-scoped logger to context.
func WithLogger(ctx context.Context, log *zap.Logger) context.Context {
	if ctx == nil || log == nil {
		return ctx
	}
	return context.WithValue(ctx, loggerCtxKey{}, log)
}

// LoggerFromContext returns the scoped logger when present, else fallback.
func LoggerFromContext(ctx context.Context, fallback *zap.Logger) *zap.Logger {
	if ctx != nil {
		if log, ok := ctx.Value(loggerCtxKey{}).(*zap.Logger); ok && log != nil {
			return log
		}
	}
	return fallback
}

func WithMachineID(ctx context.Context, machineID string) context.Context {
	return withStringValue(ctx, ctxMachineID, machineID)
}

func MachineIDFromContext(ctx context.Context) string {
	if v := stringValueFromContext(ctx, ctxMachineID); v != "" {
		return v
	}
	if v, ok := appmw.MachineIDFromContext(ctx); ok {
		return v
	}
	return ""
}

func WithOrganizationID(ctx context.Context, organizationID string) context.Context {
	return withStringValue(ctx, ctxOrganizationID, organizationID)
}

func OrganizationIDFromContext(ctx context.Context) string {
	if v := stringValueFromContext(ctx, ctxOrganizationID); v != "" {
		return v
	}
	if p, ok := auth.PrincipalFromContext(ctx); ok && p.OrganizationID != uuid.Nil {
		return p.OrganizationID.String()
	}
	return ""
}

func WithOperatorID(ctx context.Context, operatorID string) context.Context {
	return withStringValue(ctx, ctxOperatorID, operatorID)
}

func OperatorIDFromContext(ctx context.Context) string {
	if v := stringValueFromContext(ctx, ctxOperatorID); v != "" {
		return v
	}
	if p, ok := auth.PrincipalFromContext(ctx); ok && p.TechnicianID != uuid.Nil {
		return p.TechnicianID.String()
	}
	return ""
}

func WithOrderID(ctx context.Context, orderID string) context.Context {
	return withStringValue(ctx, ctxOrderID, orderID)
}

func OrderIDFromContext(ctx context.Context) string {
	return stringValueFromContext(ctx, ctxOrderID)
}

func WithPaymentID(ctx context.Context, paymentID string) context.Context {
	return withStringValue(ctx, ctxPaymentID, paymentID)
}

func PaymentIDFromContext(ctx context.Context) string {
	return stringValueFromContext(ctx, ctxPaymentID)
}

func WithVendID(ctx context.Context, vendID string) context.Context {
	return withStringValue(ctx, ctxVendID, vendID)
}

func VendIDFromContext(ctx context.Context) string {
	return stringValueFromContext(ctx, ctxVendID)
}

func WithCommandID(ctx context.Context, commandID string) context.Context {
	return withStringValue(ctx, ctxCommandID, commandID)
}

func CommandIDFromContext(ctx context.Context) string {
	return stringValueFromContext(ctx, ctxCommandID)
}

// ContextFields returns stable correlation fields for structured logs when available.
func ContextFields(ctx context.Context) []zap.Field {
	fields := make([]zap.Field, 0, 9)
	if ctx == nil {
		return fields
	}
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		fields = append(fields, zap.String("trace_id", sc.TraceID().String()))
	}
	if v := appmw.RequestIDFromContext(ctx); v != "" {
		fields = append(fields, zap.String("request_id", v))
	}
	if v := MachineIDFromContext(ctx); v != "" {
		fields = append(fields, zap.String("machine_id", v))
	}
	if v := OrganizationIDFromContext(ctx); v != "" {
		fields = append(fields, zap.String("organization_id", v))
	}
	if v := OperatorIDFromContext(ctx); v != "" {
		fields = append(fields, zap.String("operator_id", v))
	}
	if v := OrderIDFromContext(ctx); v != "" {
		fields = append(fields, zap.String("order_id", v))
	}
	if v := PaymentIDFromContext(ctx); v != "" {
		fields = append(fields, zap.String("payment_id", v))
	}
	if v := VendIDFromContext(ctx); v != "" {
		fields = append(fields, zap.String("vend_id", v))
	}
	if v := CommandIDFromContext(ctx); v != "" {
		fields = append(fields, zap.String("command_id", v))
	}
	return fields
}

// EnrichLogger adds stable correlation fields from context onto a logger.
func EnrichLogger(log *zap.Logger, ctx context.Context) *zap.Logger {
	if log == nil {
		return nil
	}
	fields := ContextFields(ctx)
	if len(fields) == 0 {
		return log
	}
	return log.With(fields...)
}
