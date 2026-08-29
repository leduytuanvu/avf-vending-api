package layoutassignment

import (
	"encoding/json"
	"testing"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/stretchr/testify/require"
)

func TestValidateReportedSlotsUnique_acceptsDistinctCodes(t *testing.T) {
	payload, err := json.Marshal([]map[string]string{
		{"slotCode": "A1"},
		{"slotCode": "A2"},
		{"slotCode": "B1"},
	})
	require.NoError(t, err)
	require.NoError(t, validateReportedSlotsUnique(payload))
}

func TestValidateReportedSlotsUnique_rejectsDuplicateCodes(t *testing.T) {
	payload, err := json.Marshal([]map[string]string{
		{"slotCode": "A1"},
		{"slotCode": "A1"},
	})
	require.NoError(t, err)
	err = validateReportedSlotsUnique(payload)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate slotCode")
}

func TestValidateReportedSlotsUnique_rejectsEmptySlotCode(t *testing.T) {
	payload, err := json.Marshal([]map[string]string{{"slotCode": "  "}})
	require.NoError(t, err)
	err = validateReportedSlotsUnique(payload)
	require.Error(t, err)
	require.Contains(t, err.Error(), "slotCode is required")
}

func TestValidateReportedSlotsUnique_rejectsInvalidJSON(t *testing.T) {
	err := validateReportedSlotsUnique([]byte(`{"slotCode":"A1"}`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid slots payload")
}

func TestClassifyLayoutRow_provenFromLayoutSpec(t *testing.T) {
	class, evidence := classifyLayoutRow(dbMachineSlotLayout("grid-6x10", map[string]any{
		"rows": float64(6),
		"cols": float64(10),
	}))
	require.Equal(t, "PROVEN", class)
	require.Equal(t, int32(6), evidence["rows"])
	require.Equal(t, int32(10), evidence["cols"])
}

func TestClassifyLayoutRow_inferredFromLayoutKey(t *testing.T) {
	class, evidence := classifyLayoutRow(dbMachineSlotLayout("grid-10x8", map[string]any{}))
	require.Equal(t, "INFERRED_SAFE", class)
	require.Equal(t, int32(8), evidence["rows"])
	require.Equal(t, int32(10), evidence["cols"])
}

func TestClassifyLayoutRow_requiresReviewForUnknownKey(t *testing.T) {
	class, _ := classifyLayoutRow(dbMachineSlotLayout("belt-8", map[string]any{}))
	require.Equal(t, "REQUIRES_REVIEW", class)
}

func dbMachineSlotLayout(layoutKey string, spec map[string]any) db.MachineSlotLayout {
	specBytes, _ := json.Marshal(spec)
	return db.MachineSlotLayout{
		LayoutKey:  layoutKey,
		LayoutSpec: specBytes,
	}
}
