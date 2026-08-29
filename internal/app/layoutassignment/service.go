package layoutassignment

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/planogram"
	"github.com/avf/avf-vending-api/internal/app/setupapp"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/platform/pgxutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var layoutKeyPattern = regexp.MustCompile(`^grid-(\d+)x(\d+)$`)

// Service implements atomic SERVER layout assignment and layout state reads.
type Service struct {
	Pool   *pgxpool.Pool
	Setup  *postgres.SetupRepository
}

// AssignServerLayout atomically assigns a published planogram version as the current SERVER layout.
func (s *Service) AssignServerLayout(ctx context.Context, in AssignServerLayoutInput) (AssignServerLayoutResult, error) {
	if s.Pool == nil {
		return AssignServerLayoutResult{}, fmt.Errorf("database pool is not configured")
	}
	if in.MachineID == uuid.Nil || in.LayoutVersionID == uuid.Nil {
		return AssignServerLayoutResult{}, fmt.Errorf("machineId and layoutVersionId are required")
	}

	if replay, ok, idemErr := s.beginAssignServerLayoutIdempotency(ctx, in); idemErr != nil {
		return AssignServerLayoutResult{}, idemErr
	} else if ok {
		return replay, nil
	}

	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AssignServerLayoutResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := pgxutil.NewQueries(tx)
	machine, err := q.GetMachineByIDForUpdate(ctx, in.MachineID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return AssignServerLayoutResult{}, setupapp.ErrNotFound
		}
		return AssignServerLayoutResult{}, err
	}

	vRow, err := q.GetMachinePlanogramVersionByID(ctx, in.LayoutVersionID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return AssignServerLayoutResult{}, ErrLayoutVersionNotFound
		}
		return AssignServerLayoutResult{}, err
	}
	if vRow.MachineID != in.MachineID {
		return AssignServerLayoutResult{}, ErrMachineMismatch
	}

	rows, cols, dimErr := resolveVersionDimensions(vRow)
	if dimErr != nil {
		return AssignServerLayoutResult{}, dimErr
	}
	if err := ValidateGridDimensions(rows, cols); err != nil {
		return AssignServerLayoutResult{}, err
	}
	machineType := ""
	if machine.MachineType.Valid {
		machineType = machine.MachineType.String
	}
	if err := ValidateMachineTypeLaneBound(machineType, rows, cols); err != nil {
		return AssignServerLayoutResult{}, err
	}

	current, curErr := q.GetCurrentMachineLayoutAssignment(ctx, db.GetCurrentMachineLayoutAssignmentParams{
		MachineID: in.MachineID,
		Source:    SourceServer,
	})
	if curErr != nil && curErr != pgx.ErrNoRows {
		return AssignServerLayoutResult{}, curErr
	}
	if in.ExpectedCurrentRevision != nil && curErr == nil {
		if current.Revision != *in.ExpectedCurrentRevision {
			return AssignServerLayoutResult{}, ErrRevisionConflict
		}
	}

	saveIn, err := planogram.SnapshotBytesToSaveInput(vRow.Snapshot, true)
	if err != nil {
		return AssignServerLayoutResult{}, err
	}
	if s.Setup == nil {
		return AssignServerLayoutResult{}, fmt.Errorf("setup repository is not configured")
	}
	if err := s.Setup.SaveDraftOrCurrentSlotConfigsInTx(ctx, tx, in.MachineID, saveIn); err != nil {
		return AssignServerLayoutResult{}, err
	}

	if err := q.PlanogramSetMachinePublishedVersion(ctx, db.PlanogramSetMachinePublishedVersionParams{
		PublishedPlanogramVersionID: pgtype.UUID{Bytes: vRow.ID, Valid: true},
		ID:                          in.MachineID,
	}); err != nil {
		return AssignServerLayoutResult{}, err
	}

	slotParts := fingerprintPartsFromSaveInput(saveIn)

	nextRev, err := q.NextMachineLayoutAssignmentRevision(ctx, db.NextMachineLayoutAssignmentRevisionParams{
		MachineID: in.MachineID,
		Source:    SourceServer,
	})
	if err != nil {
		return AssignServerLayoutResult{}, err
	}
	fp := AssignmentFingerprint(SourceServer, vRow.ID.String(), nextRev, rows, cols, slotParts)

	if err := q.CloseCurrentMachineLayoutAssignment(ctx, db.CloseCurrentMachineLayoutAssignmentParams{
		MachineID: in.MachineID,
		Source:    SourceServer,
	}); err != nil {
		return AssignServerLayoutResult{}, err
	}

	var createdBy pgtype.UUID
	if in.ActorAccountID != nil {
		createdBy = pgtype.UUID{Bytes: *in.ActorAccountID, Valid: true}
	}
	var orgVer pgtype.UUID
	if in.OrgLayoutVersionID != nil {
		orgVer = pgtype.UUID{Bytes: *in.OrgLayoutVersionID, Valid: true}
	}
	assignRow, err := q.InsertMachineLayoutAssignment(ctx, db.InsertMachineLayoutAssignmentParams{
		MachineID:           in.MachineID,
		Source:              SourceServer,
		LayoutID:            pgtype.UUID{},
		LayoutVersionID:     pgtype.UUID{Bytes: vRow.ID, Valid: true},
		OrgLayoutVersionID:  orgVer,
		Revision:            nextRev,
		GridRows:            rows,
		GridCols:            cols,
		Fingerprint:         fp,
		CreatedBy:           createdBy,
	})
	if err != nil {
		return AssignServerLayoutResult{}, err
	}

	desiredSource := SourceServer
	if err := q.UpsertMachineLayoutStateDesired(ctx, db.UpsertMachineLayoutStateDesiredParams{
		MachineID:              in.MachineID,
		DesiredSource:          pgtype.Text{String: desiredSource, Valid: true},
		DesiredAssignmentID:    pgtype.UUID{Bytes: assignRow.ID, Valid: true},
		DesiredLayoutVersionID: pgtype.UUID{Bytes: vRow.ID, Valid: true},
		DesiredRevision:        pgtype.Int4{Int32: nextRev, Valid: true},
		DesiredFingerprint:     pgtype.Text{String: fp, Valid: true},
	}); err != nil {
		return AssignServerLayoutResult{}, err
	}

	var opSess pgtype.UUID
	if in.OperatorSessionID != nil {
		opSess = pgtype.UUID{Bytes: *in.OperatorSessionID, Valid: true}
	}
	_, _, snapErr := postgres.InsertMachineConfigSnapshotTx(ctx, tx, uuid.Nil, in.MachineID, opSess,
		saveIn.PlanogramID.String(), saveIn.PlanogramRevision, &vRow.ID)
	if snapErr != nil {
		return AssignServerLayoutResult{}, snapErr
	}

	if err := tx.Commit(ctx); err != nil {
		return AssignServerLayoutResult{}, err
	}

	state, _ := s.GetLayoutState(ctx, in.MachineID)
	syncStatus := SyncStatusPending
	if state != nil {
		syncStatus = state.SyncStatus
	}

	out := AssignServerLayoutResult{
		AssignmentID:  assignRow.ID,
		Source:        SourceServer,
		Revision:      nextRev,
		Rows:          rows,
		Columns:       cols,
		Fingerprint:   fp,
		DesiredSource: &desiredSource,
		SyncStatus:    syncStatus,
	}
	if err := s.storeAssignServerLayoutIdempotency(ctx, in, out); err != nil {
		return AssignServerLayoutResult{}, err
	}
	return out, nil
}

// GetLayoutState returns desired/reported layout state for admin display.
func (s *Service) GetLayoutState(ctx context.Context, machineID uuid.UUID) (*LayoutStateView, error) {
	if s.Pool == nil {
		return nil, fmt.Errorf("database pool is not configured")
	}
	q := pgxutil.NewQueries(s.Pool)
	view := &LayoutStateView{MachineID: machineID}

	state, err := q.GetMachineLayoutState(ctx, machineID)
	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}
	if err == nil {
		if state.DesiredSource.Valid {
			ds := state.DesiredSource.String
			view.DesiredSource = &ds
		}
		if state.DesiredRevision.Valid {
			r := state.DesiredRevision.Int32
			view.DesiredRevision = &r
		}
		if state.DesiredFingerprint.Valid {
			f := state.DesiredFingerprint.String
			view.DesiredFingerprint = &f
		}
		if state.ReportedSource.Valid {
			rs := state.ReportedSource.String
			view.ReportedSource = &rs
		}
		if state.ReportedRevision.Valid {
			r := state.ReportedRevision.Int32
			view.ReportedRevision = &r
		}
		if state.ReportedFingerprint.Valid {
			f := state.ReportedFingerprint.String
			view.ReportedFingerprint = &f
		}
		if state.ReportedAt.Valid {
			t := state.ReportedAt.Time
			view.ReportedAt = &t
		}
		var applyFail *string
		if state.ApplyFailureReason.Valid {
			a := state.ApplyFailureReason.String
			applyFail = &a
		}
		view.SyncStatus = DeriveSyncStatus(view.DesiredSource, view.ReportedSource, view.DesiredFingerprint, view.ReportedFingerprint, applyFail)
		view.ApplyFailureReason = applyFail
	} else {
		view.SyncStatus = SyncStatusOfflineUnknown
	}

	if srv, serr := q.GetCurrentMachineLayoutAssignment(ctx, db.GetCurrentMachineLayoutAssignmentParams{
		MachineID: machineID,
		Source:    SourceServer,
	}); serr == nil {
		view.ServerAssignment = mapAssignmentRow(srv)
	}
	if loc, lerr := q.GetCurrentMachineLayoutAssignment(ctx, db.GetCurrentMachineLayoutAssignmentParams{
		MachineID: machineID,
		Source:    SourceLocal,
	}); lerr == nil {
		view.LocalAssignment = mapAssignmentRow(loc)
	}
	if mirror, merr := q.GetMachineLocalLayoutMirror(ctx, machineID); merr == nil {
		view.LocalMirror = &LocalMirrorView{
			LocalLayoutID:    mirror.LocalLayoutID,
			Revision:         mirror.Revision,
			Rows:             mirror.GridRows,
			Columns:          mirror.GridCols,
			Fingerprint:      mirror.Fingerprint,
			ReportedAt:       mirror.ReportedAt,
			DeviceInstanceID: mirror.DeviceInstanceID,
		}
	}

	return view, nil
}

func mapAssignmentRow(row db.MachineLayoutAssignment) *AssignmentView {
	v := &AssignmentView{
		AssignmentID: row.ID,
		Source:       row.Source,
		Revision:     row.Revision,
		Rows:         row.GridRows,
		Columns:      row.GridCols,
		Fingerprint:  row.Fingerprint,
		EffectiveFrom: row.EffectiveFrom,
	}
	if row.LayoutVersionID.Valid {
		id := uuid.UUID(row.LayoutVersionID.Bytes)
		v.LayoutVersionID = &id
	}
	return v
}

func resolveVersionDimensions(vRow db.MachinePlanogramVersion) (rows, cols int32, err error) {
	if vRow.GridRows.Valid && vRow.GridCols.Valid && vRow.GridRows.Int32 > 0 && vRow.GridCols.Int32 > 0 {
		return vRow.GridRows.Int32, vRow.GridCols.Int32, nil
	}
	var body struct {
		Items []struct {
			LayoutKey string `json:"layoutKey"`
		} `json:"items"`
	}
	if uerr := json.Unmarshal(vRow.Snapshot, &body); uerr != nil {
		return 0, 0, ErrUnknownDimensions
	}
	for _, it := range body.Items {
		key := strings.TrimSpace(it.LayoutKey)
		if key == "" {
			continue
		}
		m := layoutKeyPattern.FindStringSubmatch(key)
		if len(m) == 3 {
			var c, r int32
			if _, scanErr := fmt.Sscanf(m[1], "%d", &c); scanErr != nil {
				continue
			}
			if _, scanErr := fmt.Sscanf(m[2], "%d", &r); scanErr != nil {
				continue
			}
			return r, c, nil
		}
	}
	return 0, 0, ErrUnknownDimensions
}

func fingerprintPartsFromSaveInput(in setupapp.SlotConfigSaveInput) []string {
	parts := make([]string, 0, len(in.Items))
	for _, it := range in.Items {
		pid := ""
		if it.ProductID != nil {
			pid = it.ProductID.String()
		}
		parts = append(parts, fmt.Sprintf("%s:%s:%d:%s", it.CabinetCode, it.SlotCode, derefInt32(it.LegacySlotIndex), pid))
	}
	return parts
}

func derefInt32(p *int32) int32 {
	if p == nil {
		return 0
	}
	return *p
}
