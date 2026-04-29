package planogram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	appaudit "github.com/avf/avf-vending-api/internal/app/audit"
	appdevice "github.com/avf/avf-vending-api/internal/app/device"
	"github.com/avf/avf-vending-api/internal/app/setupapp"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	domaindevice "github.com/avf/avf-vending-api/internal/domain/device"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	draftStatusEditing   = "editing"
	draftStatusValidated = "validated"

	auditActionPublish  = "machine.planogram_publish"
	auditActionRollback = "machine.planogram_rollback"
)

// Service manages enterprise planogram drafts and immutable published versions.
type Service struct {
	pool        *pgxpool.Pool
	setup       *postgres.SetupRepository
	remote      *appdevice.MQTTCommandDispatcher
	audit       *appaudit.Service
	commandType string
}

// Deps wires persistence and optional MQTT dispatch / audit.
type Deps struct {
	Pool           *pgxpool.Pool
	Setup          *postgres.SetupRepository
	RemoteCommands *appdevice.MQTTCommandDispatcher
	Audit          *appaudit.Service
	CommandType    string
}

// NewService returns a planogram service or panics when Pool or Setup is nil.
func NewService(d Deps) *Service {
	if d.Pool == nil || d.Setup == nil {
		panic("planogram.NewService: Pool and Setup are required")
	}
	ct := strings.TrimSpace(d.CommandType)
	if ct == "" {
		ct = "machine_planogram_publish"
	}
	return &Service{
		pool:        d.Pool,
		setup:       d.Setup,
		remote:      d.RemoteCommands,
		audit:       d.Audit,
		commandType: ct,
	}
}

// Summary is GET …/planogram.
type Summary struct {
	Published *PublishedInfo `json:"published,omitempty"`
	Drafts    []DraftRow     `json:"drafts"`
}

// PublishedInfo is the active published immutable version pointer for the machine.
type PublishedInfo struct {
	VersionID   uuid.UUID `json:"versionId"`
	VersionNo   int32     `json:"versionNo"`
	PublishedAt time.Time `json:"publishedAt"`
}

// DraftRow is a mutable draft snapshot.
type DraftRow struct {
	ID        uuid.UUID       `json:"id"`
	Status    string          `json:"status"`
	Snapshot  json.RawMessage `json:"snapshot"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

// VersionListItem is GET …/planogram/versions.
type VersionListItem struct {
	ID            uuid.UUID       `json:"id"`
	VersionNo     int32           `json:"versionNo"`
	PublishedAt   time.Time       `json:"publishedAt"`
	SourceDraftID *uuid.UUID      `json:"sourceDraftId,omitempty"`
	Snapshot      json.RawMessage `json:"snapshot"`
}

// PublishResult is returned after publish or rollback-driven config bump.
type PublishResult struct {
	DraftID                     uuid.UUID `json:"draftId,omitempty"`
	PublishedPlanogramVersionID uuid.UUID `json:"publishedPlanogramVersionId"`
	VersionNo                   int32     `json:"versionNo"`
	DesiredConfigVersion        int32     `json:"desiredConfigVersion"`
	PlanogramID                 string    `json:"planogramId"`
	PlanogramRevision           int32     `json:"planogramRevision"`
}

func (s *Service) auditRecord(ctx context.Context, orgID uuid.UUID, action string, machineID uuid.UUID, meta map[string]any) {
	if s.audit == nil {
		return
	}
	md, _ := json.Marshal(meta)
	rid := machineID.String()
	_ = s.audit.Record(ctx, compliance.EnterpriseAuditRecord{
		OrganizationID: orgID,
		ActorType:      compliance.ActorUser,
		Action:         action,
		ResourceType:   "machine_planogram",
		ResourceID:     &rid,
		Metadata:       md,
	})
}

// GetSummary returns published pointer (if any) and drafts for the machine.
func (s *Service) GetSummary(ctx context.Context, orgID, machineID uuid.UUID) (*Summary, error) {
	q := db.New(s.pool)
	meta, err := q.PlanogramGetPublishedMetaForMachine(ctx, machineID)
	if err != nil {
		return nil, err
	}
	var pub *PublishedInfo
	if meta.PublishedPlanogramVersionID.Valid {
		vrow, err := q.PlanogramGetVersionByIDForMachine(ctx, db.PlanogramGetVersionByIDForMachineParams{
			ID:             uuid.UUID(meta.PublishedPlanogramVersionID.Bytes),
			OrganizationID: orgID,
			MachineID:      machineID,
		})
		if err == nil {
			pub = &PublishedInfo{
				VersionID:   vrow.ID,
				VersionNo:   vrow.VersionNo,
				PublishedAt: vrow.PublishedAt,
			}
		}
	}

	drafts, err := q.PlanogramListDraftsForMachine(ctx, db.PlanogramListDraftsForMachineParams{
		OrganizationID: orgID,
		MachineID:      machineID,
	})
	if err != nil {
		return nil, err
	}
	outDrafts := make([]DraftRow, 0, len(drafts))
	for _, d := range drafts {
		outDrafts = append(outDrafts, DraftRow{
			ID:        d.ID,
			Status:    d.Status,
			Snapshot:  d.Snapshot,
			CreatedAt: d.CreatedAt,
			UpdatedAt: d.UpdatedAt,
		})
	}
	return &Summary{Published: pub, Drafts: outDrafts}, nil
}

// CreateDraft inserts a draft row (does not change runtime slot configs).
func (s *Service) CreateDraft(ctx context.Context, orgID, machineID uuid.UUID, snapshot json.RawMessage) (uuid.UUID, error) {
	if len(snapshot) == 0 || !json.Valid(snapshot) {
		return uuid.Nil, fmt.Errorf("%w: snapshot JSON required", ErrInvalidSnapshot)
	}
	if _, err := snapshotBytesToSaveInput(snapshot, false); err != nil {
		return uuid.Nil, err
	}
	row, err := db.New(s.pool).PlanogramInsertDraft(ctx, db.PlanogramInsertDraftParams{
		OrganizationID: orgID,
		MachineID:      machineID,
		Status:         draftStatusEditing,
		Snapshot:       []byte(snapshot),
	})
	if err != nil {
		return uuid.Nil, err
	}
	return row.ID, nil
}

// PatchDraft replaces draft snapshot and optionally status.
func (s *Service) PatchDraft(ctx context.Context, orgID, machineID, draftID uuid.UUID, snapshot json.RawMessage, status *string) error {
	q := db.New(s.pool)
	prev, err := q.PlanogramGetMachineDraftByID(ctx, db.PlanogramGetMachineDraftByIDParams{
		ID:             draftID,
		OrganizationID: orgID,
		MachineID:      machineID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	nextSnap := prev.Snapshot
	if len(snapshot) > 0 {
		if !json.Valid(snapshot) {
			return fmt.Errorf("%w: snapshot must be JSON", ErrInvalidSnapshot)
		}
		if _, err := snapshotBytesToSaveInput(snapshot, false); err != nil {
			return err
		}
		nextSnap = []byte(snapshot)
	}
	st := prev.Status
	if status != nil && strings.TrimSpace(*status) != "" {
		st = strings.TrimSpace(*status)
		if st != draftStatusEditing && st != draftStatusValidated {
			return fmt.Errorf("%w: invalid status", ErrInvalidSnapshot)
		}
	}
	_, err = q.PlanogramPatchDraftSnapshot(ctx, db.PlanogramPatchDraftSnapshotParams{
		ID:             draftID,
		OrganizationID: orgID,
		MachineID:      machineID,
		Snapshot:       nextSnap,
		Status:         st,
	})
	return err
}

// ValidateDraft runs validation rules and marks the draft validated.
func (s *Service) ValidateDraft(ctx context.Context, orgID, machineID, draftID uuid.UUID) error {
	q := db.New(s.pool)
	dr, err := q.PlanogramGetMachineDraftByID(ctx, db.PlanogramGetMachineDraftByIDParams{
		ID:             draftID,
		OrganizationID: orgID,
		MachineID:      machineID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	save, err := snapshotBytesToSaveInput(dr.Snapshot, true)
	if err != nil {
		return err
	}
	if err := validatePublishSnapshot(ctx, q, orgID, machineID, save); err != nil {
		return err
	}
	_, err = q.PlanogramPatchDraftSnapshot(ctx, db.PlanogramPatchDraftSnapshotParams{
		ID:             draftID,
		OrganizationID: orgID,
		MachineID:      machineID,
		Snapshot:       dr.Snapshot,
		Status:         draftStatusValidated,
	})
	return err
}

func validatePublishSnapshot(ctx context.Context, q *db.Queries, orgID, machineID uuid.UUID, save setupapp.SlotConfigSaveInput) error {
	if len(save.Items) == 0 {
		return fmt.Errorf("%w: at least one slot item is required to publish", ErrValidation)
	}
	assort, err := q.FleetAdminListAssortmentProductsByMachine(ctx, db.FleetAdminListAssortmentProductsByMachineParams{
		ID:             machineID,
		OrganizationID: orgID,
	})
	if err != nil {
		return err
	}
	allowed := make(map[uuid.UUID]struct{}, len(assort))
	for _, p := range assort {
		allowed[p.ProductID] = struct{}{}
	}
	seen := make(map[string]struct{}, len(save.Items))
	for _, it := range save.Items {
		if strings.TrimSpace(it.CabinetCode) == "" || strings.TrimSpace(it.LayoutKey) == "" || strings.TrimSpace(it.SlotCode) == "" {
			return fmt.Errorf("%w: cabinetCode, layoutKey, and slotCode are required", ErrValidation)
		}
		key := strings.TrimSpace(it.CabinetCode) + "|" + strings.TrimSpace(it.LayoutKey) + "|" + fmt.Sprint(it.LayoutRevision) + "|" + strings.TrimSpace(it.SlotCode)
		if _, dup := seen[key]; dup {
			return fmt.Errorf("%w: duplicate slot key %s", ErrValidation, key)
		}
		seen[key] = struct{}{}
		if it.PriceMinor < 0 {
			return fmt.Errorf("%w: priceMinor must be >= 0", ErrValidation)
		}
		if it.ProductID != nil {
			if _, ok := allowed[*it.ProductID]; !ok {
				return fmt.Errorf("%w: product %s is not in the machine assortment", ErrValidation, it.ProductID.String())
			}
			if it.MaxQuantity <= 0 {
				return fmt.Errorf("%w: maxQuantity must be > 0 when a product is assigned", ErrValidation)
			}
		}
		cabRow, err := q.FleetAdminGetMachineCabinetByMachineAndCode(ctx, db.FleetAdminGetMachineCabinetByMachineAndCodeParams{
			MachineID:   machineID,
			CabinetCode: strings.TrimSpace(it.CabinetCode),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("%w: cabinet %s not found", ErrValidation, strings.TrimSpace(it.CabinetCode))
			}
			return err
		}
		_, err = q.FleetAdminGetMachineSlotLayoutByKey(ctx, db.FleetAdminGetMachineSlotLayoutByKeyParams{
			MachineID:        machineID,
			MachineCabinetID: cabRow.ID,
			LayoutKey:        strings.TrimSpace(it.LayoutKey),
			Revision:         it.LayoutRevision,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("%w: slot layout not found for cabinet %s key %s rev %d", ErrValidation, strings.TrimSpace(it.CabinetCode), strings.TrimSpace(it.LayoutKey), it.LayoutRevision)
			}
			return err
		}
	}
	return nil
}

// PublishDraft validates, writes an immutable version, applies runtime configs, snapshots machine_configs, and dispatches MQTT.
func (s *Service) PublishDraft(ctx context.Context, orgID, machineID, draftID uuid.UUID, idempotencyKey string, actorAccountID *uuid.UUID) (PublishResult, error) {
	q := db.New(s.pool)
	dr, err := q.PlanogramGetMachineDraftByID(ctx, db.PlanogramGetMachineDraftByIDParams{
		ID:             draftID,
		OrganizationID: orgID,
		MachineID:      machineID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PublishResult{}, ErrNotFound
		}
		return PublishResult{}, err
	}
	if strings.TrimSpace(dr.Status) != draftStatusValidated {
		return PublishResult{}, fmt.Errorf("%w: draft must be validated before publish", ErrValidation)
	}
	save, err := snapshotBytesToSaveInput(dr.Snapshot, true)
	if err != nil {
		return PublishResult{}, err
	}
	if err := validatePublishSnapshot(ctx, q, orgID, machineID, save); err != nil {
		return PublishResult{}, err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return PublishResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := db.New(tx)
	seq, err := qtx.PlanogramNextMachineVersionNo(ctx, machineID)
	if err != nil {
		return PublishResult{}, err
	}
	nextNo := seq + 1

	var pubBy pgtype.UUID
	if actorAccountID != nil && *actorAccountID != uuid.Nil {
		pubBy = pgtype.UUID{Bytes: *actorAccountID, Valid: true}
	}

	vRow, err := qtx.PlanogramInsertVersion(ctx, db.PlanogramInsertVersionParams{
		OrganizationID: orgID,
		MachineID:      machineID,
		VersionNo:      nextNo,
		Snapshot:       dr.Snapshot,
		SourceDraftID:  pgtype.UUID{Bytes: draftID, Valid: true},
		PublishedBy:    pubBy,
	})
	if err != nil {
		return PublishResult{}, err
	}

	if err := insertVersionSlots(ctx, qtx, vRow.ID, dr.Snapshot); err != nil {
		return PublishResult{}, err
	}

	if err := qtx.PlanogramSetMachinePublishedVersion(ctx, db.PlanogramSetMachinePublishedVersionParams{
		ID:                          machineID,
		PublishedPlanogramVersionID: pgtype.UUID{Bytes: vRow.ID, Valid: true},
		OrganizationID:              orgID,
	}); err != nil {
		return PublishResult{}, err
	}

	if err := s.setup.ApplyPublishedSlotConfigsInTx(ctx, tx, machineID, save); err != nil {
		return PublishResult{}, err
	}

	pgStr := save.PlanogramID.String()
	mc, cfgRev, err := postgres.InsertMachineConfigSnapshotTx(ctx, tx, orgID, machineID, pgtype.UUID{}, pgStr, save.PlanogramRevision, &vRow.ID)
	if err != nil {
		return PublishResult{}, err
	}
	_ = mc

	if err := tx.Commit(ctx); err != nil {
		return PublishResult{}, err
	}

	res := PublishResult{
		DraftID:                     draftID,
		PublishedPlanogramVersionID: vRow.ID,
		VersionNo:                   vRow.VersionNo,
		DesiredConfigVersion:        cfgRev,
		PlanogramID:                 pgStr,
		PlanogramRevision:           save.PlanogramRevision,
	}

	s.auditRecord(ctx, orgID, auditActionPublish, machineID, map[string]any{
		"draftId":                     draftID.String(),
		"publishedPlanogramVersionId": vRow.ID.String(),
		"versionNo":                   vRow.VersionNo,
		"desiredConfigVersion":        cfgRev,
	})

	if err := s.dispatchPlanogramCommand(ctx, machineID, idempotencyKey, nil, res); err != nil {
		return res, err
	}

	return res, nil
}

func insertVersionSlots(ctx context.Context, q *db.Queries, versionID uuid.UUID, snapshot []byte) error {
	save, err := snapshotBytesToSaveInput(snapshot, true)
	if err != nil {
		return err
	}
	for _, it := range save.Items {
		var leg pgtype.Int4
		if it.LegacySlotIndex != nil {
			leg = pgtype.Int4{Int32: *it.LegacySlotIndex, Valid: true}
		}
		var pid pgtype.UUID
		if it.ProductID != nil {
			pid = pgtype.UUID{Bytes: *it.ProductID, Valid: true}
		}
		if err := q.PlanogramInsertVersionSlot(ctx, db.PlanogramInsertVersionSlotParams{
			VersionID:       versionID,
			CabinetCode:     strings.TrimSpace(it.CabinetCode),
			LayoutKey:       strings.TrimSpace(it.LayoutKey),
			LayoutRevision:  it.LayoutRevision,
			SlotCode:        strings.TrimSpace(it.SlotCode),
			LegacySlotIndex: leg,
			ProductID:       pid,
			MaxQuantity:     it.MaxQuantity,
			PriceMinor:      it.PriceMinor,
		}); err != nil {
			return err
		}
	}
	return nil
}

// ListVersions returns immutable versions newest first.
func (s *Service) ListVersions(ctx context.Context, orgID, machineID uuid.UUID) ([]VersionListItem, error) {
	rows, err := db.New(s.pool).PlanogramListVersionsForMachine(ctx, db.PlanogramListVersionsForMachineParams{
		OrganizationID: orgID,
		MachineID:      machineID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]VersionListItem, 0, len(rows))
	for _, r := range rows {
		var sd *uuid.UUID
		if r.SourceDraftID.Valid {
			u := uuid.UUID(r.SourceDraftID.Bytes)
			sd = &u
		}
		out = append(out, VersionListItem{
			ID:            r.ID,
			VersionNo:     r.VersionNo,
			PublishedAt:   r.PublishedAt,
			SourceDraftID: sd,
			Snapshot:      r.Snapshot,
		})
	}
	return out, nil
}

// Rollback repoints the published pointer to a prior immutable version and reapplies runtime configs.
func (s *Service) Rollback(ctx context.Context, orgID, machineID, versionID uuid.UUID, idempotencyKey string) (PublishResult, error) {
	q := db.New(s.pool)
	vrow, err := q.PlanogramGetVersionByIDForMachine(ctx, db.PlanogramGetVersionByIDForMachineParams{
		ID:             versionID,
		OrganizationID: orgID,
		MachineID:      machineID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PublishResult{}, ErrNotFound
		}
		return PublishResult{}, err
	}

	save, err := snapshotBytesToSaveInput(vrow.Snapshot, true)
	if err != nil {
		return PublishResult{}, err
	}
	if err := validatePublishSnapshot(ctx, q, orgID, machineID, save); err != nil {
		return PublishResult{}, err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return PublishResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := db.New(tx)
	if err := qtx.PlanogramSetMachinePublishedVersion(ctx, db.PlanogramSetMachinePublishedVersionParams{
		ID:                          machineID,
		PublishedPlanogramVersionID: pgtype.UUID{Bytes: vrow.ID, Valid: true},
		OrganizationID:              orgID,
	}); err != nil {
		return PublishResult{}, err
	}
	if err := s.setup.ApplyPublishedSlotConfigsInTx(ctx, tx, machineID, save); err != nil {
		return PublishResult{}, err
	}
	mc, cfgRev, err := postgres.InsertMachineConfigSnapshotTx(ctx, tx, orgID, machineID, pgtype.UUID{}, save.PlanogramID.String(), save.PlanogramRevision, &vrow.ID)
	if err != nil {
		return PublishResult{}, err
	}
	_ = mc
	if err := tx.Commit(ctx); err != nil {
		return PublishResult{}, err
	}

	res := PublishResult{
		PublishedPlanogramVersionID: vrow.ID,
		VersionNo:                   vrow.VersionNo,
		DesiredConfigVersion:        cfgRev,
		PlanogramID:                 save.PlanogramID.String(),
		PlanogramRevision:           save.PlanogramRevision,
	}

	s.auditRecord(ctx, orgID, auditActionRollback, machineID, map[string]any{
		"publishedPlanogramVersionId": vrow.ID.String(),
		"versionNo":                   vrow.VersionNo,
		"desiredConfigVersion":        cfgRev,
	})

	if err := s.dispatchPlanogramCommand(ctx, machineID, idempotencyKey, nil, res); err != nil {
		return res, err
	}
	return res, nil
}

func (s *Service) dispatchPlanogramCommand(ctx context.Context, machineID uuid.UUID, idempotencyKey string, operatorSessionID *uuid.UUID, res PublishResult) error {
	if s.remote == nil {
		return nil
	}
	idem := strings.TrimSpace(idempotencyKey)
	if idem == "" {
		idem = "planogram-" + uuid.NewString()
	}
	payload := map[string]any{
		"planogramId":                 res.PlanogramID,
		"planogramRevision":           res.PlanogramRevision,
		"desiredConfigVersion":        res.DesiredConfigVersion,
		"publishedPlanogramVersionId": res.PublishedPlanogramVersionID.String(),
		"publishedPlanogramVersionNo": res.VersionNo,
	}
	payloadBytes, _ := json.Marshal(payload)
	desired, _ := json.Marshal(map[string]any{
		"desiredConfigVersion":        res.DesiredConfigVersion,
		"planogramId":                 res.PlanogramID,
		"planogramRevision":           res.PlanogramRevision,
		"publishedPlanogramVersionId": res.PublishedPlanogramVersionID.String(),
	})
	var opSess *uuid.UUID
	if operatorSessionID != nil && *operatorSessionID != uuid.Nil {
		opSess = operatorSessionID
	}
	_, err := s.remote.DispatchRemoteMQTTCommand(ctx, appdevice.RemoteCommandDispatchInput{
		Append: domaindevice.AppendCommandInput{
			MachineID:         machineID,
			CommandType:       s.commandType,
			Payload:           payloadBytes,
			IdempotencyKey:    idem,
			DesiredState:      desired,
			OperatorSessionID: opSess,
		},
	})
	return err
}
