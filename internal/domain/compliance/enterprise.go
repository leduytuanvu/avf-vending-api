package compliance

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// EnterpriseAuditRecord is a normalized append-only audit_events row input.
type EnterpriseAuditRecord struct {
	OrganizationID uuid.UUID
	ActorType      string
	ActorID        *string
	Action         string
	ResourceType   string
	ResourceID     *string
	// MachineID optionally scopes the event to a machine (inventory, commands, device flows).
	MachineID *uuid.UUID
	// SiteID optionally scopes the event to a site (fleet transfers, site-scoped operations).
	SiteID     *uuid.UUID
	RequestID  *string
	TraceID    *string
	IPAddress  *string
	UserAgent  *string
	BeforeJSON []byte
	AfterJSON  []byte
	Metadata   []byte
	// Outcome is success or failure (see OutcomeSuccess / OutcomeFailure). Empty defaults to success at persistence.
	Outcome string
	// OccurredAt is optional business time for the event; empty defaults to DB now() at insert.
	OccurredAt *time.Time
}

// EnterpriseRecorder persists rows to audit_events.
//
// Record logs administrative/inventory mutations where failing audit must not roll back the primary row
// (legacy behavior). Prefer RecordCritical for security/commerce/inventory guarantees.
//
// RecordCritical returns nil only when the audit row is persisted or CriticalFailOpen is enabled on the
// concrete audit.Service (development/test escape hatch).
//
// RecordCriticalTx must write inside the caller's transaction so audit failure rolls back the mutation.
type EnterpriseRecorder interface {
	Record(ctx context.Context, in EnterpriseAuditRecord) error
	RecordCritical(ctx context.Context, in EnterpriseAuditRecord) error
	RecordCriticalTx(ctx context.Context, tx pgx.Tx, in EnterpriseAuditRecord) error
}
