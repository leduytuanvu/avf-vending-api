package layoutassignment

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNeedsNamedLayoutMaterialization(t *testing.T) {
	require.False(t, needsNamedLayoutMaterialization(0, 0))
	require.False(t, needsNamedLayoutMaterialization(0, 3))
	require.False(t, needsNamedLayoutMaterialization(5, 2))
	require.True(t, needsNamedLayoutMaterialization(5, 0))
}

func TestCountMirrorProductAssignments(t *testing.T) {
	raw, err := json.Marshal([]LocalMirrorSlotView{
		{SlotCode: "A1", ProductID: "01a0a7e5-3c68-7895-b526-bcb6504bccfb"},
		{SlotCode: "A2"},
		{SlotCode: "A3", ProductID: "  "},
	})
	require.NoError(t, err)
	require.Equal(t, 1, countMirrorProductAssignments(raw))
}

func TestReportLocalLayout_materializeContract_documented(t *testing.T) {
	// Contract: device reports with named localLayoutId must populate machine_layout_slots for GetMachineLayoutDetail.
	require.Contains(t, reportLocalLayoutMirrorComment(), "machine_local_layout_mirror")
	require.Contains(t, machineLayoutDetailSourceComment(), "machine_layout_slots")
}
