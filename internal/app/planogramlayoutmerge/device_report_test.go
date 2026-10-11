package planogramlayoutmerge

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateDeviceMergePairs_rejectsOverlap(t *testing.T) {
	err := validateDeviceMergePairs([]DeviceMergePair{
		{LeftSlotCode: "A1", RightSlotCode: "A2"},
		{LeftSlotCode: "A2", RightSlotCode: "A3"},
	})
	require.Error(t, err)
}

func TestValidateDeviceMergePairs_acceptsDistinct(t *testing.T) {
	err := validateDeviceMergePairs([]DeviceMergePair{
		{LeftSlotCode: "A1", RightSlotCode: "A2"},
		{LeftSlotCode: "B1", RightSlotCode: "B2"},
	})
	require.NoError(t, err)
}
