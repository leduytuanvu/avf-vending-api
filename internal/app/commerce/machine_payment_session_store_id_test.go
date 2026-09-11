package commerce

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateMachinePaymentSessionInput_StoreIDMatchesMachineCode(t *testing.T) {
	t.Parallel()
	in := CreateMachinePaymentSessionInput{
		MachineExternalCode: "AVF000132",
		StoreID:             "AVF000132",
	}
	require.Equal(t, "AVF000132", in.StoreID)
	require.Equal(t, in.MachineExternalCode, in.StoreID)
}
