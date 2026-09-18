package commerce

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// CashForensicStore persists hardware cash movement evidence.
type CashForensicStore interface {
	RecordCashMovements(ctx context.Context, in RecordCashMovementsInput) (RecordCashMovementsResult, error)
	BindAcceptanceEventsToOrder(ctx context.Context, machineID, orderID uuid.UUID, deviceEventIDs []string) error
	GetMachinePhysicalCashPosition(ctx context.Context, machineID uuid.UUID, currency string, asOf *time.Time) (MachinePhysicalCashPosition, error)
	InsertCashAdjustment(ctx context.Context, in InsertCashAdjustmentInput) (CashAdjustmentView, error)
	ListCashLedger(ctx context.Context, in ListCashLedgerInput) (ListCashLedgerResult, error)
	GetCashLedgerEventDetail(ctx context.Context, eventID uuid.UUID, movementClass string) (CashLedgerEventDetail, error)
	GetOrderCashForensics(ctx context.Context, orderID uuid.UUID) (OrderCashForensicsView, error)
}

// RecordCashMovementsInput batches movement events from the machine.
type RecordCashMovementsInput struct {
	MachineID      uuid.UUID
	IdempotencyKey string
	Events         []CashMovementEventInput
}

type CashMovementEventInput struct {
	Kind                      string
	DeviceEventID             string
	OccurredAt                time.Time
	DenominationMinor         int64
	AmountMinor               int64
	CreditSource              string
	LifecycleType             string
	WithdrawalID              string
	NoteSequence              int32
	EventType                 string
	RecyclerCountBefore       *int32
	RecyclerCountAfter        *int32
	OutcomeFinality           string
	RawRecordHex              string
	BootID                    string
	OrderID                   *uuid.UUID
	Currency                  string
	CashboxCount              *int32
	ObservationSource         string
	RecyclerDenominationMinor int64
	RawMetadata               []byte
}

type RecordCashMovementsResult struct {
	Replay          bool
	AcceptedCount   int32
	DuplicateCount  int32
}

type InsertCashAdjustmentInput struct {
	MachineID         uuid.UUID
	AmountMinor       int64
	Bucket            string
	Reason            string
	OperatorAccountID *uuid.UUID
	IdempotencyKey    string
	Currency          string
	Metadata          []byte
}

type CashAdjustmentView struct {
	ID           uuid.UUID
	MachineID    uuid.UUID
	AmountMinor  int64
	Bucket       string
	Reason       string
	Currency     string
	CreatedAt    time.Time
}

type MachinePhysicalCashPosition struct {
	MachineID                  uuid.UUID
	Currency                   string
	AsOf                       time.Time
	PhysicalExpectedCashboxMinor int64
	PhysicalExpectedRecyclerMinor  int64
	PhysicalExpectedMachineMinor   int64
	SalesNetExpectedMinor          int64
	ObservedRecyclerMinor          int64
	ObservedRecyclerCount          int32
	ObservedRecyclerDenomMinor     int64
	LastPhysicalCountMinor         int64
	LastPhysicalCountAt            *time.Time
	VarianceMinor                  int64
	UnresolvedLiabilityMinor       int64
	AmbiguousEventCount            int64
	EvidenceStatus                 string
	Disclosure                     string
}

type ListCashLedgerInput struct {
	MachineID      *uuid.UUID
	OrderID        *uuid.UUID
	From           time.Time
	To             time.Time
	Limit          int32
	AfterOccurredAt *time.Time
	AfterID        *uuid.UUID
	MovementClass  string
}

type CashLedgerRow struct {
	ID              uuid.UUID
	MachineID       uuid.UUID
	OrderID         *uuid.UUID
	MovementClass   string
	EventType       string
	Destination     string
	DenominationMinor int64
	AmountMinor     int64
	Currency        string
	OccurredAtDevice time.Time
	RecordedAt      time.Time
	EvidenceStatus  string
	DeviceEventID   string
}

type ListCashLedgerResult struct {
	Items      []CashLedgerRow
	NextCursor *CashLedgerCursor
}

type CashLedgerCursor struct {
	OccurredAt time.Time
	ID         uuid.UUID
}

type CashLedgerEventDetail struct {
	Row         CashLedgerRow
	RawMetadata map[string]any
	Extra       map[string]any
}

type OrderCashForensicsView struct {
	OrderMoneyView
	RemainderMinor int64
	LifecycleEvents []CashLifecycleEventView
	PayoutEvents    []CashPayoutEventView
}

type CashLifecycleEventView struct {
	DeviceEventID     string
	LifecycleType     string
	DenominationMinor int64
	RawRecordHex      string
	OccurredAt        time.Time
}

type CashPayoutEventView struct {
	ID                   uuid.UUID
	WithdrawalID         string
	NoteSequence         int32
	EventType            string
	DenominationMinor    int64
	AmountMinor          int64
	OutcomeFinality      string
	RecyclerCountBefore  *int32
	RecyclerCountAfter   *int32
	OccurredAt           time.Time
}
