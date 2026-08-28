package layoutassignment

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateGridDimensions_default6x10(t *testing.T) {
	rows, cols := DefaultCreationDimensions()
	require.Equal(t, int32(6), rows)
	require.Equal(t, int32(10), cols)
	require.NoError(t, ValidateGridDimensions(rows, cols))
}

func TestValidateMachineTypeLaneBound_tcnRejects90Slots(t *testing.T) {
	err := ValidateMachineTypeLaneBound("tcn", 9, 10)
	require.ErrorIs(t, err, ErrExceedsHardwareLaneCapacity)
}

func TestValidateMachineTypeLaneBound_tcnAccepts80Slots(t *testing.T) {
	require.NoError(t, ValidateMachineTypeLaneBound("tcn", 8, 10))
}

func TestDeriveSyncStatus_driftOnSourceMismatch(t *testing.T) {
	server := SourceServer
	local := SourceLocal
	fp := "abc"
	status := DeriveSyncStatus(&server, &local, &fp, &fp, nil)
	require.Equal(t, SyncStatusDrift, status)
}
