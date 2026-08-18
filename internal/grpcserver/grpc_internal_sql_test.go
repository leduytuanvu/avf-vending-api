package grpcserver

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGrpcInternalSQL_IncludesSQLState(t *testing.T) {
	err := grpcInternalSQL("telemetry append failed", &pgconn.PgError{Code: "22P02", Message: "invalid input syntax for type json"})
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.Internal, st.Code())
	require.Equal(t, "telemetry append failed sqlstate=22P02", st.Message())
}

func TestGrpcInternalSQL_PlainWhenNotPg(t *testing.T) {
	err := grpcInternalSQL("telemetry append failed", errors.New("boom"))
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, "telemetry append failed", st.Message())
}
