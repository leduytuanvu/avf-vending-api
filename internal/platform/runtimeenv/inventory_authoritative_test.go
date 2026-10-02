package runtimeenv

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInventoryAuthoritativeServer(t *testing.T) {
	t.Setenv("INVENTORY_AUTHORITATIVE_SERVER", "")
	require.True(t, InventoryAuthoritativeServer())
	t.Setenv("INVENTORY_AUTHORITATIVE_SERVER", "false")
	require.False(t, InventoryAuthoritativeServer())
	t.Setenv("INVENTORY_AUTHORITATIVE_SERVER", "0")
	require.False(t, InventoryAuthoritativeServer())
}
