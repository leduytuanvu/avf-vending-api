package machineidempotency

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/pgjson"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	DefaultTTL                       = 24 * time.Hour
	DefaultInProgressStale           = 2 * time.Minute
	ErrMsgIdempotencyPayloadMismatch = "idempotency_payload_mismatch"
	ErrMsgIdempotencyFinalizeFailed  = "machine_idempotency_finalize_failed"
)

// Ledger persists machine mutation idempotency rows in PostgreSQL (source of truth).
type Ledger struct {
	q               *db.Queries
	audit           compliance.EnterpriseRecorder
	idempotencyTTL  time.Duration
	staleInProgress time.Duration
}

// NewLedger constructs a Ledger backed by sqlc queries.
func NewLedger(pool *pgxpool.Pool, audit compliance.EnterpriseRecorder) *Ledger {
	if pool == nil {
		return nil
	}
	return &Ledger{
		q:               db.New(pool),
		audit:           audit,
		idempotencyTTL:  DefaultTTL,
		staleInProgress: DefaultInProgressStale,
	}
}

// BeginMutation inserts or refreshes the idempotency row and returns a replay response when applicable.
// newResponse builds an empty protobuf for the operation when decoding stored JSON snapshots.
func (l *Ledger) BeginMutation(ctx context.Context, claims plauth.MachineAccessClaims, operation, key string, requestHash []byte, traceID string, newResponse func(operation string) proto.Message) (db.UpsertMachineIdempotencyKeyRow, proto.Message, error) {
	if l == nil || l.q == nil {
		return db.UpsertMachineIdempotencyKeyRow{}, nil, status.Error(codes.Internal, "machine idempotency not configured")
	}
	key = strings.TrimSpace(key)
	now := time.Now().UTC()
	exp := now.Add(l.idempotencyTTL)
	if l.idempotencyTTL <= 0 {
		exp = now.Add(DefaultTTL)
	}

	var row db.UpsertMachineIdempotencyKeyRow
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		row, err = l.q.UpsertMachineIdempotencyKey(ctx, db.UpsertMachineIdempotencyKeyParams{
			MachineID:      claims.MachineID,
			Operation:      operation,
			IdempotencyKey: key,
			RequestHash:    requestHash,
			ExpiresAt:      exp,
			TraceID:        traceID,
		})
		if err != nil {
			return db.UpsertMachineIdempotencyKeyRow{}, nil, status.Error(codes.Internal, "machine idempotency lookup failed")
		}
		staleAge := l.staleInProgress
		if staleAge <= 0 {
			staleAge = DefaultInProgressStale
		}
		cutoff := now.Add(-staleAge)
		if strings.EqualFold(strings.TrimSpace(row.Status), "in_progress") && !row.Inserted && row.LastSeenAt.Before(cutoff) {
			_ = l.q.DeleteStaleMachineIdempotencyInProgress(ctx, db.DeleteStaleMachineIdempotencyInProgressParams{
				MachineID:      claims.MachineID,
				Operation:      operation,
				IdempotencyKey: key,
				LastSeenAt:     cutoff,
			})
			continue
		}
		break
	}

	if now.After(row.ExpiresAt) {
		return row, nil, status.Error(codes.FailedPrecondition, "idempotency key expired")
	}
	if !bytes.Equal(row.RequestHash, requestHash) {
		_ = l.q.MarkMachineIdempotencyConflict(ctx, db.MarkMachineIdempotencyConflictParams{
			MachineID:      claims.MachineID,
			Operation:      operation,
			IdempotencyKey: key,
		})
		l.recordAudit(ctx, claims, compliance.ActionMachineIdempotencyConflict, operation, key, map[string]any{"status": row.Status})
		return row, nil, status.Error(codes.FailedPrecondition, ErrMsgIdempotencyPayloadMismatch)
	}

	switch strings.ToLower(strings.TrimSpace(row.Status)) {
	case "succeeded":
		if len(row.ResponseSnapshot) == 0 {
			return row, nil, status.Error(codes.FailedPrecondition, "idempotency response unavailable")
		}
		resp := newResponse(operation)
		if resp == nil {
			return row, nil, status.Error(codes.Internal, "machine idempotency response type unavailable")
		}
		if err := protojson.Unmarshal(row.ResponseSnapshot, resp); err != nil {
			return row, nil, status.Error(codes.Internal, "machine idempotency response decode failed")
		}
		l.recordAudit(ctx, claims, compliance.ActionMachineIdempotencyReplayed, operation, key, nil)
		return row, resp, nil
	case "in_progress":
		if row.Inserted {
			return row, nil, nil
		}
		staleAge := l.staleInProgress
		if staleAge <= 0 {
			staleAge = DefaultInProgressStale
		}
		if time.Since(row.LastSeenAt) > staleAge {
			return row, nil, status.Error(codes.FailedPrecondition, "idempotent operation still recovering; retry later")
		}
		return row, nil, status.Error(codes.Aborted, "idempotent operation already in progress")
	case "failed":
		return row, nil, status.Error(codes.FailedPrecondition, "idempotent operation failed_final; use a new idempotency key")
	case "conflict":
		return row, nil, status.Error(codes.FailedPrecondition, ErrMsgIdempotencyPayloadMismatch)
	case "expired":
		return row, nil, status.Error(codes.FailedPrecondition, "idempotency key expired")
	default:
		return row, nil, status.Error(codes.FailedPrecondition, "idempotency key unavailable")
	}
}

// MarkSucceeded stores the JSON snapshot after the handler succeeds.
func (l *Ledger) MarkSucceeded(ctx context.Context, claims plauth.MachineAccessClaims, operation, key string, resp proto.Message, traceID string) error {
	if l == nil || l.q == nil {
		return status.Error(codes.Internal, "machine idempotency not configured")
	}
	snap, err := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(resp)
	if err != nil {
		return status.Error(codes.Internal, "machine idempotency response encode failed")
	}
	key = strings.TrimSpace(key)
	if _, err := l.q.MarkMachineIdempotencySucceeded(ctx, db.MarkMachineIdempotencySucceededParams{
		ResponseSnapshot: pgjson.RequiredString(snap),
		TraceID:          traceID,
		MachineID:        claims.MachineID,
		Operation:        operation,
		IdempotencyKey:   key,
	}); err != nil {
		return wrapFinalizeStoreError(err)
	}
	return nil
}

// MarkFailed marks in-progress rows failed after handler errors.
func (l *Ledger) MarkFailed(ctx context.Context, claims plauth.MachineAccessClaims, operation, key, traceID string) error {
	if l == nil || l.q == nil {
		return nil
	}
	return l.q.MarkMachineIdempotencyFailed(ctx, db.MarkMachineIdempotencyFailedParams{TraceID: traceID,

		MachineID:      claims.MachineID,
		Operation:      operation,
		IdempotencyKey: strings.TrimSpace(key),
	})
}

func (l *Ledger) recordAudit(ctx context.Context, claims plauth.MachineAccessClaims, action, operation, key string, meta map[string]any) {
	if l == nil || l.audit == nil {
		return
	}
	if meta == nil {
		meta = map[string]any{}
	}
	meta["operation"] = operation
	meta["idempotency_key"] = key
	md, _ := json.Marshal(meta)
	mid := claims.MachineID.String()
	_ = l.audit.Record(ctx, compliance.EnterpriseAuditRecord{
		ActorType:    compliance.ActorMachine,
		ActorID:      &mid,
		Action:       action,
		ResourceType: "machine_idempotency_key",
		ResourceID:   &mid,
		Metadata:     md,
	})
}

type finalizeStoreError struct {
	SQLState string
	err      error
}

func (e *finalizeStoreError) Error() string {
	if e == nil {
		return "machine idempotency response store failed"
	}
	if e.SQLState != "" {
		return "machine idempotency response store failed sqlstate=" + e.SQLState
	}
	return "machine idempotency response store failed"
}

func (e *finalizeStoreError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func wrapFinalizeStoreError(err error) error {
	if err == nil {
		return nil
	}
	out := &finalizeStoreError{err: err}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		out.SQLState = pgErr.Code
	}
	return out
}
