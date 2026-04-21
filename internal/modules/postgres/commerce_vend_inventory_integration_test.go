package postgres_test

import (
	"context"
	"strings"
	"testing"

	"github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestApplyCommerceVendSuccessInventory_rejectsNonSuccessVend(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orderIDem := "vend-inv-" + uuid.NewString()
	orderRes, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      2,
		Currency:       "USD",
		SubtotalMinor:  200,
		TaxMinor:       0,
		TotalMinor:     200,
		IdempotencyKey: orderIDem,
		OrderStatus:    "created",
		VendState:      "pending",
	})
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `UPDATE vend_sessions SET state = 'failed' WHERE order_id = $1 AND slot_index = $2`, orderRes.Order.ID, int32(2))
	require.NoError(t, err)

	_, err = store.ApplyCommerceVendSuccessInventory(ctx,
		testfixtures.DevOrganizationID,
		testfixtures.DevMachineID,
		orderRes.Order.ID,
		2,
		testfixtures.DevProductWater,
		"test-idem-vend-sale",
		nil,
	)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "success"), "expected vend state guard, got: %v", err)
}
