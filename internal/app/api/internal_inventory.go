package api

import (
	"context"

	"github.com/avf/avf-vending-api/internal/app/setupapp"
	"github.com/google/uuid"
)

// InternalInventoryQueryService exposes planogram slot inventory reads for internal gRPC (split-ready).
type InternalInventoryQueryService interface {
	GetMachineSlotView(ctx context.Context, machineID uuid.UUID) (setupapp.MachineSlotView, error)
}

type internalInventoryAdapter struct {
	m InternalMachineQueryService
}

// NewInternalInventoryQueryService adapts the machine internal query port for inventory-only callers.
func NewInternalInventoryQueryService(m InternalMachineQueryService) InternalInventoryQueryService {
	if m == nil {
		panic("api.NewInternalInventoryQueryService: nil machine query service")
	}
	return &internalInventoryAdapter{m: m}
}

func (a *internalInventoryAdapter) GetMachineSlotView(ctx context.Context, machineID uuid.UUID) (setupapp.MachineSlotView, error) {
	return a.m.GetMachineSlotView(ctx, machineID)
}
