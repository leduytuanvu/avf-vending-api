package layoutassignment

import (
	"strings"

	"github.com/avf/avf-vending-api/internal/app/planogramlayoutmerge"
)

func materializeMergeTopologyFromReport(in ReportLocalLayoutInput) *MaterializeMergeTopology {
	if !in.MergeTopologyIncluded {
		return nil
	}
	return &MaterializeMergeTopology{
		Included: true,
		Pairs:    snapshotPairsToDeviceMergePairs(in.MergePairs),
	}
}

func materializeMergeTopologyFromSnapshot(in ReportLayoutSnapshotInput) *MaterializeMergeTopology {
	included := in.MergeTopologyIncluded
	if !included && len(in.MergePairs) > 0 {
		// Technician snapshot uploads may send pairs without the explicit flag during rollout.
		included = true
	}
	if !included && in.SnapshotReason == SnapshotReasonTechnicianCommit {
		// Technician leave snapshot may carry an explicit empty pair list after split-all.
		included = true
	}
	if !included {
		return nil
	}
	return &MaterializeMergeTopology{
		Included: true,
		Pairs:    snapshotPairsToDeviceMergePairs(in.MergePairs),
	}
}

func snapshotPairsToDeviceMergePairs(pairs []SnapshotMergePairInput) []planogramlayoutmerge.DeviceMergePair {
	out := make([]planogramlayoutmerge.DeviceMergePair, 0, len(pairs))
	for _, p := range pairs {
		left := strings.TrimSpace(p.LeftSlotCode)
		right := strings.TrimSpace(p.RightSlotCode)
		if left == "" || right == "" {
			continue
		}
		out = append(out, planogramlayoutmerge.DeviceMergePair{
			LeftSlotCode:  left,
			RightSlotCode: right,
		})
	}
	return out
}
