package grpcserver

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// machineMutationContext carries validated idempotency and provenance for machine-originated writes.
type machineMutationContext struct {
	IdempotencyKey    string
	ClientEventID     string
	ClientCreatedAt   time.Time
	OperatorSessionID *uuid.UUID
}

func parseMachineMutationContext(ctx context.Context, protoCtx *machinev1.IdempotencyContext) (machineMutationContext, error) {
	if protoCtx == nil {
		return machineMutationContext{}, status.Error(codes.InvalidArgument, "context is required")
	}
	md, _ := metadata.FromIncomingContext(ctx)
	metaKey := strings.TrimSpace(firstMetadata(md, "x-idempotency-key", "idempotency-key", "x-idempotencykey"))
	bodyKey := strings.TrimSpace(protoCtx.GetIdempotencyKey())
	if metaKey != "" && bodyKey != "" && metaKey != bodyKey {
		return machineMutationContext{}, status.Error(codes.InvalidArgument, "idempotency_key mismatch between metadata and body")
	}
	idem := metaKey
	if idem == "" {
		idem = bodyKey
	}
	if idem == "" {
		return machineMutationContext{}, status.Error(codes.InvalidArgument, "idempotency_key required")
	}
	ce := strings.TrimSpace(protoCtx.GetClientEventId())
	if ce == "" {
		return machineMutationContext{}, status.Error(codes.InvalidArgument, "client_event_id required")
	}
	ts := protoCtx.GetClientCreatedAt()
	if ts == nil || !ts.IsValid() {
		return machineMutationContext{}, status.Error(codes.InvalidArgument, "client_created_at required")
	}
	t := ts.AsTime().UTC()

	var opSid *uuid.UUID
	if v := strings.TrimSpace(protoCtx.GetOperatorSessionId()); v != "" {
		u, err := uuid.Parse(v)
		if err != nil || u == uuid.Nil {
			return machineMutationContext{}, status.Error(codes.InvalidArgument, "invalid operator_session_id")
		}
		opSid = &u
	}

	return machineMutationContext{
		IdempotencyKey:    idem,
		ClientEventID:     ce,
		ClientCreatedAt:   t,
		OperatorSessionID: opSid,
	}, nil
}

func validateOperatorSessionForMachine(ctx context.Context, q *db.Queries, claims plauth.MachineAccessClaims, sessionID *uuid.UUID) error {
	if sessionID == nil {
		return nil
	}
	sess, err := q.GetOperatorSessionByID(ctx, *sessionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return status.Error(codes.NotFound, "operator_session_not_found")
		}
		return status.Errorf(codes.Internal, "operator session lookup failed")
	}
	if sess.MachineID != claims.MachineID {
		return status.Error(codes.PermissionDenied, "operator_session machine mismatch")
	}
	if sess.OrganizationID != claims.OrganizationID {
		return status.Error(codes.PermissionDenied, "operator_session organization mismatch")
	}
	if !strings.EqualFold(strings.TrimSpace(sess.Status), "ACTIVE") {
		return status.Error(codes.FailedPrecondition, "operator_session not active")
	}
	return nil
}
