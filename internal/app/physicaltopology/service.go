package physicaltopology

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/pgxutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Slot is one physical machine slot (includes unassigned / merge companion rows).
type Slot struct {
	SlotCode     string
	SlotIndex    int32
	CabinetCode  string
	CabinetIndex int32
	ProductID    *uuid.UUID
	MaxQuantity  int32
	PriceMinor   int64
	MergeRole    string
	MergeWith    string
	IsCurrent    bool
}

// Snapshot is the full physical topology for a machine.
type Snapshot struct {
	MachineID uuid.UUID
	GridRows  int32
	GridCols  int32
	Slots     []Slot
}

// Service reads physical slot topology from machine_slot_configs + layout assignment.
type Service struct {
	Pool *pgxpool.Pool
}

// ListSnapshot returns all current physical slot configs for the machine.
func (s *Service) ListSnapshot(ctx context.Context, machineID uuid.UUID) (Snapshot, error) {
	var out Snapshot
	if s == nil || s.Pool == nil || machineID == uuid.Nil {
		return out, fmt.Errorf("physicaltopology: invalid service or machine_id")
	}
	q := pgxutil.NewQueries(s.Pool)
	rows, err := q.InventoryAdminListCurrentMachineSlotConfigsByMachine(ctx, machineID)
	if err != nil {
		return out, err
	}
	rowsN, colsN := resolveGridDimensions(ctx, q, machineID)
	out.MachineID = machineID
	out.GridRows = rowsN
	out.GridCols = colsN
	out.Slots = mapConfigRows(rows)
	return out, nil
}

func resolveGridDimensions(ctx context.Context, q *db.Queries, machineID uuid.UUID) (int32, int32) {
	if assign, err := q.GetCurrentMachineLayoutAssignment(ctx, db.GetCurrentMachineLayoutAssignmentParams{
		MachineID: machineID,
		Source:    "SERVER",
	}); err == nil && assign.GridRows > 0 && assign.GridCols > 0 {
		return assign.GridRows, assign.GridCols
	}
	return 6, 10
}

func mapConfigRows(rows []db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow) []Slot {
	out := make([]Slot, 0, len(rows))
	for _, row := range rows {
		var pid *uuid.UUID
		if row.ProductID.Valid {
			u := uuid.UUID(row.ProductID.Bytes)
			pid = &u
		}
		var idx int32
		if row.SlotIndex.Valid {
			idx = row.SlotIndex.Int32
		}
		mergeRole, mergeWith := parseMergeMetadata(row.Metadata)
		out = append(out, Slot{
			SlotCode:     strings.TrimSpace(row.SlotCode),
			SlotIndex:    idx,
			CabinetCode:  strings.TrimSpace(row.CabinetCode),
			CabinetIndex: row.CabinetIndex,
			ProductID:    pid,
			MaxQuantity:  row.MaxQuantity,
			PriceMinor:   row.PriceMinor,
			MergeRole:    mergeRole,
			MergeWith:    mergeWith,
			IsCurrent:    row.IsCurrent,
		})
	}
	return out
}

func parseMergeMetadata(raw []byte) (role, with string) {
	if len(raw) == 0 {
		return "", ""
	}
	var meta map[string]any
	if err := json.Unmarshal(raw, &meta); err != nil {
		return "", ""
	}
	if v, ok := meta["mergeRole"].(string); ok {
		role = strings.TrimSpace(v)
	}
	if v, ok := meta["mergeWith"].(string); ok {
		with = strings.TrimSpace(v)
	}
	return role, with
}

// EnsureMissingGridConfigs inserts stub current configs for grid positions missing from DB.
func EnsureMissingGridConfigs(
	ctx context.Context,
	tx pgx.Tx,
	machineID uuid.UUID,
	gridRows, gridCols int32,
	template db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow,
) error {
	if gridRows < 1 || gridCols < 1 {
		return nil
	}
	q := db.New(tx)
	existing, err := q.InventoryAdminListCurrentMachineSlotConfigsByMachine(ctx, machineID)
	if err != nil {
		return err
	}
	byCode := make(map[string]struct{}, len(existing))
	for _, row := range existing {
		byCode[strings.TrimSpace(row.SlotCode)] = struct{}{}
	}
	metaBytes, _ := json.Marshal(map[string]any{"materializedBy": "ensurePhysicalGrid"})
	meta := string(metaBytes)
	eff := template.EffectiveFrom
	if eff.IsZero() {
		eff = time.Now().UTC()
	}
	for _, code := range AllSlotCodes(int(gridRows), int(gridCols)) {
		if _, ok := byCode[code]; ok {
			continue
		}
		idx := SlotIndexFromCode(code, int(gridCols))
		_, err := q.FleetAdminApplyMachineSlotConfigCurrent(ctx, db.FleetAdminApplyMachineSlotConfigCurrentParams{
			MachineID:           machineID,
			SlotCode:            code,
			MachineCabinetID:    template.MachineCabinetID,
			MachineSlotLayoutID: template.MachineSlotLayoutID,
			SlotIndex:           pgtype.Int4{Int32: idx, Valid: idx > 0},
			ProductID:           pgtype.UUID{Valid: false},
			MaxQuantity:         template.MaxQuantity,
			PriceMinor:          0,
			EffectiveFrom:       eff,
			Metadata:            meta,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// EnsureSlotConfigStub inserts a current config row for slotCode when missing.
func EnsureSlotConfigStub(
	ctx context.Context,
	q *db.Queries,
	machineID uuid.UUID,
	slotCode string,
	slotIndex int32,
	template db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow,
) error {
	slotCode = strings.TrimSpace(slotCode)
	if slotCode == "" {
		return nil
	}
	rows, err := q.InventoryAdminListCurrentMachineSlotConfigsByMachine(ctx, machineID)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if strings.TrimSpace(row.SlotCode) == slotCode {
			return nil
		}
	}
	metaBytes, _ := json.Marshal(map[string]any{"materializedBy": "ensureSlotConfigStub"})
	meta := string(metaBytes)
	eff := template.EffectiveFrom
	if eff.IsZero() {
		eff = time.Now().UTC()
	}
	if slotIndex <= 0 {
		slotIndex = template.SlotIndex.Int32
	}
	_, err = q.FleetAdminApplyMachineSlotConfigCurrent(ctx, db.FleetAdminApplyMachineSlotConfigCurrentParams{
		MachineID:           machineID,
		SlotCode:            slotCode,
		MachineCabinetID:    template.MachineCabinetID,
		MachineSlotLayoutID: template.MachineSlotLayoutID,
		SlotIndex:           pgtype.Int4{Int32: slotIndex, Valid: slotIndex > 0},
		ProductID:           pgtype.UUID{Valid: false},
		MaxQuantity:         template.MaxQuantity,
		PriceMinor:          0,
		EffectiveFrom:       eff,
		Metadata:            meta,
	})
	return err
}

// PickTemplateRow chooses a representative config row for grid materialization.
func PickTemplateRow(rows []db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow) (db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow, bool) {
	if len(rows) == 0 {
		return db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow{}, false
	}
	return rows[0], true
}
