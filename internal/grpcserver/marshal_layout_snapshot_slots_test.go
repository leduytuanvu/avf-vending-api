package grpcserver

import (
	"encoding/json"
	"testing"

	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/stretchr/testify/require"
)

func TestMarshalLayoutSnapshotSlots_includesZeroInventoryAndDisabled(t *testing.T) {
	raw, err := marshalLayoutSnapshotSlots([]*machinev1.LayoutSnapshotSlot{{
		SlotCode:         "A1",
		MaxQuantity:      5,
		CurrentInventory: 0,
		Enabled:          false,
	}})
	require.NoError(t, err)

	var decoded []map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.Len(t, decoded, 1)
	require.Equal(t, float64(0), decoded[0]["currentInventory"])
	require.Equal(t, false, decoded[0]["enabled"])
	require.Contains(t, string(raw), `"currentInventory":0`)
	require.Contains(t, string(raw), `"enabled":false`)
}
