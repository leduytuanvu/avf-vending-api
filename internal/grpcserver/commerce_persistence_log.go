package grpcserver

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
)

// CommerceOperation identifies which commerce RPC persistence failed.
type CommerceOperation string

const (
	OpCreateQuote          CommerceOperation = "create_quote"
	OpCreateOrderFromQuote CommerceOperation = "create_order_from_quote"
	OpCreateOrder          CommerceOperation = "create_order"
	OpCreatePaymentSession CommerceOperation = "create_payment_session"
	OpGenericCommerce      CommerceOperation = "commerce"
)

// CommercePersistenceContext carries safe correlation fields for persistence error logs.
type CommercePersistenceContext struct {
	MachineID      uuid.UUID
	QuoteID        uuid.UUID
	OrderID        uuid.UUID
	IdempotencyKey string
	SnapshotBytes  int
}

func logCommercePersistenceError(op CommerceOperation, pgErr *pgconn.PgError, ctx CommercePersistenceContext) {
	if pgErr == nil {
		return
	}
	fields := []zap.Field{
		zap.String("event", "COMMERCE_PERSISTENCE_ERROR"),
		zap.String("operation", string(op)),
		zap.String("sqlstate", pgErr.Code),
		zap.String("severity", pgErr.Severity),
		zap.String("table_name", pgErr.TableName),
		zap.String("column_name", pgErr.ColumnName),
		zap.String("constraint_name", pgErr.ConstraintName),
		zap.String("routine", pgErr.Routine),
		zap.String("message", pgErr.Message),
	}
	if ctx.MachineID != uuid.Nil {
		fields = append(fields, zap.String("machine_id", ctx.MachineID.String()))
	}
	if ctx.QuoteID != uuid.Nil {
		fields = append(fields, zap.String("quote_id", ctx.QuoteID.String()))
	}
	if ctx.OrderID != uuid.Nil {
		fields = append(fields, zap.String("order_id", ctx.OrderID.String()))
	}
	if key := hashIdempotencyKey(ctx.IdempotencyKey); key != "" {
		fields = append(fields, zap.String("idempotency_key_sha256", key))
	}
	if ctx.SnapshotBytes > 0 {
		fields = append(fields, zap.Int("snapshot_bytes", ctx.SnapshotBytes))
	}
	zap.L().Error("COMMERCE_PERSISTENCE_ERROR", fields...)
}

func hashIdempotencyKey(key string) string {
	trim := strings.TrimSpace(key)
	if trim == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(trim))
	return hex.EncodeToString(sum[:])[:12]
}

func persistenceReasonForOp(op CommerceOperation, suffix string) string {
	switch op {
	case OpCreateQuote:
		return "quote_" + suffix
	case OpCreateOrderFromQuote, OpCreateOrder:
		return "order_" + suffix
	case OpCreatePaymentSession:
		return "payment_session_" + suffix
	default:
		return "commerce_" + suffix
	}
}
