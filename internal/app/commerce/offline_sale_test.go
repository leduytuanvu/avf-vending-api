package commerce

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestParseOfflineSalePayload_acceptsNestedSnapshotObject(t *testing.T) {
	t.Parallel()
	productID := uuid.NewString()
	payload := []byte(`{
		"orderId":"ord-1",
		"machineId":"machine-1",
		"cashReceivedMinor":5000,
		"payableTotalMinor":5000,
		"currency":"USD",
		"pricingSnapshot":{
			"snapshotId":"snap-1",
			"machineId":"machine-1",
			"payableTotalMinor":5000,
			"currency":"USD",
			"localPricingRevision":3,
			"slotConfigVersion":1,
			"capturedAtEpochMs":1700000000000,
			"lines":[{"slotCode":"A01","productId":"` + productID + `","unitPriceMinor":5000,"quantity":1}]
		}
	}`)
	wire, snap, err := parseOfflineSalePayload(payload)
	require.NoError(t, err)
	require.Equal(t, "ord-1", wire.OrderID)
	require.Equal(t, "snap-1", snap.SnapshotID)
	require.Equal(t, productID, snap.Lines[0].ProductID)
}

func TestParseOfflineSalePayload_acceptsStringEncodedSnapshot(t *testing.T) {
	t.Parallel()
	productID := uuid.NewString()
	inner, err := json.Marshal(map[string]any{
		"snapshotId":           "snap-2",
		"payableTotalMinor":    2000,
		"currency":             "VND",
		"localPricingRevision": 1,
		"slotConfigVersion":    1,
		"lines": []map[string]any{
			{"slotCode": "B02", "productId": productID, "unitPriceMinor": 2000, "quantity": 1},
		},
	})
	require.NoError(t, err)
	payload, err := json.Marshal(map[string]any{
		"orderId":           "ord-2",
		"cashReceivedMinor": 2000,
		"payableTotalMinor": 2000,
		"currency":          "VND",
		"pricingSnapshot":   string(inner),
	})
	require.NoError(t, err)
	_, snap, err := parseOfflineSalePayload(payload)
	require.NoError(t, err)
	require.Equal(t, "snap-2", snap.SnapshotID)
	require.Equal(t, productID, snap.Lines[0].ProductID)
}

func TestMachinePricingSnapshotFromAppCheckout_mapsTotals(t *testing.T) {
	t.Parallel()
	productID := uuid.New()
	snap := appCheckoutPricingSnapshot{
		SnapshotID:           "snap-x",
		PayableTotalMinor:    4500,
		LocalPricingRevision: 2,
		SlotConfigVersion:    1,
		CapturedAtEpochMs:    1700000000000,
		Lines: []appCheckoutLine{
			{SlotCode: "A01", ProductID: productID.String(), UnitPriceMinor: 4500, Quantity: 1},
		},
	}
	out := machinePricingSnapshotFromAppCheckout(snap, 4500)
	require.Equal(t, int64(4500), out.TotalMinor)
	require.Equal(t, "snap-x", out.SnapshotID)
	require.Len(t, out.Lines, 1)
	require.Equal(t, productID, out.Lines[0].ProductID)
}

func TestMachinePricingSnapshotFromAppCheckout_mapsAllSnapshotLines(t *testing.T) {
	t.Parallel()
	productA := uuid.New()
	productB := uuid.New()
	snap := appCheckoutPricingSnapshot{
		SnapshotID:        "snap-multi",
		PayableTotalMinor: 20_000,
		Currency:          "VND",
		Lines: []appCheckoutLine{
			{SlotCode: "A1", ProductID: productA.String(), UnitPriceMinor: 10_000, Quantity: 1},
			{SlotCode: "A2", ProductID: productB.String(), UnitPriceMinor: 10_000, Quantity: 1},
		},
	}

	out := machinePricingSnapshotFromAppCheckout(snap, 20_000)

	require.Len(t, out.Lines, 2)
	require.Equal(t, int32(1), out.Lines[0].LineSequence)
	require.Equal(t, int32(2), out.Lines[1].LineSequence)
	require.Equal(t, productA, out.Lines[0].ProductID)
	require.Equal(t, productB, out.Lines[1].ProductID)
	require.Equal(t, int64(20_000), out.TotalMinor)
}

func TestParseOfflineSalePayload_requiresProductIdOnEveryLine(t *testing.T) {
	t.Parallel()
	payload := []byte(`{
		"orderId":"order-1",
		"machineId":"` + uuid.NewString() + `",
		"transactionId":"tx-1",
		"cashReceivedMinor":10000,
		"payableTotalMinor":10000,
		"currency":"VND",
		"pricingSnapshot":{
			"snapshotId":"snap-1",
			"payableTotalMinor":10000,
			"currency":"VND",
			"lines":[
				{"slotCode":"A1","productId":"` + uuid.NewString() + `","unitPriceMinor":5000,"quantity":1},
				{"slotCode":"A2","productId":"","unitPriceMinor":5000,"quantity":1}
			]
		}
	}`)

	_, _, err := parseOfflineSalePayload(payload)
	require.Error(t, err)
}
