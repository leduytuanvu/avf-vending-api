package layoutassignment

import (
	"context"
	"fmt"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/pgxutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// MachineLayoutSlotView is one slot row for a named layout detail read.
type MachineLayoutSlotView struct {
	SlotCode             string
	SlotOrdinal          int32
	LogicalCoordinate    string
	PhysicalLane         int32
	ProductID            string
	MaxQuantity          int32
	PriceMinor           int64
	LocalPricingRevision int64
	CurrentInventory     int32
	Enabled              bool
	OperationalState     string
}

// MachineLayoutMergePairView is a merge pair for a named layout.
type MachineLayoutMergePairView struct {
	LeftSlotCode  string
	RightSlotCode string
}

// MachineLayoutDetailView is layout summary plus slots and merge pairs.
type MachineLayoutDetailView struct {
	Summary    MachineLayoutSummaryView
	Slots      []MachineLayoutSlotView
	MergePairs []MachineLayoutMergePairView
}

// GetMachineLayoutDetail returns one named layout owned by [machineID].
func (s *Service) GetMachineLayoutDetail(ctx context.Context, machineID, layoutID uuid.UUID) (*MachineLayoutDetailView, error) {
	if s.Pool == nil {
		return nil, fmt.Errorf("database pool is not configured")
	}
	if machineID == uuid.Nil {
		return nil, fmt.Errorf("machineId is required")
	}
	if layoutID == uuid.Nil {
		return nil, fmt.Errorf("layoutId is required")
	}
	q := pgxutil.NewQueries(s.Pool)
	row, err := q.GetMachineLayoutByID(ctx, db.GetMachineLayoutByIDParams{
		ID:        layoutID,
		MachineID: machineID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrLayoutNotFound
		}
		return nil, err
	}
	slotRows, err := q.ListMachineLayoutSlots(ctx, layoutID)
	if err != nil {
		return nil, err
	}
	pairRows, err := q.ListMachineLayoutMergePairs(ctx, layoutID)
	if err != nil {
		return nil, err
	}
	view := &MachineLayoutDetailView{
		Summary: MachineLayoutSummaryView{
			LayoutID:    row.ID,
			Name:        row.Name,
			Status:      row.Status,
			GridRows:    row.GridRows,
			GridCols:    row.GridCols,
			Revision:    row.LayoutRevision,
			Fingerprint: row.Fingerprint,
		},
		Slots:      make([]MachineLayoutSlotView, 0, len(slotRows)),
		MergePairs: make([]MachineLayoutMergePairView, 0, len(pairRows)),
	}
	for _, sl := range slotRows {
		logical := ""
		if sl.SlotRow.Valid && sl.SlotColumn.Valid {
			logical = fmt.Sprintf("%d,%d", sl.SlotRow.Int32, sl.SlotColumn.Int32)
		}
		var physical int32
		if sl.PhysicalLane.Valid {
			physical = sl.PhysicalLane.Int32
		}
		productID := ""
		if sl.ProductID.Valid {
			productID = uuid.UUID(sl.ProductID.Bytes).String()
		}
		var priceMinor int64
		if sl.PriceMinor.Valid {
			priceMinor = sl.PriceMinor.Int64
		}
		view.Slots = append(view.Slots, MachineLayoutSlotView{
			SlotCode:          sl.SlotCode,
			SlotOrdinal:       sl.SlotOrdinal,
			LogicalCoordinate: logical,
			PhysicalLane:      physical,
			ProductID:         productID,
			MaxQuantity:       sl.MaxQuantity,
			PriceMinor:        priceMinor,
			CurrentInventory:  sl.CurrentInventory,
			Enabled:           sl.Enabled,
			OperationalState:  sl.OperationalState,
		})
	}
	for _, p := range pairRows {
		view.MergePairs = append(view.MergePairs, MachineLayoutMergePairView{
			LeftSlotCode:  p.LeftSlotCode,
			RightSlotCode: p.RightSlotCode,
		})
	}
	return view, nil
}
