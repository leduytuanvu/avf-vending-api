package compliance

import (
	"context"

	"github.com/google/uuid"
)

// AuditRecord is input for persisting an audit log row.
type AuditRecord struct {
	OrganizationID uuid.UUID
	ActorType      string
	ActorID        string
	Action         string
	ResourceType   string
	ResourceID     *uuid.UUID
	Payload        []byte
	IP             *string
}

// AuditRepository appends audit records to durable storage.
type AuditRepository interface {
	Record(ctx context.Context, in AuditRecord) (AuditLog, error)
}
