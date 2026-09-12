package fleet

import (
	"context"
	"fmt"

	"github.com/avf/avf-vending-api/internal/app/physicaltopology"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/pgxutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const defaultLayoutName = "Layout 1"
const defaultSlotCapacity = int32(6)
const defaultBootstrapGridRows = int32(6)
const defaultBootstrapGridCols = int32(10)

// BootstrapDefaultLayout1 creates the initial named layout for a newly provisioned machine.
func BootstrapDefaultLayout1(ctx context.Context, tx pgx.Tx, machineID uuid.UUID, rows, cols int32) (uuid.UUID, error) {
	if machineID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("machineId is required")
	}
	if rows < 1 || cols < 1 {
		rows, cols = defaultBootstrapGridRows, defaultBootstrapGridCols
	}
	q := pgxutil.NewQueries(tx)
	layoutRow, err := q.InsertMachineLayout(ctx, db.InsertMachineLayoutParams{
		MachineID:      machineID,
		Name:           defaultLayoutName,
		Status:         "READY",
		GridRows:       rows,
		GridCols:       cols,
		LayoutRevision: 1,
		Fingerprint:    fmt.Sprintf("bootstrap:%dx%d:v1", rows, cols),
	})
	if err != nil {
		return uuid.Nil, err
	}
	codes := physicaltopology.AllSlotCodes(int(rows), int(cols))
	for ordinal, code := range codes {
		slotRow := int32(ordinal / int(cols))
		slotCol := int32(ordinal%int(cols)) + 1
		lane := physicaltopology.SlotIndexFromCode(code, int(cols))
		if err := q.InsertMachineLayoutSlot(ctx, db.InsertMachineLayoutSlotParams{
			LayoutID:         layoutRow.ID,
			SlotCode:         code,
			SlotOrdinal:      int32(ordinal + 1),
			SlotRow:          pgtype.Int4{Int32: slotRow, Valid: true},
			SlotColumn:       pgtype.Int4{Int32: slotCol, Valid: true},
			PhysicalLane:     pgtype.Int4{Int32: lane, Valid: lane > 0},
			MaxQuantity:      defaultSlotCapacity,
			CurrentInventory: 0,
			Enabled:          true,
			OperationalState: "unassigned",
		}); err != nil {
			return uuid.Nil, err
		}
	}
	if err := q.SetMachineActiveLayoutPointers(ctx, db.SetMachineActiveLayoutPointersParams{
		ID:                     machineID,
		ActiveLayoutID:         pgtype.UUID{Bytes: layoutRow.ID, Valid: true},
		DesiredActiveLayoutID:  pgtype.UUID{Bytes: layoutRow.ID, Valid: true},
		ReportedActiveLayoutID: pgtype.UUID{Bytes: layoutRow.ID, Valid: true},
	}); err != nil {
		return uuid.Nil, err
	}
	return layoutRow.ID, nil
}
