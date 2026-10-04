package grpcserver

import (
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
)

type offlineEventPersistenceContext struct {
	MachineID       uuid.UUID
	OfflineSequence int64
	ClientEventID   string
	IdempotencyKey  string
	EventType       string
}

func logMachineOfflineInsertError(ctx offlineEventPersistenceContext, err error) {
	if err == nil {
		return
	}
	fields := []zap.Field{
		zap.String("event", "MACHINE_OFFLINE_INSERT_ERROR"),
		zap.String("machine_id", ctx.MachineID.String()),
		zap.Int64("offline_sequence", ctx.OfflineSequence),
		zap.String("event_type", ctx.EventType),
		zap.String("client_event_id", strings.TrimSpace(ctx.ClientEventID)),
		zap.String("idempotency_key", strings.TrimSpace(ctx.IdempotencyKey)),
		zap.String("error", err.Error()),
	}
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe != nil {
		fields = append(fields,
			zap.String("sqlstate", pe.Code),
			zap.String("severity", pe.Severity),
			zap.String("table_name", pe.TableName),
			zap.String("column_name", pe.ColumnName),
			zap.String("constraint_name", pe.ConstraintName),
			zap.String("detail", pe.Detail),
			zap.String("pg_message", pe.Message),
		)
	}
	zap.L().Error("MACHINE_OFFLINE_INSERT_ERROR", fields...)
}

// offlineEventInsertFailedReason returns a safe client-facing REJECTED reason.
func offlineEventInsertFailedReason(err error) string {
	const prefix = "offline event insert failed"
	if err == nil {
		return prefix
	}
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe != nil {
		parts := []string{prefix, pe.Code}
		if c := strings.TrimSpace(pe.ConstraintName); c != "" {
			parts = append(parts, c)
		} else if m := strings.TrimSpace(pe.Message); m != "" {
			parts = append(parts, truncateForClientReason(m, 120))
		}
		return strings.Join(parts, ": ")
	}
	return prefix + ": " + truncateForClientReason(err.Error(), 120)
}

func truncateForClientReason(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
