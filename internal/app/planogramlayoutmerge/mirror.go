package planogramlayoutmerge

import (
	"context"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// RebuildNamedLayoutMergePairsFromLane replaces machine_layout_merge_pairs for [layoutID]
// from active machine_lane_merge_pairs on [machineID].
func RebuildNamedLayoutMergePairsFromLane(
	ctx context.Context,
	q *db.Queries,
	machineID uuid.UUID,
	layoutID uuid.UUID,
) error {
	if machineID == uuid.Nil || layoutID == uuid.Nil {
		return nil
	}
	if err := q.DeleteMachineLayoutMergePairsByLayoutID(ctx, layoutID); err != nil {
		return err
	}
	pairs, err := q.PlanogramListActiveMergePairsForMachine(ctx, machineID)
	if err != nil {
		return err
	}
	for _, p := range pairs {
		if err := q.InsertMachineLayoutMergePair(ctx, db.InsertMachineLayoutMergePairParams{
			LayoutID:      layoutID,
			LeftSlotCode:  p.LeftSlotCode,
			RightSlotCode: p.RightSlotCode,
		}); err != nil {
			return err
		}
	}
	return nil
}

func resolveNamedLayoutID(ctx context.Context, q *db.Queries, machineID uuid.UUID) (uuid.UUID, error) {
	if machineID == uuid.Nil {
		return uuid.Nil, nil
	}
	machine, err := q.GetMachineByID(ctx, machineID)
	if err != nil {
		return uuid.Nil, err
	}
	for _, candidate := range []pgtype.UUID{
		machine.ReportedActiveLayoutID,
		machine.ActiveLayoutID,
		machine.DesiredActiveLayoutID,
	} {
		if !candidate.Valid || candidate.Bytes == uuid.Nil {
			continue
		}
		id := uuid.UUID(candidate.Bytes)
		if _, err := q.GetMachineLayoutByID(ctx, db.GetMachineLayoutByIDParams{
			ID:        id,
			MachineID: machineID,
		}); err == nil {
			return id, nil
		}
	}
	return uuid.Nil, nil
}

// MirrorLaneMergesToNamedLayout syncs machine_layout_merge_pairs from active lane merges.
func MirrorLaneMergesToNamedLayout(ctx context.Context, q *db.Queries, machineID uuid.UUID) error {
	layoutID, err := resolveNamedLayoutID(ctx, q, machineID)
	if err != nil {
		return err
	}
	if layoutID == uuid.Nil {
		return nil
	}
	return RebuildNamedLayoutMergePairsFromLane(ctx, q, machineID, layoutID)
}
