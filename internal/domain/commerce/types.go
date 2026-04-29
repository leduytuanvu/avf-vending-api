package commerce

import (
	"time"

	"github.com/google/uuid"
)

// Order is a domain projection of a persisted customer order.
type Order struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	MachineID      uuid.UUID
	Status         string
	Currency       string
	SubtotalMinor  int64
	TaxMinor       int64
	TotalMinor     int64
	IdempotencyKey *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// VendSession tracks a single vend attempt for an order line.
type VendSession struct {
	ID                    uuid.UUID
	OrderID               uuid.UUID
	MachineID             uuid.UUID
	SlotIndex             int32
	ProductID             uuid.UUID
	State                 string
	FinalCommandAttemptID *uuid.UUID
	CreatedAt             time.Time
}

// Payment represents money movement for an order.
type Payment struct {
	ID                   uuid.UUID
	OrderID              uuid.UUID
	Provider             string
	State                string
	AmountMinor          int64
	Currency             string
	IdempotencyKey       *string
	ReconciliationStatus string
	SettlementStatus     string
	SettlementBatchID    *uuid.UUID
	CreatedAt            time.Time
}

// OutboxEvent is a durable async propagation record (Postgres is SoR; Redis is not).
type OutboxEvent struct {
	ID             int64
	OrganizationID *uuid.UUID
	Topic          string
	EventType      string
	Payload        []byte
	AggregateType  string
	AggregateID    uuid.UUID
	IdempotencyKey *string
	CreatedAt      time.Time
	PublishedAt    *time.Time

	PublishAttemptCount  int32
	LastPublishError     *string
	LastPublishAttemptAt *time.Time
	NextPublishAfter     *time.Time
	DeadLetteredAt       *time.Time

	Status      string
	LockedBy    *string
	LockedUntil *time.Time

	UpdatedAt          time.Time
	MaxPublishAttempts int32
}
