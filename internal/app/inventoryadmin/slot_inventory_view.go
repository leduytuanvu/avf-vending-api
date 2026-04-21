package inventoryadmin

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// defaultAdminCabinetCode is returned when no cabinet row exists for this machine (pure legacy inventory).
// Keep in sync with db/queries/inventory_admin.sql coalesce(..., 'CAB-A').
const defaultAdminCabinetCode = "CAB-A"

// SlotInventoryViewItem is the merged legacy planogram slot state plus cabinet slot config context for admin UIs.
type SlotInventoryViewItem struct {
	MachineID         uuid.UUID
	PlanogramID       uuid.UUID
	PlanogramName     string
	SlotIndex         int32
	CabinetCode       string
	CabinetIndex      int32
	SlotCode          string
	ProductID         *uuid.UUID
	ProductSku        string
	ProductName       string
	Capacity          int32
	ParLevel          int32
	CurrentStock      int32
	LowStockThreshold int32
	PriceMinor        int64
	Currency          string
	Status            string
	PlanogramRevision int32
	UpdatedAt         time.Time
	IsEmpty           bool
	LowStock          bool
}

// ListSlotInventoryView returns planogram-backed slot rows enriched with cabinet codes and derived merchandising fields.
func (s *Service) ListSlotInventoryView(ctx context.Context, machineID uuid.UUID) ([]SlotInventoryViewItem, error) {
	if s == nil {
		return nil, fmt.Errorf("inventoryadmin: nil service")
	}
	rows, err := s.q.InventoryAdminListMachineSlots(ctx, machineID)
	if err != nil {
		return nil, err
	}
	cfgs, err := s.q.InventoryAdminListCurrentMachineSlotConfigsByMachine(ctx, machineID)
	if err != nil {
		return nil, err
	}
	head, err := s.q.InventoryAdminGetMachineOrg(ctx, machineID)
	if err != nil {
		return nil, err
	}
	currency, err := s.q.InventoryAdminGetOrgDefaultCurrency(ctx, head.OrganizationID)
	if err != nil {
		return nil, err
	}

	out := make([]SlotInventoryViewItem, 0, len(rows))
	for _, row := range rows {
		cfg := pickSlotConfigForLegacyRow(cfgs, row)
		capacity := row.MaxQuantity
		parLevel := row.MaxQuantity
		priceMinor := row.PriceMinor
		cabinetCode := strings.TrimSpace(row.CabinetCode)
		if cabinetCode == "" {
			cabinetCode = defaultAdminCabinetCode
		}
		cabinetIndex := row.CabinetIndex
		slotCode := fmt.Sprintf("legacy-%d", row.SlotIndex)
		if cfg != nil {
			if cfg.MaxQuantity > 0 {
				capacity = cfg.MaxQuantity
				parLevel = cfg.MaxQuantity
			}
			if cfg.PriceMinor != 0 {
				priceMinor = cfg.PriceMinor
			}
			if cc := strings.TrimSpace(cfg.CabinetCode); cc != "" {
				cabinetCode = cc
			}
			cabinetIndex = cfg.CabinetIndex
			if sc := strings.TrimSpace(cfg.SlotCode); sc != "" {
				slotCode = sc
			}
		}
		lowTh := lowStockThreshold(capacity)
		st := slotStatus(row.CurrentQuantity, capacity, row.IsEmpty, pgBool(row.LowStock))
		var pid *uuid.UUID
		if row.ProductID.Valid {
			u := uuid.UUID(row.ProductID.Bytes)
			pid = &u
		}
		out = append(out, SlotInventoryViewItem{
			MachineID:         row.MachineID,
			PlanogramID:       row.PlanogramID,
			PlanogramName:     row.PlanogramName,
			SlotIndex:         row.SlotIndex,
			CabinetCode:       cabinetCode,
			CabinetIndex:      cabinetIndex,
			SlotCode:          slotCode,
			ProductID:         pid,
			ProductSku:        textFromPg(row.ProductSku),
			ProductName:       textFromPg(row.ProductName),
			Capacity:          capacity,
			ParLevel:          parLevel,
			CurrentStock:      row.CurrentQuantity,
			LowStockThreshold: lowTh,
			PriceMinor:        priceMinor,
			Currency:          currency,
			Status:            st,
			PlanogramRevision: row.PlanogramRevisionApplied,
			UpdatedAt:         row.UpdatedAt,
			IsEmpty:           row.IsEmpty,
			LowStock:          pgBool(row.LowStock),
		})
	}
	return out, nil
}

func textFromPg(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

func pgBool(b pgtype.Bool) bool {
	return b.Valid && b.Bool
}

func int4Val(v pgtype.Int4) (int32, bool) {
	if !v.Valid {
		return 0, false
	}
	return v.Int32, true
}

func pickSlotConfigForLegacyRow(cfgs []db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow, row db.InventoryAdminListMachineSlotsRow) *db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow {
	for i := range cfgs {
		c := &cfgs[i]
		if si, ok := int4Val(c.SlotIndex); ok && si == row.SlotIndex {
			return c
		}
	}
	if row.ProductID.Valid {
		pid := uuid.UUID(row.ProductID.Bytes)
		for i := range cfgs {
			c := &cfgs[i]
			if c.ProductID.Valid && uuid.UUID(c.ProductID.Bytes) == pid {
				return c
			}
		}
	}
	return nil
}

func lowStockThreshold(capacity int32) int32 {
	if capacity <= 0 {
		return 0
	}
	return int32(math.Floor(0.15 * float64(capacity)))
}

func slotStatus(current, capacity int32, isEmpty bool, lowStock bool) string {
	if isEmpty || current <= 0 {
		return "out_of_stock"
	}
	if lowStock || (capacity > 0 && float64(current)/float64(capacity) < 0.15) {
		return "low_stock"
	}
	return "ok"
}
