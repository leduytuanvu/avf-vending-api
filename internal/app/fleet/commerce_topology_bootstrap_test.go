package fleet

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultBootstrapGridDimensions(t *testing.T) {
	rows, cols := DefaultBootstrapGridDimensions()
	require.Equal(t, int32(6), rows)
	require.Equal(t, int32(10), cols)
}

func TestDefaultCommerceTopologyConstants(t *testing.T) {
	require.Equal(t, "CAB-A", defaultCommerceCabinetCode)
	require.Equal(t, "default", defaultCommerceLayoutKey)
	require.Equal(t, int32(1), defaultCommerceLayoutRevision)
}
