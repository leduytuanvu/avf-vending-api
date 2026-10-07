package layoutassignment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/pgjson"
	"github.com/avf/avf-vending-api/internal/platform/pgxutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ReportLocalLayout upserts the LOCAL mirror (machine_local_layout_mirror) and reported layout state.
// Named layout slots used by GetMachineLayoutDetail are updated via materializeDeviceSlotsToNamedLayout.
// When snapshot ingest is enabled, also appends immutable history via ReportLayoutSnapshot.
func (s *Service) ReportLocalLayout(ctx context.Context, auth MachineAuthContext, in ReportLocalLayoutInput) (ReportLocalLayoutResult, error) {
	if SnapshotIngestEnabled(ctx, s.FeatureFlags, in.MachineID) {
		snapIn := ReportLayoutSnapshotFromLocalLayout(in)
		if snapIn.CapturedAt.IsZero() {
			snapIn.CapturedAt = time.Now().UTC()
		}
		snapOut, snapErr := s.ReportLayoutSnapshot(ctx, auth, snapIn)
		if snapErr != nil {
			if errors.Is(snapErr, ErrLayoutRevisionConflict) {
				return ReportLocalLayoutResult{}, snapErr
			}
			// Fall through to legacy mirror path when layout row missing during rollout.
			if !errors.Is(snapErr, ErrLayoutNotFound) {
				return ReportLocalLayoutResult{}, snapErr
			}
		} else {
			return ReportLocalLayoutResult{
				Accepted:          snapOut.Accepted,
				StoredRevision:    snapOut.StoredRevision,
				StoredGeneration:  snapOut.StoredGeneration,
				StoredFingerprint: snapOut.StoredFingerprint,
			}, nil
		}
	}
	return s.reportLocalLayoutLegacy(ctx, auth, in)
}

func (s *Service) reportLocalLayoutLegacy(ctx context.Context, auth MachineAuthContext, in ReportLocalLayoutInput) (ReportLocalLayoutResult, error) {
	if s.Pool == nil {
		return ReportLocalLayoutResult{}, fmt.Errorf("database pool is not configured")
	}
	if auth.MachineID == uuid.Nil {
		return ReportLocalLayoutResult{}, fmt.Errorf("machine auth context is required")
	}
	if in.MachineID == uuid.Nil || in.MachineID != auth.MachineID {
		return ReportLocalLayoutResult{}, fmt.Errorf("machine auth context does not match report")
	}
	if in.LocalLayoutID == uuid.Nil {
		return ReportLocalLayoutResult{}, fmt.Errorf("localLayoutId is required")
	}
	if in.Revision < 1 {
		return ReportLocalLayoutResult{}, fmt.Errorf("revision must be >= 1")
	}
	if strings.TrimSpace(in.Fingerprint) == "" {
		return ReportLocalLayoutResult{}, fmt.Errorf("fingerprint is required")
	}
	if strings.TrimSpace(in.DeviceInstanceID) == "" {
		return ReportLocalLayoutResult{}, fmt.Errorf("deviceInstanceId is required")
	}
	if err := ValidateGridDimensions(in.Rows, in.Columns); err != nil {
		return ReportLocalLayoutResult{}, err
	}
	if len(in.SlotsJSON) == 0 || !json.Valid(in.SlotsJSON) {
		return ReportLocalLayoutResult{}, fmt.Errorf("slots payload is required")
	}
	if err := validateReportedSlotsUnique(in.SlotsJSON); err != nil {
		return ReportLocalLayoutResult{}, err
	}

	readQ := pgxutil.NewQueries(s.Pool)
	mirror, mirrorErr := readQ.GetMachineLocalLayoutMirror(ctx, in.MachineID)
	storedRev := int32(0)
	storedFP := ""
	if mirrorErr == nil {
		storedRev = mirror.Revision
		storedFP = strings.TrimSpace(mirror.Fingerprint)
	} else if mirrorErr != pgx.ErrNoRows {
		return ReportLocalLayoutResult{}, mirrorErr
	}

	inFP := strings.TrimSpace(in.Fingerprint)
	if in.Revision < storedRev {
		return ReportLocalLayoutResult{Accepted: true, StoredRevision: storedRev, StoredGeneration: in.LocalGeneration, StoredFingerprint: storedFP}, nil
	}
	if in.Revision == storedRev {
		if inFP == storedFP {
			if err := s.repairNamedLayoutMaterializationIfNeeded(ctx, in, inFP); err != nil {
				return ReportLocalLayoutResult{}, err
			}
			return ReportLocalLayoutResult{Accepted: true, StoredRevision: storedRev, StoredGeneration: in.LocalGeneration, StoredFingerprint: storedFP}, nil
		}
		return ReportLocalLayoutResult{}, ErrLayoutRevisionConflict
	}

	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ReportLocalLayoutResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := pgxutil.NewQueries(tx)
	now := time.Now().UTC()
	if err := q.UpsertMachineLocalLayoutMirror(ctx, db.UpsertMachineLocalLayoutMirrorParams{
		MachineID:        in.MachineID,
		LocalLayoutID:    in.LocalLayoutID,
		Revision:         in.Revision,
		GridRows:         in.Rows,
		GridCols:         in.Columns,
		Slots:            pgjson.RequiredString(in.SlotsJSON),
		Fingerprint:      inFP,
		ReportedAt:       now,
		DeviceInstanceID: strings.TrimSpace(in.DeviceInstanceID),
	}); err != nil {
		return ReportLocalLayoutResult{}, err
	}

	if err := s.materializeDeviceSlotsToNamedLayout(
		ctx,
		tx,
		in.MachineID,
		in.LocalLayoutID,
		in.SlotsJSON,
		inFP,
		"report_local_layout",
	); err != nil {
		return ReportLocalLayoutResult{}, err
	}

	_, err = q.UpdateMachineLayoutStateReported(ctx, db.UpdateMachineLayoutStateReportedParams{
		ReportedSource:           pgtype.Text{String: SourceLocal, Valid: true},
		ReportedAssignmentID:     pgtype.UUID{},
		ReportedLayoutVersionID:  pgtype.UUID{},
		ReportedRevision:         pgtype.Int4{Int32: in.Revision, Valid: true},
		ReportedFingerprint:      pgtype.Text{String: inFP, Valid: true},
		ReportedAt:               pgtype.Timestamptz{Time: now, Valid: true},
		ReportedDeviceInstanceID: pgtype.Text{String: strings.TrimSpace(in.DeviceInstanceID), Valid: true},
		ApplyFailureReason:       pgtype.Text{},
		MachineID:                in.MachineID,
	})
	if err != nil {
		return ReportLocalLayoutResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return ReportLocalLayoutResult{}, err
	}

	return ReportLocalLayoutResult{Accepted: true, StoredRevision: in.Revision, StoredGeneration: in.LocalGeneration, StoredFingerprint: inFP}, nil
}

func (s *Service) repairNamedLayoutMaterializationIfNeeded(ctx context.Context, in ReportLocalLayoutInput, fingerprint string) error {
	mirrorAssignments := countMirrorProductAssignments(in.SlotsJSON)
	if mirrorAssignments == 0 {
		return nil
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	layoutID, err := s.resolveNamedLayoutIDForMaterialize(ctx, tx, in.MachineID, in.LocalLayoutID)
	if err != nil {
		return err
	}
	if layoutID == uuid.Nil {
		return nil
	}
	q := pgxutil.NewQueries(tx)
	slotRows, err := q.ListMachineLayoutSlots(ctx, layoutID)
	if err != nil {
		return err
	}
	if !needsNamedLayoutMaterialization(mirrorAssignments, countAssignedNamedLayoutSlots(slotRows)) {
		return nil
	}
	if err := s.materializeDeviceSlotsToNamedLayout(
		ctx,
		tx,
		in.MachineID,
		in.LocalLayoutID,
		in.SlotsJSON,
		fingerprint,
		"report_local_layout_repair",
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func validateReportedSlotsUnique(slotsJSON []byte) error {
	var slots []struct {
		SlotCode string `json:"slotCode"`
	}
	if err := json.Unmarshal(slotsJSON, &slots); err != nil {
		return fmt.Errorf("invalid slots payload")
	}
	seen := make(map[string]struct{}, len(slots))
	for _, sl := range slots {
		code := strings.TrimSpace(sl.SlotCode)
		if code == "" {
			return fmt.Errorf("slotCode is required for each slot")
		}
		if _, dup := seen[code]; dup {
			return fmt.Errorf("duplicate slotCode %q", code)
		}
		seen[code] = struct{}{}
	}
	return nil
}
