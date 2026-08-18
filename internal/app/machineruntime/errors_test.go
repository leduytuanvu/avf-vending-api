package machineruntime

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestWrapRuntimeStage_IncludesSQLState(t *testing.T) {
	err := wrapRuntimeStage("start runtime session", "insert_runtime_session", "StartMachineRuntimeAppSession",
		&pgconn.PgError{Code: "22P02", Message: "invalid input syntax for type json", TableName: "machine_runtime_app_sessions", ColumnName: "blockers"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "stage=insert_runtime_session")
	require.Contains(t, err.Error(), "sqlstate=22P02")
	require.NotContains(t, err.Error(), "Bearer")
	stage, sqlName, sqlstate, next, ok := RuntimeSessionDiag(err)
	require.True(t, ok)
	require.Equal(t, "insert_runtime_session", stage)
	require.Equal(t, "StartMachineRuntimeAppSession", sqlName)
	require.Equal(t, "22P02", sqlstate)
	require.Contains(t, next, "text")
}

func TestJSONText_RejectsMalformed(t *testing.T) {
	_, err := jsonArrayText(json.RawMessage("not-json"))
	require.ErrorIs(t, err, ErrInvalidRuntimeJSON)
	s, err := jsonArrayText(nil)
	require.NoError(t, err)
	require.Equal(t, "[]", s)
}

func TestLooksLikeSecretKey(t *testing.T) {
	require.True(t, looksLikeSecretKey("Authorization"))
	require.True(t, looksLikeSecretKey("jwt"))
	require.False(t, looksLikeSecretKey("sqlstate"))
	require.False(t, errors.Is(ErrInvalidRuntimeJSON, errors.New("other")))
}
