package grpcserver

import (
	"context"
	"testing"

	"github.com/avf/avf-vending-api/internal/app/activation"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestMapRefreshMachineSessionError_knownActivationErrors(t *testing.T) {
	ctx := context.Background()
	if status.Code(mapRefreshMachineSessionError(ctx, activation.ErrRefreshInvalid)) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated for invalid refresh")
	}
	if status.Code(mapRefreshMachineSessionError(ctx, activation.ErrMachineNotEligible)) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied for ineligible machine")
	}
}

func TestMapRefreshMachineSessionError_transientPgCodes(t *testing.T) {
	ctx := context.Background()
	for _, code := range []string{"40001", "40P01", "55P03", "57014"} {
		err := mapRefreshMachineSessionError(ctx, &pgconn.PgError{Code: code, Message: "transient"})
		if status.Code(err) != codes.Unavailable {
			t.Fatalf("sqlstate %s: expected Unavailable, got %v", code, status.Code(err))
		}
	}
}

func TestMapRefreshMachineSessionError_uniqueViolation(t *testing.T) {
	ctx := context.Background()
	err := mapRefreshMachineSessionError(ctx, &pgconn.PgError{Code: "23505", ConstraintName: "machine_sessions_pkey"})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition, got %v", status.Code(err))
	}
}

func TestMapRefreshMachineSessionError_otherPgErrorIncludesSqlstate(t *testing.T) {
	ctx := context.Background()
	err := mapRefreshMachineSessionError(ctx, &pgconn.PgError{Code: "23503", Message: "fk violation"})
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", status.Code(err))
	}
	if status.Convert(err).Message() != "refresh_db_error:23503" {
		t.Fatalf("unexpected message: %v", status.Convert(err).Message())
	}
}

func TestMapRefreshMachineSessionError_deadlineExceededRetryable(t *testing.T) {
	ctx := context.Background()
	err := mapRefreshMachineSessionError(ctx, context.DeadlineExceeded)
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("expected Unavailable for deadline exceeded, got %v", status.Code(err))
	}
}
