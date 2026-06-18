package grpcserver

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestGetBootstrapDeviceConfigPassThrough_structFromJSON(t *testing.T) {
	t.Parallel()
	meta := map[string]any{
		"board_protocol": "tcn",
		"device_config": map[string]any{
			"schemaVersion": float64(1),
			"bill": map[string]any{
				"enabledDenominationsVnd": []any{float64(10000), float64(20000)},
				"changeDenominationVnd":   float64(10000),
			},
			"tcn": map[string]any{
				"laneMode":        float64(10),
				"shakeCount":      float64(2),
				"dropWaitSeconds": float64(3),
			},
		},
	}
	raw, err := json.Marshal(meta)
	require.NoError(t, err)

	got := structFromJSON(raw)
	require.NotNil(t, got)
	dc := got.GetFields()["device_config"].GetStructValue()
	require.NotNil(t, dc)
	require.Equal(t, float64(1), dc.GetFields()["schemaVersion"].GetNumberValue())
	bill := dc.GetFields()["bill"].GetStructValue()
	require.NotNil(t, bill)
	require.Equal(t, float64(10000), bill.GetFields()["changeDenominationVnd"].GetNumberValue())
}

func TestGetBootstrapDeviceConfigPassThrough_structpbRoundTrip(t *testing.T) {
	t.Parallel()
	meta := map[string]any{
		"device_config": map[string]any{
			"schemaVersion": float64(1),
			"tcn": map[string]any{
				"laneMode": float64(12),
			},
		},
	}
	s, err := structpb.NewStruct(meta)
	require.NoError(t, err)
	require.Equal(t, float64(12), s.GetFields()["device_config"].GetStructValue().GetFields()["tcn"].GetStructValue().GetFields()["laneMode"].GetNumberValue())
}
