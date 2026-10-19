package layoutassignment

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMaterializeMergeTopologyFromSnapshot_technicianCommitEmptyPairsIncluded(t *testing.T) {
	out := materializeMergeTopologyFromSnapshot(ReportLayoutSnapshotInput{
		SnapshotReason:        SnapshotReasonTechnicianCommit,
		MergeTopologyIncluded: false,
		MergePairs:            nil,
	})
	require.NotNil(t, out)
	require.True(t, out.Included)
	require.Empty(t, out.Pairs)
}

func TestMaterializeMergeTopologyFromSnapshot_explicitFlagEmptyPairs(t *testing.T) {
	out := materializeMergeTopologyFromSnapshot(ReportLayoutSnapshotInput{
		SnapshotReason:        SnapshotReasonPeriodic30M,
		MergeTopologyIncluded: true,
		MergePairs:            nil,
	})
	require.NotNil(t, out)
	require.True(t, out.Included)
	require.Empty(t, out.Pairs)
}

func TestMaterializeMergeTopologyFromSnapshot_periodicWithoutFlagOrPairsNil(t *testing.T) {
	out := materializeMergeTopologyFromSnapshot(ReportLayoutSnapshotInput{
		SnapshotReason:        SnapshotReasonPeriodic30M,
		MergeTopologyIncluded: false,
		MergePairs:            nil,
	})
	require.Nil(t, out)
}

func TestMaterializeMergeTopologyFromSnapshot_legacyNonEmptyPairs(t *testing.T) {
	out := materializeMergeTopologyFromSnapshot(ReportLayoutSnapshotInput{
		SnapshotReason:        SnapshotReasonLegacyReport,
		MergeTopologyIncluded: false,
		MergePairs: []SnapshotMergePairInput{
			{LeftSlotCode: "A9", RightSlotCode: "A10"},
		},
	})
	require.NotNil(t, out)
	require.True(t, out.Included)
	require.Len(t, out.Pairs, 1)
}
