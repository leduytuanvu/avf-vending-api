package layoutassignment

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestComputeAssignServerLayoutRequestHash_stableForSamePayload(t *testing.T) {
	machineID := uuid.MustParse("01932f50-7c6a-7b5a-8c3d-123456789abc")
	layoutVersionID := uuid.MustParse("01932f50-7c6a-7b5a-8c3d-abcdef000001")
	orgID := uuid.MustParse("01932f50-7c6a-7b5a-8c3d-abcdef000002")
	rev := int32(3)

	in := AssignServerLayoutInput{
		MachineID:               machineID,
		LayoutVersionID:         layoutVersionID,
		OrgLayoutVersionID:      &orgID,
		ExpectedCurrentRevision: &rev,
		IdempotencyKey:          "should-not-affect-hash",
	}
	other := in
	other.IdempotencyKey = "different-key"

	require.Equal(t, ComputeAssignServerLayoutRequestHash(in), ComputeAssignServerLayoutRequestHash(other))
}

func TestComputeAssignServerLayoutRequestHash_changesWhenPayloadDiffers(t *testing.T) {
	machineID := uuid.MustParse("01932f50-7c6a-7b5a-8c3d-123456789abc")
	layoutA := uuid.MustParse("01932f50-7c6a-7b5a-8c3d-abcdef000001")
	layoutB := uuid.MustParse("01932f50-7c6a-7b5a-8c3d-abcdef000099")

	a := AssignServerLayoutInput{MachineID: machineID, LayoutVersionID: layoutA}
	b := AssignServerLayoutInput{MachineID: machineID, LayoutVersionID: layoutB}
	require.NotEqual(t, ComputeAssignServerLayoutRequestHash(a), ComputeAssignServerLayoutRequestHash(b))
}

func TestComputeAssignServerLayoutRequestHash_expectedRevisionMatters(t *testing.T) {
	machineID := uuid.MustParse("01932f50-7c6a-7b5a-8c3d-123456789abc")
	layoutVersionID := uuid.MustParse("01932f50-7c6a-7b5a-8c3d-abcdef000001")
	rev1 := int32(1)
	rev2 := int32(2)

	a := AssignServerLayoutInput{MachineID: machineID, LayoutVersionID: layoutVersionID, ExpectedCurrentRevision: &rev1}
	b := AssignServerLayoutInput{MachineID: machineID, LayoutVersionID: layoutVersionID, ExpectedCurrentRevision: &rev2}
	require.NotEqual(t, ComputeAssignServerLayoutRequestHash(a), ComputeAssignServerLayoutRequestHash(b))
}
