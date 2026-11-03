package httpserver

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestParseProductListItemInputs_prefersItemsAndKeepsZero(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	zero := int64(0)
	body := V1AdminProductListItemsReplaceRequest{
		ProductIDs: []string{uuid.New().String()},
		Items: []V1AdminProductListItemWrite{
			{ProductID: id.String(), UnitPriceMinor: &zero},
		},
	}
	got, err := parseProductListItemInputs(body)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, id, got[0].ProductID)
	require.NotNil(t, got[0].UnitPriceMinor)
	require.Equal(t, int64(0), *got[0].UnitPriceMinor)
}

func TestParseProductListItemInputs_productIdsInherit(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	got, err := parseProductListItemInputs(V1AdminProductListItemsReplaceRequest{
		ProductIDs: []string{id.String()},
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, id, got[0].ProductID)
	require.Nil(t, got[0].UnitPriceMinor)
}

func TestProductListItemWriteJSON_roundTripZero(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"items":[{"productId":"11111111-1111-1111-1111-111111111111","unitPriceMinor":0}]}`)
	var body V1AdminProductListItemsReplaceRequest
	require.NoError(t, json.Unmarshal(raw, &body))
	require.Len(t, body.Items, 1)
	require.NotNil(t, body.Items[0].UnitPriceMinor)
	require.Equal(t, int64(0), *body.Items[0].UnitPriceMinor)

	out, err := json.Marshal(V1AdminProductListItemMembership{
		ProductID:      "p1",
		Sku:            "SKU",
		Name:           "Name",
		UnitPriceMinor: body.Items[0].UnitPriceMinor,
	})
	require.NoError(t, err)
	require.Contains(t, string(out), `"unitPriceMinor":0`)
}
