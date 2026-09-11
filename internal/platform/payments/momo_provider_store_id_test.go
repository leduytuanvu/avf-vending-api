package payments

import (
	"testing"

	"github.com/avf/avf-vending-api/internal/platform/payments/psp/momo"
	"github.com/stretchr/testify/require"
)

func TestResolveMoMoStoreID_PrefersInputStoreID(t *testing.T) {
	t.Parallel()
	got := resolveMoMoStoreID(
		CreatePaymentSessionInput{StoreID: "AVF000132"},
		momo.Credentials{TerminalID: "AVF000001"},
	)
	require.Equal(t, "AVF000132", got)
}

func TestResolveMoMoStoreID_FallsBackToTerminalID(t *testing.T) {
	t.Parallel()
	got := resolveMoMoStoreID(
		CreatePaymentSessionInput{},
		momo.Credentials{TerminalID: "AVF000001"},
	)
	require.Equal(t, "AVF000001", got)
}

func TestMomoOrderInfo_IncludesMachineCode(t *testing.T) {
	t.Parallel()
	require.Equal(t, "Thanh toan AVF000132 don hang MoMo ORDER1", momoOrderInfo("AVF000132", "ORDER1"))
	require.Equal(t, "Thanh toan don hang MoMo ORDER1", momoOrderInfo("", "ORDER1"))
}
