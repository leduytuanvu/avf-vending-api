package retail

import (
	"time"

	"github.com/google/uuid"
)

// Product is a vendable SKU scoped to an organization.
type Product struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Sku            string
	Name           string
	Description    string
	Active         bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// PriceBook groups prices for a currency and effective window.
type PriceBook struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	Currency       string
	EffectiveFrom  time.Time
	IsDefault      bool
	CreatedAt      time.Time
}

// Planogram is a slot layout revision for an organization.
type Planogram struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	Revision       int32
	Status         string
	CreatedAt      time.Time
}

// Slot is one position in a planogram.
type Slot struct {
	ID          uuid.UUID
	PlanogramID uuid.UUID
	SlotIndex   int32
	ProductID   *uuid.UUID
	CreatedAt   time.Time
}

// MachineSlotState is live inventory/price projection for a machine slot.
type MachineSlotState struct {
	ID                       uuid.UUID
	MachineID                uuid.UUID
	PlanogramID              uuid.UUID
	SlotIndex                int32
	CurrentQuantity          int32
	PriceMinor               int64
	PlanogramRevisionApplied int32
	UpdatedAt                time.Time
}
