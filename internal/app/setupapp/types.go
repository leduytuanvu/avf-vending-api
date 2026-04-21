package setupapp

import (
	"time"

	domainfleet "github.com/avf/avf-vending-api/internal/domain/fleet"
	"github.com/google/uuid"
)

// CabinetUpsert is a single cabinet row upserted under a machine.
type CabinetUpsert struct {
	Code      string
	Title     string
	SortOrder int32
	Metadata  []byte
}

// TopologyLayoutUpsert upserts a cabinet-scoped slot layout revision.
type TopologyLayoutUpsert struct {
	CabinetCode string
	LayoutKey   string
	Revision    int32
	LayoutSpec  []byte
	Status      string
}

// SlotConfigSaveInput writes draft or current machine_slot_configs rows and optionally syncs legacy machine_slot_state.
type SlotConfigSaveInput struct {
	PlanogramID         uuid.UUID
	PlanogramRevision   int32
	PublishAsCurrent    bool
	SyncLegacyReadModel bool
	Items               []SlotConfigSaveItem
}

// SlotConfigSaveItem targets a published layout revision and cabinet identified by code.
type SlotConfigSaveItem struct {
	CabinetCode     string
	LayoutKey       string
	LayoutRevision  int32
	SlotCode        string
	LegacySlotIndex *int32
	ProductID       *uuid.UUID
	MaxQuantity     int32
	PriceMinor      int64
	EffectiveFrom   time.Time
	Metadata        []byte
}

// CabinetView is a persisted machine cabinet.
type CabinetView struct {
	ID        uuid.UUID
	MachineID uuid.UUID
	Code      string
	Title     string
	SortOrder int32
	Metadata  []byte
	CreatedAt time.Time
	UpdatedAt time.Time
}

// AssortmentProductView is one product in the machine's active primary assortment.
type AssortmentProductView struct {
	ProductID      uuid.UUID
	SKU            string
	Name           string
	SortOrder      int32
	AssortmentID   uuid.UUID
	AssortmentName string
}

// CabinetSlotConfigView is a current machine_slot_configs row with cabinet context.
type CabinetSlotConfigView struct {
	ConfigID          uuid.UUID
	CabinetCode       string
	SlotCode          string
	SlotIndex         *int32
	ProductID         *uuid.UUID
	ProductSKU        string
	ProductName       string
	MaxQuantity       int32
	PriceMinor        int64
	EffectiveFrom     time.Time
	IsCurrent         bool
	MachineSlotLayout uuid.UUID
}

// MachineBootstrap aggregates machine identity, cabinets, primary assortment, and current cabinet slot configs.
type MachineBootstrap struct {
	Machine             domainfleet.Machine
	Cabinets            []CabinetView
	AssortmentProducts  []AssortmentProductView
	CurrentCabinetSlots []CabinetSlotConfigView
}

// LegacySlotRow mirrors planogram-backed machine_slot_state joined to catalog (legacy read model).
type LegacySlotRow struct {
	PlanogramID       uuid.UUID
	PlanogramName     string
	SlotIndex         int32
	CurrentQuantity   int32
	MaxQuantity       int32
	PriceMinor        int64
	ProductID         *uuid.UUID
	ProductSKU        string
	ProductName       string
	PlanogramRevision int32
}

// ConfiguredSlotRow is a current cabinet slot config row.
type ConfiguredSlotRow struct {
	CabinetCode string
	SlotCode    string
	SlotIndex   *int32
	ProductID   *uuid.UUID
	ProductSKU  string
	ProductName string
	MaxQuantity int32
	PriceMinor  int64
}

// MachineSlotView joins legacy planogram slot state with new cabinet slot configs.
type MachineSlotView struct {
	LegacySlots     []LegacySlotRow
	ConfiguredSlots []ConfiguredSlotRow
}
