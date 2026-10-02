package inventoryadmin

import (
	"testing"

	"github.com/avf/avf-vending-api/internal/app/layoutassignment"
	"github.com/stretchr/testify/require"
)

func TestMirrorSlotJSONMapsToAdminFields(t *testing.T) {
	raw := []byte(`[{"slotCode":"A1","slotOrdinal":1,"productName":"Cola","currentInventory":2,"maxQuantity":10,"priceMinor":5000}]`)
	slots := layoutassignment.ParseMirrorSlots(raw)
	require.Len(t, slots, 1)
	require.Equal(t, "A1", slots[0].SlotCode)
	require.Equal(t, "Cola", slots[0].ProductName)
}
