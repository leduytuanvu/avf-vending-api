package planogramlayoutmerge

import (
	"context"
	"fmt"
	"strings"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// DeviceMergePair is one active merge pair reported by a machine layout ingest.
type DeviceMergePair struct {
	LeftSlotCode  string
	RightSlotCode string
}

// ReplaceMachineMergeTopologyFromDeviceReport replaces active lane merges and named layout merge rows
// with the reported pair set. An empty pair list clears all active merges for the machine.
func ReplaceMachineMergeTopologyFromDeviceReport(
	ctx context.Context,
	q *db.Queries,
	machineID uuid.UUID,
	layoutID uuid.UUID,
	pairs []DeviceMergePair,
) error {
	if machineID == uuid.Nil {
		return fmt.Errorf("machine_id required")
	}
	if err := validateDeviceMergePairs(pairs); err != nil {
		return err
	}
	if err := q.PlanogramDeactivateAllMergePairsForMachine(ctx, machineID); err != nil {
		return err
	}
	if len(pairs) > 0 {
		revision, err := q.PlanogramNextMergePairRevision(ctx, machineID)
		if err != nil {
			return err
		}
		for _, p := range pairs {
			left := strings.TrimSpace(p.LeftSlotCode)
			right := strings.TrimSpace(p.RightSlotCode)
			if _, err := q.PlanogramInsertActiveMergePair(ctx, db.PlanogramInsertActiveMergePairParams{
				MachineID:         machineID,
				LeftSlotCode:      left,
				RightSlotCode:     right,
				CabinetCode:       "",
				LayoutKey:         "default",
				LayoutRevision:    1,
				Revision:          revision,
				OperatorSessionID: pgtype.UUID{},
			}); err != nil {
				return err
			}
		}
	}
	if layoutID == uuid.Nil {
		return nil
	}
	if err := q.DeleteMachineLayoutMergePairsByLayoutID(ctx, layoutID); err != nil {
		return err
	}
	for _, p := range pairs {
		left := strings.TrimSpace(p.LeftSlotCode)
		right := strings.TrimSpace(p.RightSlotCode)
		if err := q.InsertMachineLayoutMergePair(ctx, db.InsertMachineLayoutMergePairParams{
			LayoutID:      layoutID,
			LeftSlotCode:  left,
			RightSlotCode: right,
		}); err != nil {
			return err
		}
	}
	return nil
}

func validateDeviceMergePairs(pairs []DeviceMergePair) error {
	seenSlot := make(map[string]string)
	for _, p := range pairs {
		left := strings.TrimSpace(p.LeftSlotCode)
		right := strings.TrimSpace(p.RightSlotCode)
		if left == "" || right == "" || left == right {
			return fmt.Errorf("invalid merge pair %q-%q", left, right)
		}
		for _, code := range []string{left, right} {
			if other, ok := seenSlot[code]; ok && other != left {
				return fmt.Errorf("merge pair overlap on slot %q", code)
			}
			seenSlot[code] = left
		}
	}
	return nil
}
