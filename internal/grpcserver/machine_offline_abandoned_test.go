package grpcserver

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestIsMachineOfflineUniqueViolation(t *testing.T) {
	t.Parallel()
	require.False(t, isMachineOfflineUniqueViolation(nil))
	require.False(t, isMachineOfflineUniqueViolation(errors.New("offline cursor update failed after abandon")))
	require.True(t, isMachineOfflineUniqueViolation(&pgconn.PgError{
		Code:           "23505",
		ConstraintName: "ux_machine_offline_client_event_id",
	}))
}

func TestAbandonedOfflineInsertFailedReasonKeepsSeq(t *testing.T) {
	t.Parallel()
	err := &pgconn.PgError{
		Code:           "23505",
		ConstraintName: "ux_machine_offline_client_event_id",
	}
	reason := abandonedOfflineInsertFailedReason(322, err)
	require.Contains(t, reason, "abandoned offline_sequence 322 insert failed")
	require.Contains(t, reason, "23505")
	require.Contains(t, reason, "ux_machine_offline_client_event_id")
}
