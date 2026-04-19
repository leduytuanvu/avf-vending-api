package inventoryadmin

import (
	"context"
	"errors"
	"fmt"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Service reads machine slot state for admin UIs.
type Service struct {
	q *db.Queries
}

// NewService constructs an inventory admin reader.
func NewService(q *db.Queries) (*Service, error) {
	if q == nil {
		return nil, fmt.Errorf("inventoryadmin: nil Queries")
	}
	return &Service{q: q}, nil
}

// MachineHead is minimal machine metadata for responses.
type MachineHead struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	Status         string
}

// ResolveMachine loads the machine row or ErrMachineNotFound.
func (s *Service) ResolveMachine(ctx context.Context, machineID uuid.UUID) (MachineHead, error) {
	if s == nil {
		return MachineHead{}, errors.New("inventoryadmin: nil service")
	}
	if machineID == uuid.Nil {
		return MachineHead{}, ErrMachineNotFound
	}
	row, err := s.q.InventoryAdminGetMachineOrg(ctx, machineID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return MachineHead{}, ErrMachineNotFound
		}
		return MachineHead{}, err
	}
	return MachineHead{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		Name:           row.Name,
		Status:         row.Status,
	}, nil
}

// ListSlots returns joined slot / planogram / product rows for a machine.
func (s *Service) ListSlots(ctx context.Context, machineID uuid.UUID) ([]db.InventoryAdminListMachineSlotsRow, error) {
	if s == nil {
		return nil, errors.New("inventoryadmin: nil service")
	}
	return s.q.InventoryAdminListMachineSlots(ctx, machineID)
}

// AggregateInventory returns per-product rollups for a machine.
func (s *Service) AggregateInventory(ctx context.Context, machineID uuid.UUID) ([]db.InventoryAdminAggregateMachineInventoryRow, error) {
	if s == nil {
		return nil, errors.New("inventoryadmin: nil service")
	}
	return s.q.InventoryAdminAggregateMachineInventory(ctx, machineID)
}
