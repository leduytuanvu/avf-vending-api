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

func TestMapCommercePersistenceErrForOp_sqlStates(t *testing.T) {
	t.Parallel()
	cases := []struct {
		op      CommerceOperation
		code    string
		grpc    codes.Code
		message string
	}{
		{OpCreateQuote, "22P02", codes.Internal, "quote_persistence_failed"},
		{OpCreateQuote, "22023", codes.Internal, "quote_persistence_failed"},
		{OpCreateQuote, "23505", codes.FailedPrecondition, "quote_conflict"},
		{OpCreateOrderFromQuote, "22P02", codes.Internal, "order_persistence_failed"},
		{OpCreatePaymentSession, "22P02", codes.Internal, "payment_session_persistence_failed"},
		{OpCreatePaymentSession, "23505", codes.FailedPrecondition, "payment_session_conflict"},
		{OpCreateQuote, "08006", codes.Unavailable, "commerce_backend_unavailable"},
		{OpCreateQuote, "57P03", codes.Unavailable, "commerce_backend_unavailable"},
	}
	for _, tc := range cases {
		err := &pgconn.PgError{Code: tc.code, Message: "test"}
		st, ok := status.FromError(mapCommercePersistenceErrForOp(tc.op, err, CommercePersistenceContext{}))
		if !ok {
			t.Fatalf("op=%s code=%s: expected status", tc.op, tc.code)
		}
		if st.Code() != tc.grpc || st.Message() != tc.message {
			t.Fatalf("op=%s code=%s: got code=%v msg=%q want code=%v msg=%q", tc.op, tc.code, st.Code(), st.Message(), tc.grpc, tc.message)
		}
	}
}

func TestMapCommerceGRPCErrForOp_createQuoteNotPaymentSessionReason(t *testing.T) {
	t.Parallel()
	err := &pgconn.PgError{Code: "22P02", Message: "invalid input syntax for type json", TableName: "checkout_quotes", ColumnName: "machine_pricing_snapshot"}
	st, ok := status.FromError(mapCommerceGRPCErrForOp(OpCreateQuote, err))
	if !ok || st.Code() != codes.Internal || st.Message() != "quote_persistence_failed" {
		t.Fatalf("got code=%v msg=%q ok=%v", st.Code(), st.Message(), ok)
	}
}
