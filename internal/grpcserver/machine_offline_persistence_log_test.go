package grpcserver

import (
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestOfflineEventInsertFailedReason_pgErrorIncludesCodeAndConstraint(t *testing.T) {
	err := &pgconn.PgError{
		Code:            "23505",
		ConstraintName:  "ux_machine_offline_client_event_id",
		Message:         "duplicate key value violates unique constraint",
	}
	reason := offlineEventInsertFailedReason(err)
	require.Contains(t, reason, "offline event insert failed")
	require.Contains(t, reason, "23505")
	require.Contains(t, reason, "ux_machine_offline_client_event_id")
}

func TestOfflineEventInsertFailedReason_nonPgError(t *testing.T) {
	reason := offlineEventInsertFailedReason(errTestPlain("connection reset"))
	require.Contains(t, reason, "offline event insert failed")
	require.Contains(t, reason, "connection reset")
}

type errTestPlain string

func (e errTestPlain) Error() string { return string(e) }
