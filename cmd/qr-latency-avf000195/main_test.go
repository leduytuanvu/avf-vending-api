package main

import (
	"encoding/base64"
	"testing"

	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/stretchr/testify/require"
)

func TestForbidProdLiveMoMo(t *testing.T) {
	t.Parallel()
	require.Error(t, forbidProdLiveMoMo("grpcs://machine-api.ldtv.dev:443", "live", []string{"momo"}))
	require.NoError(t, forbidProdLiveMoMo("grpcs://machine-api.ldtv.dev:443", "sandbox", []string{"momo"}))
	require.NoError(t, forbidProdLiveMoMo("grpcs://machine-api.ldtv.dev:443", "live", []string{"psp_grpc_int", "sandbox"}))
	require.NoError(t, forbidProdLiveMoMo("localhost:9090", "live", []string{"momo"}))
}

func TestDecodeMachineIDFromJWT(t *testing.T) {
	t.Parallel()
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"machine_id":"01a0a7e5-3c68-7895-b526-bcb6504bccfb"}`))
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	id, err := decodeMachineIDFromJWT(header + "." + payload + ".")
	require.NoError(t, err)
	require.Equal(t, "01a0a7e5-3c68-7895-b526-bcb6504bccfb", id)
}

func TestPickFirstInStockCatalogLine(t *testing.T) {
	t.Parallel()
	pid, slot, ok := pickFirstInStockCatalogLine([]*machinev1.CatalogSlotItem{
		{ProductId: "empty", IsAvailable: true, AvailableQuantity: 0, SlotIndex: 2},
		{ProductId: "gone", IsAvailable: false, AvailableQuantity: 4, SlotIndex: 3},
		{ProductId: "ok-prod", IsAvailable: true, AvailableQuantity: 2, SlotIndex: 7},
	})
	require.True(t, ok)
	require.Equal(t, "ok-prod", pid)
	require.Equal(t, int32(7), slot)

	_, _, ok = pickFirstInStockCatalogLine(nil)
	require.False(t, ok)
}
