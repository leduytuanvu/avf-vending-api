package layoutassignment

import (
	"context"
	"fmt"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/fleet"
	"github.com/avf/avf-vending-api/internal/app/physicaltopology"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/pgxutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// materializeDeviceSlotsToNamedLayout replaces machine_layout_slots for a named layout from a device
// mirror/snapshot slots JSON array, then syncs commerce slot configs so catalog/planogram and
// GetMachineLayoutDetail stay aligned.
func (s *Service) materializeDeviceSlotsToNamedLayout(
	ctx context.Context,
	tx pgx.Tx,
	machineID uuid.UUID,
	hintLayoutID uuid.UUID,
	slotsJSON []byte,
	fingerprint string,
	materializedBy string,
) error {
	layoutID, err := s.resolveNamedLayoutIDForMaterialize(ctx, tx, machineID, hintLayoutID)
	if err != nil {
		return err
	}
	if layoutID == uuid.Nil {
		return nil
	}
	q := pgxutil.NewQueries(tx)
	layout, err := q.GetMachineLayoutByID(ctx, db.GetMachineLayoutByIDParams{
		ID:        layoutID,
		MachineID: machineID,
	})
	if err != nil {
		return err
	}
	mirrorSlots := ParseMirrorSlots(slotsJSON)
	if len(mirrorSlots) == 0 {
		return nil
	}
	if err := q.DeleteMachineLayoutMergePairsByLayoutID(ctx, layoutID); err != nil {
		return err
	}
	if err := q.DeleteMachineLayoutSlotsByLayoutID(ctx, layoutID); err != nil {
		return err
	}
	cols := int(layout.GridCols)
	for _, sl := range mirrorSlots {
		code := strings.TrimSpace(sl.SlotCode)
		if code == "" {
			continue
		}
		ordinal := sl.SlotOrdinal
		if ordinal < 1 {
			ordinal = int32(physicaltopology.SlotIndexFromCode(code, cols))
		}
		lane := physicaltopology.SlotIndexFromCode(code, cols)
		var pid pgtype.UUID
		if pidStr := strings.TrimSpace(sl.ProductID); pidStr != "" {
			parsed, parseErr := uuid.Parse(pidStr)
			if parseErr == nil {
				pid = pgtype.UUID{Bytes: parsed, Valid: true}
			}
		}
		maxQty := sl.MaxQuantity
		if maxQty <= 0 {
			maxQty = 6
		}
		enabled := true
		if sl.Enabled != nil {
			enabled = *sl.Enabled
		}
		opState := "unassigned"
		if sl.OperationalState != nil && strings.TrimSpace(*sl.OperationalState) != "" {
			opState = strings.TrimSpace(*sl.OperationalState)
		} else if pid.Valid {
			opState = "assigned"
		}
		curInv := int32(0)
		if sl.CurrentInventory != nil {
			curInv = *sl.CurrentInventory
		}
		var slotRow, slotCol pgtype.Int4
		if lane > 0 && cols > 0 {
			zeroBased := int(lane) - 1
			slotRow = pgtype.Int4{Int32: int32(zeroBased / cols), Valid: true}
			slotCol = pgtype.Int4{Int32: int32(zeroBased%cols) + 1, Valid: true}
		}
		if err := q.InsertMachineLayoutSlot(ctx, db.InsertMachineLayoutSlotParams{
			LayoutID:         layoutID,
			SlotCode:         code,
			SlotOrdinal:      ordinal,
			SlotRow:          slotRow,
			SlotColumn:       slotCol,
			PhysicalLane:     pgtype.Int4{Int32: int32(lane), Valid: lane > 0},
			ProductID:        pid,
			MaxQuantity:      maxQty,
			PriceMinor:       pgtype.Int8{Int64: sl.PriceMinor, Valid: sl.PriceMinor > 0},
			CurrentInventory: curInv,
			Enabled:          enabled,
			OperationalState: opState,
		}); err != nil {
			return err
		}
	}
	fp := strings.TrimSpace(fingerprint)
	if fp == "" {
		fp = fmt.Sprintf("device:%s:v%d", layoutID, layout.LayoutRevision+1)
	}
	if _, err := q.UpdateMachineLayoutMetadata(ctx, db.UpdateMachineLayoutMetadataParams{
		ID:              layoutID,
		MachineID:       machineID,
		LayoutRevision:  pgtype.Int4{Int32: layout.LayoutRevision + 1, Valid: true},
		Fingerprint:     pgtype.Text{String: fp, Valid: true},
	}); err != nil {
		return err
	}
	_, err = fleet.SyncNamedLayoutSlotsToCurrentConfigs(ctx, tx, machineID, layoutID, materializedBy)
	return err
}

func (s *Service) resolveNamedLayoutIDForMaterialize(
	ctx context.Context,
	tx pgx.Tx,
	machineID uuid.UUID,
	hintLayoutID uuid.UUID,
) (uuid.UUID, error) {
	q := pgxutil.NewQueries(tx)
	if hintLayoutID != uuid.Nil {
		if _, err := q.GetMachineLayoutByID(ctx, db.GetMachineLayoutByIDParams{
			ID:        hintLayoutID,
			MachineID: machineID,
		}); err == nil {
			return hintLayoutID, nil
		}
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

func countMirrorProductAssignments(slotsJSON []byte) int {
	slots := ParseMirrorSlots(slotsJSON)
	n := 0
	for _, sl := range slots {
		if strings.TrimSpace(sl.ProductID) != "" {
			n++
		}
	}
	return n
}

func countAssignedNamedLayoutSlots(slotRows []db.MachineLayoutSlot) int {
	n := 0
	for _, sl := range slotRows {
		if sl.ProductID.Valid && sl.ProductID.Bytes != uuid.Nil {
			n++
		}
	}
	return n
}

// needsNamedLayoutMaterialization reports when mirror/snapshot payload has assignments but named slots do not.
func needsNamedLayoutMaterialization(mirrorAssignments, namedAssignments int) bool {
	return mirrorAssignments > 0 && namedAssignments == 0
}
