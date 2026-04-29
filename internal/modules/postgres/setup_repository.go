package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/setupapp"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SetupRepository implements setupapp.Repository using sqlc queries.
type SetupRepository struct {
	pool *pgxpool.Pool
}

// NewSetupRepository returns a topology / slot-config repository backed by pool.
func NewSetupRepository(pool *pgxpool.Pool) *SetupRepository {
	if pool == nil {
		panic("postgres.NewSetupRepository: nil pool")
	}
	return &SetupRepository{pool: pool}
}

var _ setupapp.Repository = (*SetupRepository)(nil)

func effectiveFromOrNow(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now().UTC()
	}
	return t.UTC()
}

// UpsertMachineTopology upserts cabinets then layout revisions in one transaction.
func (r *SetupRepository) UpsertMachineTopology(ctx context.Context, machineID uuid.UUID, cabinets []setupapp.CabinetUpsert, layouts []setupapp.TopologyLayoutUpsert) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := db.New(tx)
	m, err := q.GetMachineByIDForUpdate(ctx, machineID)
	if err != nil {
		if isNoRows(err) {
			return setupapp.ErrNotFound
		}
		return err
	}

	for _, c := range cabinets {
		code := strings.TrimSpace(c.Code)
		if code == "" {
			return errors.New("postgres: cabinet code is required")
		}
		_, err = q.FleetAdminUpsertMachineCabinet(ctx, db.FleetAdminUpsertMachineCabinetParams{
			MachineID:    machineID,
			CabinetCode:  code,
			Title:        strings.TrimSpace(c.Title),
			SortOrder:    c.SortOrder,
			CabinetIndex: c.SortOrder,
			SlotCapacity: pgtype.Int4{}, // unset; optional per-cabinet capacity for slot-range mapping on reads
			Status:       "active",
			Metadata:     defaultJSONB(c.Metadata),
		})
		if err != nil {
			return err
		}
	}

	for _, lay := range layouts {
		cc := strings.TrimSpace(lay.CabinetCode)
		if cc == "" || strings.TrimSpace(lay.LayoutKey) == "" {
			return errors.New("postgres: layout cabinet_code and layout_key are required")
		}
		cabRow, err := q.FleetAdminGetMachineCabinetByMachineAndCode(ctx, db.FleetAdminGetMachineCabinetByMachineAndCodeParams{
			MachineID:   machineID,
			CabinetCode: cc,
		})
		if err != nil {
			if isNoRows(err) {
				return setupapp.ErrCabinetNotFound
			}
			return err
		}
		_, err = q.FleetAdminUpsertMachineSlotLayout(ctx, db.FleetAdminUpsertMachineSlotLayoutParams{
			OrganizationID:   m.OrganizationID,
			MachineID:        machineID,
			MachineCabinetID: cabRow.ID,
			LayoutKey:        strings.TrimSpace(lay.LayoutKey),
			Revision:         lay.Revision,
			LayoutSpec:       defaultJSONB(lay.LayoutSpec),
			Status:           lay.Status,
		})
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func applySlotConfigSaveTx(ctx context.Context, tx pgx.Tx, machineID uuid.UUID, in setupapp.SlotConfigSaveInput) error {
	q := db.New(tx)
	m, err := q.GetMachineByIDForUpdate(ctx, machineID)
	if err != nil {
		if isNoRows(err) {
			return setupapp.ErrNotFound
		}
		return err
	}

	var legacySnapshot []db.InventoryAdminListMachineSlotsRow
	if in.SyncLegacyReadModel {
		var lsErr error
		legacySnapshot, lsErr = q.InventoryAdminListMachineSlots(ctx, machineID)
		if lsErr != nil {
			return lsErr
		}
	}

	for _, it := range in.Items {
		cabRow, err := q.FleetAdminGetMachineCabinetByMachineAndCode(ctx, db.FleetAdminGetMachineCabinetByMachineAndCodeParams{
			MachineID:   machineID,
			CabinetCode: strings.TrimSpace(it.CabinetCode),
		})
		if err != nil {
			if isNoRows(err) {
				return setupapp.ErrCabinetNotFound
			}
			return err
		}

		layoutRow, err := q.FleetAdminGetMachineSlotLayoutByKey(ctx, db.FleetAdminGetMachineSlotLayoutByKeyParams{
			MachineID:        machineID,
			MachineCabinetID: cabRow.ID,
			LayoutKey:        strings.TrimSpace(it.LayoutKey),
			Revision:         it.LayoutRevision,
		})
		if err != nil {
			if isNoRows(err) {
				return setupapp.ErrSlotLayoutNotFound
			}
			return err
		}

		eff := effectiveFromOrNow(it.EffectiveFrom)
		meta := defaultJSONB(it.Metadata)

		if in.PublishAsCurrent {
			_, err = q.FleetAdminApplyMachineSlotConfigCurrent(ctx, db.FleetAdminApplyMachineSlotConfigCurrentParams{
				OrganizationID:      m.OrganizationID,
				MachineID:           machineID,
				MachineCabinetID:    cabRow.ID,
				MachineSlotLayoutID: layoutRow.ID,
				SlotCode:            strings.TrimSpace(it.SlotCode),
				SlotIndex:           optionalInt32ToPgInt4(it.LegacySlotIndex),
				ProductID:           optionalUUIDToPg(it.ProductID),
				MaxQuantity:         it.MaxQuantity,
				PriceMinor:          it.PriceMinor,
				EffectiveFrom:       eff,
				Metadata:            meta,
			})
		} else {
			_, err = q.FleetAdminInsertMachineSlotConfigDraft(ctx, db.FleetAdminInsertMachineSlotConfigDraftParams{
				OrganizationID:      m.OrganizationID,
				MachineID:           machineID,
				MachineCabinetID:    cabRow.ID,
				MachineSlotLayoutID: layoutRow.ID,
				SlotCode:            strings.TrimSpace(it.SlotCode),
				SlotIndex:           optionalInt32ToPgInt4(it.LegacySlotIndex),
				ProductID:           optionalUUIDToPg(it.ProductID),
				MaxQuantity:         it.MaxQuantity,
				PriceMinor:          it.PriceMinor,
				EffectiveFrom:       eff,
				Metadata:            meta,
			})
		}
		if err != nil {
			return err
		}

		if in.SyncLegacyReadModel && it.LegacySlotIndex != nil {
			var curQty int32
			found := false
			for _, row := range legacySnapshot {
				if row.PlanogramID == in.PlanogramID && row.SlotIndex == *it.LegacySlotIndex {
					curQty = row.CurrentQuantity
					found = true
					break
				}
			}
			if !found {
				curQty = 0
			}
			_, err = q.InventoryAdminUpsertMachineSlotState(ctx, db.InventoryAdminUpsertMachineSlotStateParams{
				MachineID:                machineID,
				PlanogramID:              in.PlanogramID,
				SlotIndex:                *it.LegacySlotIndex,
				CurrentQuantity:          curQty,
				PriceMinor:               it.PriceMinor,
				PlanogramRevisionApplied: in.PlanogramRevision,
			})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// ApplyPublishedSlotConfigsInTx writes machine_slot_configs as draft or current inside an existing transaction.
func (r *SetupRepository) ApplyPublishedSlotConfigsInTx(ctx context.Context, tx pgx.Tx, machineID uuid.UUID, in setupapp.SlotConfigSaveInput) error {
	if len(in.Items) == 0 {
		return nil
	}
	return applySlotConfigSaveTx(ctx, tx, machineID, in)
}

// SaveDraftOrCurrentSlotConfigs writes machine_slot_configs as draft or current and optionally syncs legacy machine_slot_state.
func (r *SetupRepository) SaveDraftOrCurrentSlotConfigs(ctx context.Context, machineID uuid.UUID, in setupapp.SlotConfigSaveInput) error {
	if len(in.Items) == 0 {
		return nil
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := applySlotConfigSaveTx(ctx, tx, machineID, in); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// GetMachineBootstrap loads machine, cabinets, primary assortment products, and current cabinet slot configs.
func (r *SetupRepository) GetMachineBootstrap(ctx context.Context, machineID uuid.UUID) (setupapp.MachineBootstrap, error) {
	q := db.New(r.pool)
	m, err := q.GetMachineByID(ctx, machineID)
	if err != nil {
		if isNoRows(err) {
			return setupapp.MachineBootstrap{}, setupapp.ErrNotFound
		}
		return setupapp.MachineBootstrap{}, err
	}
	if strings.EqualFold(strings.TrimSpace(m.Status), "retired") || strings.EqualFold(strings.TrimSpace(m.Status), "decommissioned") {
		return setupapp.MachineBootstrap{}, setupapp.ErrMachineNotEligibleForBootstrap
	}

	cabs, err := q.FleetAdminListMachineCabinets(ctx, machineID)
	if err != nil {
		return setupapp.MachineBootstrap{}, err
	}

	assortRows, err := q.FleetAdminListAssortmentProductsByMachine(ctx, db.FleetAdminListAssortmentProductsByMachineParams{
		ID:             machineID,
		OrganizationID: m.OrganizationID,
	})
	if err != nil {
		return setupapp.MachineBootstrap{}, err
	}

	cfgRows, err := q.InventoryAdminListCurrentMachineSlotConfigsByMachine(ctx, machineID)
	if err != nil {
		return setupapp.MachineBootstrap{}, err
	}

	var pvID *uuid.UUID
	var pvNo int32
	if meta, perr := q.PlanogramGetPublishedMetaForMachine(ctx, machineID); perr == nil {
		if meta.PublishedPlanogramVersionID.Valid {
			u := uuid.UUID(meta.PublishedPlanogramVersionID.Bytes)
			pvID = &u
		}
		if meta.VersionNo.Valid {
			pvNo = meta.VersionNo.Int32
		}
	}

	cabinets := mapSetupCabinetViews(cabs)
	if len(cabinets) == 0 {
		// Legacy machines may have no machine_cabinets rows; expose a stable default for clients and UIs.
		cabinets = []setupapp.CabinetView{
			{
				ID:        uuid.Nil,
				MachineID: machineID,
				Code:      "CAB-A",
				Title:     "Default cabinet",
				SortOrder: 0,
				Metadata:  []byte("{}"),
				CreatedAt: m.CreatedAt,
				UpdatedAt: m.UpdatedAt,
			},
		}
	}

	out := setupapp.MachineBootstrap{
		Machine:                     mapMachine(m),
		Cabinets:                    cabinets,
		AssortmentProducts:          mapAssortmentProductViews(assortRows),
		CurrentCabinetSlots:         mapCabinetSlotConfigViews(cfgRows),
		PublishedPlanogramVersionID: pvID,
		PublishedPlanogramVersionNo: pvNo,
	}
	return out, nil
}

// GetMachineSlotView returns legacy planogram slot rows plus current cabinet slot config rows.
func (r *SetupRepository) GetMachineSlotView(ctx context.Context, machineID uuid.UUID) (setupapp.MachineSlotView, error) {
	q := db.New(r.pool)
	legacy, err := q.InventoryAdminListMachineSlots(ctx, machineID)
	if err != nil {
		return setupapp.MachineSlotView{}, err
	}
	cfg, err := q.InventoryAdminListCurrentMachineSlotConfigsByMachine(ctx, machineID)
	if err != nil {
		return setupapp.MachineSlotView{}, err
	}
	return setupapp.MachineSlotView{
		LegacySlots:     mapLegacySlotRows(legacy),
		ConfiguredSlots: mapConfiguredSlotRows(cfg),
	}, nil
}

func mapSetupCabinetViews(rows []db.MachineCabinet) []setupapp.CabinetView {
	out := make([]setupapp.CabinetView, 0, len(rows))
	for _, row := range rows {
		out = append(out, setupapp.CabinetView{
			ID:        row.ID,
			MachineID: row.MachineID,
			Code:      row.CabinetCode,
			Title:     row.Title,
			SortOrder: row.SortOrder,
			Metadata:  row.Metadata,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		})
	}
	return out
}

func mapAssortmentProductViews(rows []db.FleetAdminListAssortmentProductsByMachineRow) []setupapp.AssortmentProductView {
	out := make([]setupapp.AssortmentProductView, 0, len(rows))
	for _, row := range rows {
		out = append(out, setupapp.AssortmentProductView{
			ProductID:      row.ProductID,
			SKU:            row.ProductSku,
			Name:           row.ProductName,
			SortOrder:      row.AssortmentItemSortOrder,
			AssortmentID:   row.AssortmentID,
			AssortmentName: row.AssortmentName,
		})
	}
	return out
}

func mapCabinetSlotConfigViews(rows []db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow) []setupapp.CabinetSlotConfigView {
	out := make([]setupapp.CabinetSlotConfigView, 0, len(rows))
	for _, row := range rows {
		var slotIdx *int32
		if row.SlotIndex.Valid {
			v := row.SlotIndex.Int32
			slotIdx = &v
		}
		var pid *uuid.UUID
		if row.ProductID.Valid {
			u := uuid.UUID(row.ProductID.Bytes)
			pid = &u
		}
		out = append(out, setupapp.CabinetSlotConfigView{
			ConfigID:          row.ID,
			CabinetCode:       row.CabinetCode,
			SlotCode:          row.SlotCode,
			SlotIndex:         slotIdx,
			ProductID:         pid,
			ProductSKU:        pgTextToString(row.ProductSku),
			ProductName:       pgTextToString(row.ProductName),
			MaxQuantity:       row.MaxQuantity,
			PriceMinor:        row.PriceMinor,
			EffectiveFrom:     row.EffectiveFrom,
			IsCurrent:         row.IsCurrent,
			MachineSlotLayout: row.MachineSlotLayoutID,
		})
	}
	return out
}

func mapLegacySlotRows(rows []db.InventoryAdminListMachineSlotsRow) []setupapp.LegacySlotRow {
	out := make([]setupapp.LegacySlotRow, 0, len(rows))
	for _, row := range rows {
		var pid *uuid.UUID
		if row.ProductID.Valid {
			u := uuid.UUID(row.ProductID.Bytes)
			pid = &u
		}
		out = append(out, setupapp.LegacySlotRow{
			PlanogramID:       row.PlanogramID,
			PlanogramName:     row.PlanogramName,
			SlotIndex:         row.SlotIndex,
			CurrentQuantity:   row.CurrentQuantity,
			MaxQuantity:       row.MaxQuantity,
			PriceMinor:        row.PriceMinor,
			ProductID:         pid,
			ProductSKU:        pgTextToString(row.ProductSku),
			ProductName:       pgTextToString(row.ProductName),
			PlanogramRevision: row.PlanogramRevisionApplied,
		})
	}
	return out
}

func mapConfiguredSlotRows(rows []db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow) []setupapp.ConfiguredSlotRow {
	out := make([]setupapp.ConfiguredSlotRow, 0, len(rows))
	for _, row := range rows {
		var slotIdx *int32
		if row.SlotIndex.Valid {
			v := row.SlotIndex.Int32
			slotIdx = &v
		}
		var pid *uuid.UUID
		if row.ProductID.Valid {
			u := uuid.UUID(row.ProductID.Bytes)
			pid = &u
		}
		out = append(out, setupapp.ConfiguredSlotRow{
			CabinetCode: row.CabinetCode,
			SlotCode:    row.SlotCode,
			SlotIndex:   slotIdx,
			ProductID:   pid,
			ProductSKU:  pgTextToString(row.ProductSku),
			ProductName: pgTextToString(row.ProductName),
			MaxQuantity: row.MaxQuantity,
			PriceMinor:  row.PriceMinor,
		})
	}
	return out
}
