package compliance

import (
	"time"

	"github.com/google/uuid"
)

// AuditLog is an append-only compliance record.
type AuditLog struct {
	ID           int64
	ActorType    string
	ActorID      string
	Action       string
	ResourceType string
	ResourceID   *uuid.UUID
	Payload      []byte
	IP           *string
	CreatedAt    time.Time
}
