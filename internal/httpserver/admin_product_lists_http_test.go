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

func TestParseMachineProductListsPutRequest_fullReplace(t *testing.T) {
	t.Parallel()
	primary := uuid.New()
	supp := uuid.New()
	gotPrimary, gotSupp, err := parseMachineProductListsPutRequest(V1AdminMachineProductListsPutRequest{
		PrimaryListID:       primary.String(),
		SupplementalListIDs: []string{supp.String()},
	})
	require.NoError(t, err)
	require.Equal(t, primary, gotPrimary)
	require.Equal(t, []uuid.UUID{supp}, gotSupp)
}

func TestParseMachineProductListsPutRequest_supplementalOnly(t *testing.T) {
	t.Parallel()
	supp := uuid.New()
	gotPrimary, gotSupp, err := parseMachineProductListsPutRequest(V1AdminMachineProductListsPutRequest{
		SupplementalListIDs: []string{supp.String()},
	})
	require.NoError(t, err)
	require.Equal(t, uuid.Nil, gotPrimary)
	require.Equal(t, []uuid.UUID{supp}, gotSupp)
}

func TestParseMachineProductListsPutRequest_emptyRequiresPrimary(t *testing.T) {
	t.Parallel()
	_, _, err := parseMachineProductListsPutRequest(V1AdminMachineProductListsPutRequest{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "primaryListId required")
}

func TestParseMachineProductListsPutRequest_invalidPrimary(t *testing.T) {
	t.Parallel()
	_, _, err := parseMachineProductListsPutRequest(V1AdminMachineProductListsPutRequest{
		PrimaryListID: "not-a-uuid",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid primaryListId")
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
