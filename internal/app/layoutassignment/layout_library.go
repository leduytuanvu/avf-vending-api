package layoutassignment

import (
	"context"
	"fmt"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/pgxutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// MachineLayoutSummaryView is one named layout row for device/admin reads.
type MachineLayoutSummaryView struct {
	LayoutID    uuid.UUID `json:"layoutId"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	GridRows    int32     `json:"gridRows"`
	GridCols    int32     `json:"gridCols"`
	Revision    int32     `json:"revision"`
	Fingerprint string    `json:"fingerprint"`
}

// MachineLayoutLibraryView lists layouts plus active/desired/reported pointers.
type MachineLayoutLibraryView struct {
	MachineID              uuid.UUID                  `json:"machineId"`
	Layouts                []MachineLayoutSummaryView `json:"layouts"`
	ActiveLayoutID         *uuid.UUID                 `json:"activeLayoutId,omitempty"`
	DesiredActiveLayoutID  *uuid.UUID                 `json:"desiredActiveLayoutId,omitempty"`
	ReportedActiveLayoutID *uuid.UUID                 `json:"reportedActiveLayoutId,omitempty"`
}

// GetMachineLayoutLibrary returns named layouts and active-layout pointers for a machine.
func (s *Service) GetMachineLayoutLibrary(ctx context.Context, machineID uuid.UUID) (*MachineLayoutLibraryView, error) {
	if s.Pool == nil {
		return nil, fmt.Errorf("database pool is not configured")
	}
	if machineID == uuid.Nil {
		return nil, fmt.Errorf("machineId is required")
	}
	q := pgxutil.NewQueries(s.Pool)
	machine, err := q.GetMachineByID(ctx, machineID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("machine not found")
		}
		return nil, err
	}
	rows, err := q.ListMachineLayoutsForMachine(ctx, machineID)
	if err != nil {
		return nil, err
	}
	view := &MachineLayoutLibraryView{
		MachineID: machineID,
		Layouts:   make([]MachineLayoutSummaryView, 0, len(rows)),
	}
	for _, row := range rows {
		view.Layouts = append(view.Layouts, MachineLayoutSummaryView{
			LayoutID:    row.ID,
			Name:        row.Name,
			Status:      row.Status,
			GridRows:    row.GridRows,
			GridCols:    row.GridCols,
			Revision:    row.LayoutRevision,
			Fingerprint: row.Fingerprint,
		})
	}
	if machine.ActiveLayoutID.Valid {
		id := uuid.UUID(machine.ActiveLayoutID.Bytes)
		view.ActiveLayoutID = &id
	}
	if machine.DesiredActiveLayoutID.Valid {
		id := uuid.UUID(machine.DesiredActiveLayoutID.Bytes)
		view.DesiredActiveLayoutID = &id
	}
	if machine.ReportedActiveLayoutID.Valid {
		id := uuid.UUID(machine.ReportedActiveLayoutID.Bytes)
		view.ReportedActiveLayoutID = &id
	}
	return view, nil
}

// AckLayoutActivationInput records device confirmation of desired layout activation.
type AckLayoutActivationInput struct {
	MachineID        uuid.UUID
	LayoutID         uuid.UUID
	CaptureSequence  int64
	Fingerprint      string
	DeviceInstanceID string
}

// AckLayoutActivationResult is returned after activation ACK.
type AckLayoutActivationResult struct {
	Accepted               bool
	ReportedActiveLayoutID uuid.UUID
}

// AckLayoutActivation sets reported_active_layout_id when device confirms apply.
func (s *Service) AckLayoutActivation(ctx context.Context, auth MachineAuthContext, in AckLayoutActivationInput) (AckLayoutActivationResult, error) {
	if s.Pool == nil {
		return AckLayoutActivationResult{}, fmt.Errorf("database pool is not configured")
	}
	if auth.MachineID == uuid.Nil || in.MachineID != auth.MachineID {
		return AckLayoutActivationResult{}, fmt.Errorf("machine auth context does not match report")
	}
	if in.LayoutID == uuid.Nil {
		return AckLayoutActivationResult{}, fmt.Errorf("layoutId is required")
	}
	q := pgxutil.NewQueries(s.Pool)
	if _, err := q.GetMachineLayoutByID(ctx, db.GetMachineLayoutByIDParams{
		ID:        in.LayoutID,
		MachineID: in.MachineID,
	}); err != nil {
		if err == pgx.ErrNoRows {
			return AckLayoutActivationResult{}, ErrLayoutNotFound
		}
		return AckLayoutActivationResult{}, err
	}
	if err := q.SetMachineActiveLayoutPointers(ctx, db.SetMachineActiveLayoutPointersParams{
		ID: in.MachineID,
		ActiveLayoutID: pgtypeUUID(in.LayoutID),
		ReportedActiveLayoutID: pgtypeUUID(in.LayoutID),
	}); err != nil {
		return AckLayoutActivationResult{}, err
	}
	return AckLayoutActivationResult{
		Accepted:               true,
		ReportedActiveLayoutID: in.LayoutID,
	}, nil
}

func pgtypeUUID(id uuid.UUID) pgtype.UUID {
	if id == uuid.Nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: id, Valid: true}
}
