package layoutassignment

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestReportLayoutSnapshotFromLocalLayout_mapsLegacyFields(t *testing.T) {
	machineID := uuid.New()
	layoutID := uuid.New()
	slots := []byte(`[{"slotCode":"A1"}]`)
	in := ReportLocalLayoutInput{
		MachineID:        machineID,
		LocalLayoutID:    layoutID,
		Revision:         7,
		Rows:             6,
		Columns:          10,
		SlotsJSON:        slots,
		Fingerprint:      "fp-test",
		DeviceInstanceID: "device-1",
		LocalGeneration:  99,
	}
	out := ReportLayoutSnapshotFromLocalLayout(in)
	require.Equal(t, machineID, out.MachineID)
	require.Equal(t, layoutID, out.SnapshotID)
	require.Equal(t, layoutID, out.LayoutID)
	require.Equal(t, int64(7), out.CaptureSequence)
	require.Equal(t, int64(99), out.DeviceGeneration)
	require.Equal(t, SnapshotReasonLegacyReport, out.SnapshotReason)
	require.Equal(t, int32(1), out.PayloadVersion)
	require.Equal(t, slots, out.SlotsJSON)
	require.False(t, out.CapturedAt.IsZero())
}

func TestNormalizeIntervalKey_floorsToUtcHalfHour(t *testing.T) {
	ts := time.Date(2026, 9, 12, 14, 37, 0, 0, time.UTC).UnixMilli()
	key := NormalizeIntervalKey(ts)
	require.NotEmpty(t, key)
}

func TestValidateLayoutName_rejectsBlank(t *testing.T) {
	require.Error(t, ValidateLayoutName("  "))
	require.NoError(t, ValidateLayoutName("Layout 1"))
}

func TestNormalizeSnapshotReason_defaultsLegacy(t *testing.T) {
	require.Equal(t, SnapshotReasonLegacyReport, normalizeSnapshotReason(""))
	require.Equal(t, SnapshotReasonPeriodic30M, normalizeSnapshotReason("PERIODIC_30M"))
	require.Equal(t, SnapshotReasonPeriodic5M, normalizeSnapshotReason("PERIODIC_5M"))
}

func TestSnapshotHistoryPage_JSONTags(t *testing.T) {
	page := SnapshotHistoryPage{
		Items: []SnapshotHistoryItem{{
			SnapshotID:      uuid.New(),
			LayoutID:        uuid.New(),
			CaptureSequence: 1,
			CapturedAt:      time.Now().UTC(),
			ReceivedAt:      time.Now().UTC(),
			Fingerprint:     "fp",
			SnapshotReason:  SnapshotReasonPeriodic5M,
			PayloadVersion:  1,
		}},
		Total: 1,
	}
	raw, err := json.Marshal(page)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"items"`)
	require.Contains(t, string(raw), `"total"`)
	require.NotContains(t, string(raw), `"Items"`)
}
