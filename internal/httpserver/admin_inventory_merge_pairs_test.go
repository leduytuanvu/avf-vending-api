package httpserver

import (
	"encoding/json"
	"testing"

	"github.com/avf/avf-vending-api/internal/app/setupapp"
	"github.com/stretchr/testify/require"
)

func TestMapMergePairsToV1_empty(t *testing.T) {
	out := mapMergePairsToV1(nil)
	require.Empty(t, out)
}

func TestMapMergePairsToV1_mapsFields(t *testing.T) {
	out := mapMergePairsToV1([]setupapp.LaneMergePair{
		{
			LeftSlotCode:   "A1",
			RightSlotCode:  "A2",
			CabinetCode:    "A",
			LayoutKey:      "default",
			LayoutRevision: 1,
			Revision:       3,
		},
	})
	require.Len(t, out, 1)
	require.Equal(t, "A1", out[0].LeftSlotCode)
	require.Equal(t, "A2", out[0].RightSlotCode)
	require.Equal(t, int32(3), out[0].Revision)
}

func TestV1PlanogramMergePairsApplyRequest_jsonRoundTrip(t *testing.T) {
	body := v1PlanogramMergePairsApplyRequest{
		OperatorSessionID: "11111111-1111-4111-8111-111111111111",
		Items: []v1PlanogramMergePairApplyItem{
			{LeftSlotCode: "A1", RightSlotCode: "A2", Merge: true},
			{LeftSlotCode: "A1", RightSlotCode: "A2", Merge: false},
		},
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	var decoded v1PlanogramMergePairsApplyRequest
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.Len(t, decoded.Items, 2)
	require.True(t, decoded.Items[0].Merge)
	require.False(t, decoded.Items[1].Merge)
}
