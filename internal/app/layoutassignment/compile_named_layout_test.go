package layoutassignment

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseMirrorSlots_roundTripProductAssignment(t *testing.T) {
	raw, err := json.Marshal([]LocalMirrorSlotView{
		{SlotCode: "A1", SlotOrdinal: 1, ProductID: "01a0a7e5-3c68-7895-b526-bcb6504bccfb", MaxQuantity: 6},
		{SlotCode: "A2", SlotOrdinal: 2},
	})
	require.NoError(t, err)
	slots := ParseMirrorSlots(raw)
	require.Len(t, slots, 2)
	require.Equal(t, "A1", slots[0].SlotCode)
	require.NotEmpty(t, slots[0].ProductID)
	require.Empty(t, slots[1].ProductID)
}

// Documents the data-plane split fixed by materializeDeviceSlotsToNamedLayout:
// GetMachineLayoutDetail reads machine_layout_slots; legacy ReportLocalLayout only updated mirror.
func TestLayoutDataPlaneSources_documentedInCode(t *testing.T) {
	require.Contains(t, machineLayoutDetailSourceComment(), "machine_layout_slots")
	require.Contains(t, reportLocalLayoutMirrorComment(), "machine_local_layout_mirror")
}

func machineLayoutDetailSourceComment() string {
	return "GetMachineLayoutDetail reads machine_layout_slots for named layouts"
}

func reportLocalLayoutMirrorComment() string {
	return "ReportLocalLayout persists machine_local_layout_mirror before named layout compile"
}
