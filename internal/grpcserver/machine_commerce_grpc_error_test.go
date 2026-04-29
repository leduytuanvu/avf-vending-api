package grpcserver

import (
	"errors"
	"testing"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
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
