package fleet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/physicaltopology"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/pgjson"
	"github.com/avf/avf-vending-api/internal/platform/pgxutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultCommerceCabinetCode    = "CAB-A"
	defaultCommerceLayoutKey      = "default"
	defaultCommerceLayoutRevision = int32(1)
)

// CommerceTopologyMaterializeResult counts rows created during idempotent materialization.
type CommerceTopologyMaterializeResult struct {
	CabinetsCreated    int
	SlotLayoutsCreated int
	SlotConfigsCreated int
	SlotConfigsUpdated int
}

// MaterializeCommerceTopologyInTx ensures cabinet CAB-A, slot layout default:1, and stub current slot configs exist.
func MaterializeCommerceTopologyInTx(
	ctx context.Context,
	tx pgx.Tx,
	machineID uuid.UUID,
	gridRows, gridCols int32,
	materializedBy string,
) (CommerceTopologyMaterializeResult, error) {
	var out CommerceTopologyMaterializeResult
	if machineID == uuid.Nil {
		return out, fmt.Errorf("machineId is required")
	}
	if gridRows < 1 || gridCols < 1 {
		gridRows, gridCols = defaultBootstrapGridRows, defaultBootstrapGridCols
	}
	q := pgxutil.NewQueries(tx)

	cabs, err := q.FleetAdminListMachineCabinets(ctx, machineID)
	if err != nil {
		return out, err
	}
	var cabRow db.MachineCabinet
	for _, c := range cabs {
		if strings.EqualFold(strings.TrimSpace(c.CabinetCode), defaultCommerceCabinetCode) {
			cabRow = c
			break
		}
	}
	if cabRow.ID == uuid.Nil {
		row, upsertErr := q.FleetAdminUpsertMachineCabinet(ctx, db.FleetAdminUpsertMachineCabinetParams{
			MachineID:    machineID,
			CabinetCode:  defaultCommerceCabinetCode,
			Title:        "Default cabinet",
			SortOrder:    0,
			CabinetIndex: 0,
			Status:       "active",
			Metadata:     pgjson.RequiredString([]byte(`{"materializedBy":"` + materializedBy + `"}`)),
		})
		if upsertErr != nil {
			return out, upsertErr
		}
		cabRow = row
		out.CabinetsCreated = 1
	}

	layoutSpec, _ := json.Marshal(map[string]any{
		"rows":           gridRows,
		"cols":           gridCols,
		"materializedBy": materializedBy,
	})
	layoutLayoutID := uuid.UUID{}
	layoutRow, layoutErr := q.FleetAdminGetMachineSlotLayoutByKey(ctx, db.FleetAdminGetMachineSlotLayoutByKeyParams{
		MachineID:        machineID,
		MachineCabinetID: cabRow.ID,
		LayoutKey:        defaultCommerceLayoutKey,
		Revision:         defaultCommerceLayoutRevision,
	})
	if layoutErr != nil {
		if !errors.Is(layoutErr, pgx.ErrNoRows) {
			return out, layoutErr
		}
		inserted, insertErr := q.FleetAdminUpsertMachineSlotLayout(ctx, db.FleetAdminUpsertMachineSlotLayoutParams{
			MachineID:        machineID,
			MachineCabinetID: cabRow.ID,
			LayoutKey:        defaultCommerceLayoutKey,
			Revision:         defaultCommerceLayoutRevision,
			LayoutSpec:       pgjson.RequiredString(layoutSpec),
			Status:           "published",
		})
		if insertErr != nil {
			return out, insertErr
		}
		layoutLayoutID = inserted.ID
		out.SlotLayoutsCreated = 1
	} else {
		layoutLayoutID = layoutRow.ID
	}

	existing, err := q.InventoryAdminListCurrentMachineSlotConfigsByMachine(ctx, machineID)
	if err != nil {
		return out, err
	}
	byCode := make(map[string]db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow, len(existing))
	for _, row := range existing {
		byCode[strings.TrimSpace(row.SlotCode)] = row
	}

	metaBytes, _ := json.Marshal(map[string]any{"materializedBy": materializedBy})
	meta := string(metaBytes)
	eff := time.Now().UTC()
	codes := physicaltopology.AllSlotCodes(int(gridRows), int(gridCols))
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		if existing, ok := byCode[code]; ok {
			idx := physicaltopology.SlotIndexFromCode(code, int(gridCols))
			slotIdx := pgtype.Int4{Int32: idx, Valid: idx > 0}
			if currentSlotConfigMatchesDesired(existing, cabRow.ID, layoutLayoutID, slotIdx, pgtype.UUID{Valid: false}, defaultSlotCapacity, 0) {
				continue
			}
			if existing.MachineCabinetID != cabRow.ID || existing.MachineSlotLayoutID != layoutLayoutID || !pgInt4Equal(existing.SlotIndex, slotIdx) {
				if err := relinkCurrentMachineSlotConfigInTx(ctx, tx, existing.ID, cabRow.ID, layoutLayoutID, slotIdx, meta); err != nil {
					return out, err
				}
				out.SlotConfigsUpdated++
				continue
			}
			continue
		}
		idx := physicaltopology.SlotIndexFromCode(code, int(gridCols))
		_, applyErr := q.FleetAdminApplyMachineSlotConfigCurrent(ctx, db.FleetAdminApplyMachineSlotConfigCurrentParams{
			MachineID:           machineID,
			SlotCode:            code,
			MachineCabinetID:    cabRow.ID,
			MachineSlotLayoutID: layoutLayoutID,
			SlotIndex:           pgtype.Int4{Int32: idx, Valid: idx > 0},
			ProductID:           pgtype.UUID{Valid: false},
			MaxQuantity:         defaultSlotCapacity,
			PriceMinor:          0,
			EffectiveFrom:       eff,
			Metadata:            meta,
		})
		if applyErr != nil {
			return out, applyErr
		}
		out.SlotConfigsCreated++
	}

	return out, nil
}

func pgUUIDEqual(a, b pgtype.UUID) bool {
	if !a.Valid && !b.Valid {
		return true
	}
	if a.Valid != b.Valid {
		return false
	}
	return uuid.UUID(a.Bytes) == uuid.UUID(b.Bytes)
}

func pgInt4Equal(a, b pgtype.Int4) bool {
	if !a.Valid && !b.Valid {
		return true
	}
	if a.Valid != b.Valid {
		return false
	}
	return a.Int32 == b.Int32
}

func currentSlotConfigMatchesDesired(
	existing db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow,
	cabID, layoutID uuid.UUID,
	slotIdx pgtype.Int4,
	pid pgtype.UUID,
	maxQty int32,
	priceMinor int64,
) bool {
	if existing.MachineCabinetID != cabID {
		return false
	}
	if existing.MachineSlotLayoutID != layoutID {
		return false
	}
	if !pgInt4Equal(existing.SlotIndex, slotIdx) {
		return false
	}
	if !pgUUIDEqual(existing.ProductID, pid) {
		return false
	}
	if existing.MaxQuantity != maxQty {
		return false
	}
	if existing.PriceMinor != priceMinor {
		return false
	}
	return true
}

func relinkCurrentMachineSlotConfigInTx(
	ctx context.Context,
	tx pgx.Tx,
	configID uuid.UUID,
	cabID, layoutID uuid.UUID,
	slotIdx pgtype.Int4,
	meta string,
) error {
	var slotIndex any
	if slotIdx.Valid {
		slotIndex = slotIdx.Int32
	}
	_, err := tx.Exec(ctx, `
UPDATE machine_slot_configs
SET
  machine_cabinet_id = $2,
  machine_slot_layout_id = $3,
  slot_index = $4,
  metadata = COALESCE(NULLIF($5::text, '')::jsonb, metadata),
  updated_at = now()
WHERE id = $1 AND is_current = true
`, configID, cabID, layoutID, slotIndex, meta)
	return err
}

// ApplyOrRelinkCurrentMachineSlotConfig skips matching current configs, relinks cabinet/layout
// when commerce fields are unchanged, or applies a new current row when commerce changed.
func ApplyOrRelinkCurrentMachineSlotConfig(
	ctx context.Context,
	tx pgx.Tx,
	existingByCode map[string]db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow,
	machineID uuid.UUID,
	slotCode string,
	cabID, layoutID uuid.UUID,
	slotIdx pgtype.Int4,
	pid pgtype.UUID,
	maxQty int32,
	priceMinor int64,
	effectiveFrom time.Time,
	meta string,
) (created bool, updated bool, err error) {
	if existing, ok := existingByCode[slotCode]; ok {
		if currentSlotConfigMatchesDesired(existing, cabID, layoutID, slotIdx, pid, maxQty, priceMinor) {
			return false, false, nil
		}
		needsRelink := existing.MachineCabinetID != cabID ||
			existing.MachineSlotLayoutID != layoutID ||
			!pgInt4Equal(existing.SlotIndex, slotIdx)
		commerceUnchanged := pgUUIDEqual(existing.ProductID, pid) &&
			existing.MaxQuantity == maxQty &&
			existing.PriceMinor == priceMinor
		if needsRelink && commerceUnchanged {
			if err := relinkCurrentMachineSlotConfigInTx(ctx, tx, existing.ID, cabID, layoutID, slotIdx, meta); err != nil {
				return false, false, err
			}
			return false, true, nil
		}
	}
	q := pgxutil.NewQueries(tx)
	_, err = q.FleetAdminApplyMachineSlotConfigCurrent(ctx, db.FleetAdminApplyMachineSlotConfigCurrentParams{
		MachineID:           machineID,
		SlotCode:            slotCode,
		MachineCabinetID:    cabID,
		MachineSlotLayoutID: layoutID,
		SlotIndex:           slotIdx,
		ProductID:           pid,
		MaxQuantity:         maxQty,
		PriceMinor:          priceMinor,
		EffectiveFrom:       effectiveFrom,
		Metadata:            meta,
	})
	if err != nil {
		return false, false, err
	}
	return true, false, nil
}

func SyncNamedLayoutSlotsToCurrentConfigs(
	ctx context.Context,
	tx pgx.Tx,
	machineID uuid.UUID,
	layoutID uuid.UUID,
	materializedBy string,
) (int, error) {
	if machineID == uuid.Nil || layoutID == uuid.Nil {
		return 0, fmt.Errorf("machineId and layoutId are required")
	}
	q := pgxutil.NewQueries(tx)
	layout, err := q.GetMachineLayoutByID(ctx, db.GetMachineLayoutByIDParams{
		ID:        layoutID,
		MachineID: machineID,
	})
	if err != nil {
		return 0, err
	}

	mat, err := MaterializeCommerceTopologyInTx(ctx, tx, machineID, layout.GridRows, layout.GridCols, materializedBy)
	if err != nil {
		return 0, err
	}
	_ = mat

	cabs, err := q.FleetAdminListMachineCabinets(ctx, machineID)
	if err != nil {
		return 0, err
	}
	var cabID uuid.UUID
	for _, c := range cabs {
		if strings.EqualFold(strings.TrimSpace(c.CabinetCode), defaultCommerceCabinetCode) {
			cabID = c.ID
			break
		}
	}
	if cabID == uuid.Nil {
		return 0, fmt.Errorf("commerce cabinet %q not found after materialize", defaultCommerceCabinetCode)
	}
	layoutRow, err := q.FleetAdminGetMachineSlotLayoutByKey(ctx, db.FleetAdminGetMachineSlotLayoutByKeyParams{
		MachineID:        machineID,
		MachineCabinetID: cabID,
		LayoutKey:        defaultCommerceLayoutKey,
		Revision:         defaultCommerceLayoutRevision,
	})
	if err != nil {
		return 0, err
	}

	slots, err := q.ListMachineLayoutSlots(ctx, layoutID)
	if err != nil {
		return 0, err
	}

	existingRows, err := q.InventoryAdminListCurrentMachineSlotConfigsByMachine(ctx, machineID)
	if err != nil {
		return 0, err
	}
	existingByCode := make(map[string]db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow, len(existingRows))
	for _, row := range existingRows {
		existingByCode[strings.TrimSpace(row.SlotCode)] = row
	}

	metaBytes, _ := json.Marshal(map[string]any{"materializedBy": materializedBy, "sourceLayoutId": layoutID.String()})
	meta := string(metaBytes)
	eff := time.Now().UTC()
	updated := 0
	for _, slot := range slots {
		code := strings.TrimSpace(slot.SlotCode)
		if code == "" {
			continue
		}
		var slotIdx pgtype.Int4
		if slot.PhysicalLane.Valid && slot.PhysicalLane.Int32 > 0 {
			slotIdx = slot.PhysicalLane
		} else {
			idx := physicaltopology.SlotIndexFromCode(code, int(layout.GridCols))
			slotIdx = pgtype.Int4{Int32: idx, Valid: idx > 0}
		}
		var pid pgtype.UUID
		if slot.ProductID.Valid {
			pid = slot.ProductID
		}
		price := int64(0)
		if slot.PriceMinor.Valid {
			price = slot.PriceMinor.Int64
		}
		maxQty := slot.MaxQuantity
		if maxQty < 1 {
			maxQty = defaultSlotCapacity
		}
		if existing, ok := existingByCode[code]; ok &&
			currentSlotConfigMatchesDesired(existing, cabID, layoutRow.ID, slotIdx, pid, maxQty, price) {
			updated++
			continue
		}
		created, relinked, applyErr := ApplyOrRelinkCurrentMachineSlotConfig(
			ctx, tx, existingByCode, machineID, code, cabID, layoutRow.ID,
			slotIdx, pid, maxQty, price, eff, meta,
		)
		if applyErr != nil {
			return updated, applyErr
		}
		if created || relinked {
			updated++
		}
	}
	return updated, nil
}

// SyncAssortmentFromCurrentSlotConfigs ensures the machine primary published assortment
// includes every product assigned on current slot configs.
func SyncAssortmentFromCurrentSlotConfigs(ctx context.Context, pool *pgxpool.Pool, machineID uuid.UUID) error {
	if pool == nil || machineID == uuid.Nil {
		return fmt.Errorf("invalid pool or machineId")
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := syncAssortmentFromCurrentSlotConfigsInTx(ctx, tx, machineID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func syncAssortmentFromCurrentSlotConfigsInTx(ctx context.Context, tx pgx.Tx, machineID uuid.UUID) error {
	q := pgxutil.NewQueries(tx)
	rows, err := q.InventoryAdminListCurrentMachineSlotConfigsByMachine(ctx, machineID)
	if err != nil {
		return err
	}
	pidSeen := map[uuid.UUID]struct{}{}
	for _, row := range rows {
		if !row.ProductID.Valid {
			continue
		}
		pid := uuid.UUID(row.ProductID.Bytes)
		if pid == uuid.Nil {
			continue
		}
		pidSeen[pid] = struct{}{}
	}
	if len(pidSeen) == 0 {
		return nil
	}
	pids := make([]uuid.UUID, 0, len(pidSeen))
	for p := range pidSeen {
		pids = append(pids, p)
	}
	sort.Slice(pids, func(i, j int) bool { return pids[i].String() < pids[j].String() })

	asmRows, err := q.FleetAdminListAssortmentProductsByMachine(ctx, machineID)
	if err != nil {
		return err
	}
	var assortmentID uuid.UUID
	if len(asmRows) == 0 {
		asm, err := q.FleetAdminInsertAssortment(ctx, db.FleetAdminInsertAssortmentParams{
			Name:        fmt.Sprintf("Published slots — machine %s", machineID.String()),
			Status:      "published",
			Description: "Auto-created during commerce topology reconcile.",
			Meta:        pgjson.RequiredString([]byte(`{}`)),
		})
		if err != nil {
			return err
		}
		assortmentID = asm.ID
		n, err := q.FleetAdminBindMachinePrimaryAssortment(ctx, db.FleetAdminBindMachinePrimaryAssortmentParams{
			ID:           machineID,
			AssortmentID: assortmentID,
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("fleet: bind primary assortment affected 0 rows (machine=%s)", machineID)
		}
	} else {
		assortmentID = asmRows[0].AssortmentID
	}
	for i, pid := range pids {
		if _, err := q.FleetAdminUpsertAssortmentItem(ctx, db.FleetAdminUpsertAssortmentItemParams{
			AssortmentID: assortmentID,
			ProductID:    pid,
			SortOrder:    int32(i),
			Notes:        pgjson.RequiredString([]byte(`{}`)),
		}); err != nil {
			return err
		}
	}
	return nil
}

// ReconcileCommerceTopology ensures commerce topology exists and syncs named layout slot assignments when present.
func ReconcileCommerceTopology(ctx context.Context, pool *pgxpool.Pool, machineID uuid.UUID, dryRun bool) (CommerceTopologyMaterializeResult, int, error) {
	var mat CommerceTopologyMaterializeResult
	var synced int
	if pool == nil {
		return mat, synced, fmt.Errorf("database pool is not configured")
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return mat, synced, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := pgxutil.NewQueries(tx)
	machine, err := q.GetMachineByID(ctx, machineID)
	if err != nil {
		return mat, synced, err
	}

	gridRows, gridCols := defaultBootstrapGridRows, defaultBootstrapGridCols
	layoutID := uuid.Nil
	if machine.ActiveLayoutID.Valid {
		layoutID = uuid.UUID(machine.ActiveLayoutID.Bytes)
		layout, layErr := q.GetMachineLayoutByID(ctx, db.GetMachineLayoutByIDParams{
			ID:        layoutID,
			MachineID: machineID,
		})
		if layErr == nil {
			gridRows, gridCols = layout.GridRows, layout.GridCols
		}
	}

	if dryRun {
		cabs, _ := q.FleetAdminListMachineCabinets(ctx, machineID)
		hasCab := false
		for _, c := range cabs {
			if strings.EqualFold(strings.TrimSpace(c.CabinetCode), defaultCommerceCabinetCode) {
				hasCab = true
				break
			}
		}
		if !hasCab {
			mat.CabinetsCreated = 1
			mat.SlotLayoutsCreated = 1
		}
		cfgRows, _ := q.InventoryAdminListCurrentMachineSlotConfigsByMachine(ctx, machineID)
		want := len(physicaltopology.AllSlotCodes(int(gridRows), int(gridCols)))
		if len(cfgRows) < want {
			mat.SlotConfigsCreated = want - len(cfgRows)
		}
		if layoutID != uuid.Nil {
			slots, _ := q.ListMachineLayoutSlots(ctx, layoutID)
			synced = len(slots)
		}
		return mat, synced, nil
	}

	mat, err = MaterializeCommerceTopologyInTx(ctx, tx, machineID, gridRows, gridCols, "reconcile")
	if err != nil {
		return mat, synced, err
	}
	if layoutID != uuid.Nil {
		synced, err = SyncNamedLayoutSlotsToCurrentConfigs(ctx, tx, machineID, layoutID, "reconcile")
		if err != nil {
			return mat, synced, err
		}
	}
	if err := syncAssortmentFromCurrentSlotConfigsInTx(ctx, tx, machineID); err != nil {
		return mat, synced, err
	}

	if err := tx.Commit(ctx); err != nil {
		return mat, synced, err
	}
	slog.Info("COMMERCE_TOPOLOGY_RECONCILE",
		"machine_id", machineID.String(),
		"cabinets_created", mat.CabinetsCreated,
		"slot_layouts_created", mat.SlotLayoutsCreated,
		"slot_configs_created", mat.SlotConfigsCreated,
		"layout_slots_synced", synced,
	)
	return mat, synced, nil
}

// CommerceReadinessView summarizes whether commerce topology is materialized for admin UI.
type CommerceReadinessView struct {
	CabinetCount           int64 `json:"cabinetCount"`
	SlotLayoutCount        int64 `json:"slotLayoutCount"`
	CurrentSlotConfigCount int64 `json:"currentSlotConfigCount"`
	SnapshotHistoryCount   int64 `json:"snapshotHistoryCount"`
	NeedsReconcile         bool  `json:"needsReconcile"`
}

// LoadCommerceReadiness returns read-only commerce topology counts for a machine.
func LoadCommerceReadiness(ctx context.Context, pool *pgxpool.Pool, machineID uuid.UUID) (CommerceReadinessView, error) {
	var out CommerceReadinessView
	if pool == nil || machineID == uuid.Nil {
		return out, fmt.Errorf("invalid pool or machineId")
	}
	q := pgxutil.NewQueries(pool)
	cabs, err := q.FleetAdminListMachineCabinets(ctx, machineID)
	if err != nil {
		return out, err
	}
	out.CabinetCount = int64(len(cabs))
	var layoutCount int64
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM machine_slot_layouts WHERE machine_id = $1`, machineID).Scan(&layoutCount); err != nil {
		return out, err
	}
	out.SlotLayoutCount = layoutCount
	cfgRows, err := q.InventoryAdminListCurrentMachineSlotConfigsByMachine(ctx, machineID)
	if err != nil {
		return out, err
	}
	out.CurrentSlotConfigCount = int64(len(cfgRows))
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM machine_layout_snapshot_history WHERE machine_id = $1`, machineID).Scan(&out.SnapshotHistoryCount); err != nil {
		return out, err
	}
	machine, err := q.GetMachineByID(ctx, machineID)
	if err != nil {
		return out, err
	}
	hasActiveLayout := machine.ActiveLayoutID.Valid
	out.NeedsReconcile = hasActiveLayout && (out.CabinetCount == 0 || out.SlotLayoutCount == 0 || out.CurrentSlotConfigCount == 0)
	return out, nil
}
