package layoutassignment

import (
	"context"
	"fmt"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/setupapp"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/pgxutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// SetDesiredSource updates the desired active layout source for a machine.
func (s *Service) SetDesiredSource(ctx context.Context, in SetDesiredSourceInput) (SetDesiredSourceResult, error) {
	if s.Pool == nil {
		return SetDesiredSourceResult{}, fmt.Errorf("database pool is not configured")
	}
	if in.MachineID == uuid.Nil {
		return SetDesiredSourceResult{}, fmt.Errorf("machineId is required")
	}
	src := strings.ToUpper(strings.TrimSpace(in.Source))
	if src != SourceServer && src != SourceLocal {
		return SetDesiredSourceResult{}, fmt.Errorf("source must be SERVER or LOCAL")
	}

	q := pgxutil.NewQueries(s.Pool)
	if _, err := q.GetMachineByID(ctx, in.MachineID); err != nil {
		if err == pgx.ErrNoRows {
			return SetDesiredSourceResult{}, setupapp.ErrNotFound
		}
		return SetDesiredSourceResult{}, err
	}

	state, stateErr := q.GetMachineLayoutState(ctx, in.MachineID)
	if stateErr != nil && stateErr != pgx.ErrNoRows {
		return SetDesiredSourceResult{}, stateErr
	}
	if in.ExpectedCurrentRevision != nil {
		if stateErr == pgx.ErrNoRows {
			if *in.ExpectedCurrentRevision != 0 {
				return SetDesiredSourceResult{}, ErrRevisionConflict
			}
		} else if !state.DesiredRevision.Valid || state.DesiredRevision.Int32 != *in.ExpectedCurrentRevision {
			return SetDesiredSourceResult{}, ErrRevisionConflict
		}
	}

	var (
		desiredAssignmentID    pgtype.UUID
		desiredLayoutVersionID pgtype.UUID
		desiredRevision        pgtype.Int4
		desiredFingerprint     pgtype.Text
	)

	switch src {
	case SourceServer:
		assign, err := q.GetCurrentMachineLayoutAssignment(ctx, db.GetCurrentMachineLayoutAssignmentParams{
			MachineID: in.MachineID,
			Source:    SourceServer,
		})
		if err != nil {
			if err == pgx.ErrNoRows {
				return SetDesiredSourceResult{}, ErrLayoutAssignmentNotFound
			}
			return SetDesiredSourceResult{}, err
		}
		desiredAssignmentID = pgtype.UUID{Bytes: assign.ID, Valid: true}
		if assign.LayoutVersionID.Valid {
			desiredLayoutVersionID = assign.LayoutVersionID
		}
		desiredRevision = pgtype.Int4{Int32: assign.Revision, Valid: true}
		desiredFingerprint = pgtype.Text{String: assign.Fingerprint, Valid: true}
	case SourceLocal:
		assign, assignErr := q.GetCurrentMachineLayoutAssignment(ctx, db.GetCurrentMachineLayoutAssignmentParams{
			MachineID: in.MachineID,
			Source:    SourceLocal,
		})
		if assignErr == nil {
			desiredAssignmentID = pgtype.UUID{Bytes: assign.ID, Valid: true}
			if assign.LayoutVersionID.Valid {
				desiredLayoutVersionID = assign.LayoutVersionID
			}
			desiredRevision = pgtype.Int4{Int32: assign.Revision, Valid: true}
			desiredFingerprint = pgtype.Text{String: assign.Fingerprint, Valid: true}
		} else if assignErr == pgx.ErrNoRows {
			mirror, mirrorErr := q.GetMachineLocalLayoutMirror(ctx, in.MachineID)
			if mirrorErr != nil {
				if mirrorErr == pgx.ErrNoRows {
					return SetDesiredSourceResult{}, ErrLayoutAssignmentNotFound
				}
				return SetDesiredSourceResult{}, mirrorErr
			}
			desiredRevision = pgtype.Int4{Int32: mirror.Revision, Valid: true}
			desiredFingerprint = pgtype.Text{String: mirror.Fingerprint, Valid: true}
		} else {
			return SetDesiredSourceResult{}, assignErr
		}
	}

	if err := q.UpsertMachineLayoutStateDesired(ctx, db.UpsertMachineLayoutStateDesiredParams{
		MachineID:              in.MachineID,
		DesiredSource:          pgtype.Text{String: src, Valid: true},
		DesiredAssignmentID:    desiredAssignmentID,
		DesiredLayoutVersionID: desiredLayoutVersionID,
		DesiredRevision:        desiredRevision,
		DesiredFingerprint:     desiredFingerprint,
	}); err != nil {
		return SetDesiredSourceResult{}, err
	}

	stateView, _ := s.GetLayoutState(ctx, in.MachineID)
	syncStatus := SyncStatusPending
	if stateView != nil {
		syncStatus = stateView.SyncStatus
	}

	return SetDesiredSourceResult{
		DesiredSource:      src,
		DesiredRevision:    desiredRevision.Int32,
		DesiredFingerprint: desiredFingerprint.String,
		SyncStatus:         syncStatus,
	}, nil
}
