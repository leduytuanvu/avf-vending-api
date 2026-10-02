package inventoryadmin

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/layoutassignment"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrNoDeviceLayoutMirror is returned when snapshot-only reads are requested but the machine has no mirror row.
var ErrNoDeviceLayoutMirror = errors.New("inventoryadmin: no device layout mirror")

// ListSlotInventoryViewFromMirror maps the latest device layout mirror into admin slot list items.
func (s *Service) ListSlotInventoryViewFromMirror(ctx context.Context, machineID uuid.UUID) ([]SlotInventoryViewItem, error) {
	if s == nil {
		return nil, fmt.Errorf("inventoryadmin: nil service")
	}
	mirror, err := s.q.GetMachineLocalLayoutMirror(ctx, machineID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoDeviceLayoutMirror
		}
		return nil, err
	}
	slots := layoutassignment.ParseMirrorSlots(mirror.Slots)
	if len(slots) == 0 {
		return []SlotInventoryViewItem{}, nil
	}
	_, err = s.q.InventoryAdminGetMachineOrg(ctx, machineID)
	if err != nil {
		return nil, err
	}
	currency, err := s.q.InventoryAdminGetOrgDefaultCurrency(ctx)
	if err != nil {
		return nil, err
	}

	var planogramID uuid.UUID
	var planogramName string
	var planogramRevision int32
	legacyRows, legacyErr := s.q.InventoryAdminListMachineSlots(ctx, machineID)
	if legacyErr == nil && len(legacyRows) > 0 {
		planogramID = legacyRows[0].PlanogramID
		planogramName = legacyRows[0].PlanogramName
		planogramRevision = legacyRows[0].PlanogramRevisionApplied
	}

	out := make([]SlotInventoryViewItem, 0, len(slots))
	for i, sl := range slots {
		slotCode := strings.TrimSpace(sl.SlotCode)
		if slotCode == "" {
			continue
		}
		slotIndex := sl.SlotOrdinal
		if slotIndex <= 0 {
			slotIndex = int32(i + 1)
		}
		capacity := sl.MaxQuantity
		if capacity <= 0 {
			capacity = 6
		}
		current := int32(0)
		if sl.CurrentInventory != nil {
			current = *sl.CurrentInventory
		}
		lowTh := lowStockThreshold(capacity)
		isEmpty := sl.ProductID == "" && current <= 0
		lowStock := capacity > 0 && current > 0 && float64(current)/float64(capacity) < 0.15
		st := slotStatus(current, capacity, isEmpty, lowStock)
		var pid *uuid.UUID
		if pidStr := strings.TrimSpace(sl.ProductID); pidStr != "" {
			if u, parseErr := uuid.Parse(pidStr); parseErr == nil {
				pid = &u
			}
		}
		cur := strings.TrimSpace(sl.Currency)
		if cur == "" {
			cur = currency
		}
		out = append(out, SlotInventoryViewItem{
			MachineID:         machineID,
			PlanogramID:       planogramID,
			PlanogramName:     planogramName,
			SlotIndex:         slotIndex,
			CabinetCode:       defaultAdminCabinetCode,
			CabinetIndex:      0,
			SlotCode:          slotCode,
			ProductID:         pid,
			ProductSku:        strings.TrimSpace(sl.ProductSku),
			ProductName:       strings.TrimSpace(sl.ProductName),
			Capacity:          capacity,
			ParLevel:          capacity,
			CurrentStock:      current,
			LowStockThreshold: lowTh,
			PriceMinor:        sl.PriceMinor,
			Currency:          cur,
			Status:            st,
			PlanogramRevision: planogramRevision,
			UpdatedAt:         mirror.ReportedAt,
			IsEmpty:           isEmpty,
			LowStock:          lowStock,
		})
	}
	return out, nil
}
