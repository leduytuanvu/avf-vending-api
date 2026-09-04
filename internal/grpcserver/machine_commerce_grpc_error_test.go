package grpcserver

import (
	"errors"
	"testing"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestMapCommerceGRPCErr_knownErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		err  error
		code codes.Code
	}{
		{appcommerce.ErrInvalidArgument, codes.InvalidArgument},
		{appcommerce.ErrNotFound, codes.NotFound},
		{appcommerce.ErrOrgMismatch, codes.PermissionDenied},
		{appcommerce.ErrIllegalTransition, codes.FailedPrecondition},
		{appcommerce.ErrPaymentNotSettled, codes.FailedPrecondition},
		{appcommerce.ErrCancelNotAllowed, codes.FailedPrecondition},
		{appcommerce.ErrNotConfigured, codes.Unavailable},
		{appcommerce.ErrIdempotencyPayloadConflict, codes.Aborted},
	}
	for _, tc := range cases {
		st, ok := status.FromError(mapCommerceGRPCErr(tc.err))
		if !ok {
			t.Fatalf("expected status error for %v", tc.err)
		}
		if st.Code() != tc.code {
			t.Fatalf("got %v want %v for %v", st.Code(), tc.code, tc.err)
		}
	}
}

func TestMapCommerceGRPCErr_insufficientStock(t *testing.T) {
	t.Parallel()
	st, ok := status.FromError(mapCommerceGRPCErr(errors.New("insufficient stock for slot")))
	if !ok || st.Code() != codes.ResourceExhausted {
		t.Fatalf("got %v ok=%v", st, ok)
	}
}

func TestMapCommercePersistenceErr_sqlState22P02(t *testing.T) {
	t.Parallel()
	err := &pgconn.PgError{Code: "22P02", Message: "invalid input syntax for type json"}
	st, ok := status.FromError(mapCommercePersistenceErr(err))
	if !ok || st.Code() != codes.Internal || st.Message() != "payment_session_persistence_failed" {
		t.Fatalf("got code=%v msg=%q ok=%v", st.Code(), st.Message(), ok)
	}
}
