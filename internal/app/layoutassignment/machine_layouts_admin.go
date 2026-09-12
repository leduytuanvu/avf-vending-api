package layoutassignment

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/pgxutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// CreateMachineLayoutInput creates a new named layout for a machine.
type CreateMachineLayoutInput struct {
	MachineID uuid.UUID
	Name      string
	Status    string
	GridRows  int32
	GridCols  int32
}

// UpdateMachineLayoutInput updates mutable layout metadata.
type UpdateMachineLayoutInput struct {
	MachineID uuid.UUID
	LayoutID  uuid.UUID
	Name      *string
	Status    *string
}

// SetDesiredActiveLayoutInput sets desired_active_layout_id for offline activation.
type SetDesiredActiveLayoutInput struct {
	MachineID uuid.UUID
	LayoutID  uuid.UUID
}

// SnapshotHistoryPage is paginated immutable history.
type SnapshotHistoryPage struct {
	Items []SnapshotHistoryItem
	Total int64
}

type SnapshotHistoryItem struct {
	SnapshotID      uuid.UUID `json:"snapshotId"`
	LayoutID        uuid.UUID `json:"layoutId"`
	CaptureSequence int64     `json:"captureSequence"`
	CapturedAt      time.Time `json:"capturedAt"`
	ReceivedAt      time.Time `json:"receivedAt"`
	Fingerprint     string    `json:"fingerprint"`
	SnapshotReason  string    `json:"snapshotReason"`
	PayloadVersion  int32     `json:"payloadVersion"`
}

// ListMachineLayouts returns named layouts for admin display.
func (s *Service) ListMachineLayouts(ctx context.Context, machineID uuid.UUID) ([]MachineLayoutSummaryView, error) {
	lib, err := s.GetMachineLayoutLibrary(ctx, machineID)
	if err != nil {
		return nil, err
	}
	return lib.Layouts, nil
}

// CreateMachineLayout inserts a new named layout row.
func (s *Service) CreateMachineLayout(ctx context.Context, in CreateMachineLayoutInput) (MachineLayoutSummaryView, error) {
	if s.Pool == nil {
		return MachineLayoutSummaryView{}, fmt.Errorf("database pool is not configured")
	}
	if err := ValidateLayoutName(in.Name); err != nil {
		return MachineLayoutSummaryView{}, err
	}
	status := strings.TrimSpace(strings.ToUpper(in.Status))
	if status == "" {
		status = "DRAFT"
	}
	rows, cols := in.GridRows, in.GridCols
	if rows < 1 || cols < 1 {
		rows, cols = DefaultCreationDimensions()
	}
	if err := ValidateGridDimensions(rows, cols); err != nil {
		return MachineLayoutSummaryView{}, err
	}
	q := pgxutil.NewQueries(s.Pool)
	row, err := q.InsertMachineLayout(ctx, db.InsertMachineLayoutParams{
		MachineID:      in.MachineID,
		Name:           strings.TrimSpace(in.Name),
		Status:         status,
		GridRows:       rows,
		GridCols:       cols,
		LayoutRevision: 1,
		Fingerprint:    fmt.Sprintf("admin:create:%dx%d:v1", rows, cols),
	})
	if err != nil {
		return MachineLayoutSummaryView{}, err
	}
	return MachineLayoutSummaryView{
		LayoutID:    row.ID,
		Name:        row.Name,
		Status:      row.Status,
		GridRows:    row.GridRows,
		GridCols:    row.GridCols,
		Revision:    row.LayoutRevision,
		Fingerprint: row.Fingerprint,
	}, nil
}

// UpdateMachineLayoutMetadata updates name/status for a layout.
func (s *Service) UpdateMachineLayoutMetadata(ctx context.Context, in UpdateMachineLayoutInput) (MachineLayoutSummaryView, error) {
	if s.Pool == nil {
		return MachineLayoutSummaryView{}, fmt.Errorf("database pool is not configured")
	}
	q := pgxutil.NewQueries(s.Pool)
	var name pgtype.Text
	if in.Name != nil {
		if err := ValidateLayoutName(*in.Name); err != nil {
			return MachineLayoutSummaryView{}, err
		}
		name = pgtype.Text{String: strings.TrimSpace(*in.Name), Valid: true}
	}
	var status pgtype.Text
	if in.Status != nil {
		status = pgtype.Text{String: strings.TrimSpace(strings.ToUpper(*in.Status)), Valid: true}
	}
	row, err := q.UpdateMachineLayoutMetadata(ctx, db.UpdateMachineLayoutMetadataParams{
		ID:        in.LayoutID,
		MachineID: in.MachineID,
		Name:      name,
		Status:    status,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return MachineLayoutSummaryView{}, ErrLayoutNotFound
		}
		return MachineLayoutSummaryView{}, err
	}
	return MachineLayoutSummaryView{
		LayoutID:    row.ID,
		Name:        row.Name,
		Status:      row.Status,
		GridRows:    row.GridRows,
		GridCols:    row.GridCols,
		Revision:    row.LayoutRevision,
		Fingerprint: row.Fingerprint,
	}, nil
}

// ArchiveMachineLayout soft-deletes a layout when not last/active.
func (s *Service) ArchiveMachineLayout(ctx context.Context, machineID, layoutID uuid.UUID) error {
	if s.Pool == nil {
		return fmt.Errorf("database pool is not configured")
	}
	q := pgxutil.NewQueries(s.Pool)
	count, err := q.CountMachineLayoutsForMachine(ctx, machineID)
	if err != nil {
		return err
	}
	if count <= 1 {
		return fmt.Errorf("cannot archive the only layout for a machine")
	}
	machine, err := q.GetMachineByID(ctx, machineID)
	if err != nil {
		return err
	}
	if machine.ActiveLayoutID.Valid && uuid.UUID(machine.ActiveLayoutID.Bytes) == layoutID {
		return fmt.Errorf("cannot archive active layout")
	}
	n, err := q.ArchiveMachineLayout(ctx, db.ArchiveMachineLayoutParams{
		ID:        layoutID,
		MachineID: machineID,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrLayoutNotFound
	}
	return nil
}

// SetDesiredActiveLayout sets machines.desired_active_layout_id.
func (s *Service) SetDesiredActiveLayout(ctx context.Context, in SetDesiredActiveLayoutInput) error {
	if s.Pool == nil {
		return fmt.Errorf("database pool is not configured")
	}
	q := pgxutil.NewQueries(s.Pool)
	if _, err := q.GetMachineLayoutByID(ctx, db.GetMachineLayoutByIDParams{
		ID:        in.LayoutID,
		MachineID: in.MachineID,
	}); err != nil {
		if err == pgx.ErrNoRows {
			return ErrLayoutNotFound
		}
		return err
	}
	return q.SetMachineActiveLayoutPointers(ctx, db.SetMachineActiveLayoutPointersParams{
		ID: in.MachineID,
		DesiredActiveLayoutID: pgtype.UUID{Bytes: in.LayoutID, Valid: true},
	})
}

// ListLayoutSnapshotHistory returns paginated immutable history for a machine/layout.
func (s *Service) ListLayoutSnapshotHistory(ctx context.Context, machineID uuid.UUID, layoutID *uuid.UUID, limit, offset int32) (SnapshotHistoryPage, error) {
	if s.Pool == nil {
		return SnapshotHistoryPage{}, fmt.Errorf("database pool is not configured")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	q := pgxutil.NewQueries(s.Pool)
	var layout pgtype.UUID
	if layoutID != nil && *layoutID != uuid.Nil {
		layout = pgtype.UUID{Bytes: *layoutID, Valid: true}
	}
	total, err := q.CountMachineLayoutSnapshotHistory(ctx, db.CountMachineLayoutSnapshotHistoryParams{
		MachineID: machineID,
		LayoutID:  layout,
	})
	if err != nil {
		return SnapshotHistoryPage{}, err
	}
	rows, err := q.ListMachineLayoutSnapshotHistoryPage(ctx, db.ListMachineLayoutSnapshotHistoryPageParams{
		MachineID:  machineID,
		LayoutID:   layout,
		PageLimit:  limit,
		PageOffset: offset,
	})
	if err != nil {
		return SnapshotHistoryPage{}, err
	}
	items := make([]SnapshotHistoryItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, SnapshotHistoryItem{
			SnapshotID:      row.SnapshotID,
			LayoutID:        row.LayoutID,
			CaptureSequence: row.CaptureSequence,
			CapturedAt:      row.CapturedAt,
			ReceivedAt:      row.ReceivedAt,
			Fingerprint:     row.Fingerprint,
			SnapshotReason:  row.SnapshotReason,
			PayloadVersion:  row.PayloadVersion,
		})
	}
	return SnapshotHistoryPage{Items: items, Total: total}, nil
}

// GetLayoutSnapshotDetail returns one history row by snapshot id.
func (s *Service) GetLayoutSnapshotDetail(ctx context.Context, snapshotID uuid.UUID) (db.MachineLayoutSnapshotHistory, error) {
	if s.Pool == nil {
		return db.MachineLayoutSnapshotHistory{}, fmt.Errorf("database pool is not configured")
	}
	row, err := pgxutil.NewQueries(s.Pool).GetMachineLayoutSnapshotHistoryBySnapshotID(ctx, snapshotID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return db.MachineLayoutSnapshotHistory{}, ErrLayoutNotFound
		}
		return db.MachineLayoutSnapshotHistory{}, err
	}
	return row, nil
}
