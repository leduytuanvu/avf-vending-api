package setupapp

import (
	"context"

	"github.com/google/uuid"
)

// Repository persists machine topology (cabinets + layouts) and cabinet slot configs.
type Repository interface {
	UpsertMachineTopology(ctx context.Context, machineID uuid.UUID, cabinets []CabinetUpsert, layouts []TopologyLayoutUpsert) error
	SaveDraftOrCurrentSlotConfigs(ctx context.Context, machineID uuid.UUID, in SlotConfigSaveInput) error
	GetMachineBootstrap(ctx context.Context, machineID uuid.UUID) (MachineBootstrap, error)
	GetMachineSlotView(ctx context.Context, machineID uuid.UUID) (MachineSlotView, error)
}
