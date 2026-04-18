package operator

import (
	"time"

	"github.com/avf/avf-vending-api/internal/domain/fleet"
	domainoperator "github.com/avf/avf-vending-api/internal/domain/operator"
	"github.com/google/uuid"
)

// Deps wires operator session persistence and fleet reads used for validation.
type Deps struct {
	Sessions    domainoperator.Repository
	Machines    fleet.MachineRepository
	Technicians fleet.TechnicianRepository
	Assignments fleet.TechnicianMachineAssignmentChecker
}

// StartOperatorSessionInput is the public application input for opening a session on a machine.
// ActorType / TechnicianID / UserPrincipal must be derived from the authenticated principal at the
// edge (HTTP uses JWT only); do not accept unsanitized identity fields from untrusted clients.
type StartOperatorSessionInput struct {
	OrganizationID uuid.UUID
	MachineID      uuid.UUID
	ActorType      string
	TechnicianID   *uuid.UUID
	UserPrincipal  *string
	ExpiresAt      *time.Time
	ClientMetadata []byte
	// InitialAuthMethod, when non-empty, records login_success in the same transaction as session creation.
	InitialAuthMethod   string
	CorrelationID       *uuid.UUID
	InitialAuthMetadata []byte
	// StaleIdleReclaimAfter overrides the default idle window before a different operator can claim the machine (tests).
	StaleIdleReclaimAfter time.Duration
	// ForceAdminTakeover requests closing an in-use session immediately (org/platform admin only at HTTP edge).
	ForceAdminTakeover bool
	// AdminTakeoverAuthorized must be true when ForceAdminTakeover is true (set only by trusted adapters).
	AdminTakeoverAuthorized bool
}

// EndOperatorSessionInput ends an ACTIVE session as ENDED or REVOKED.
type EndOperatorSessionInput struct {
	OrganizationID uuid.UUID
	MachineID      uuid.UUID // must match the session row; prevents cross-machine session_id reuse
	SessionID      uuid.UUID
	FinalStatus    string
	EndedReason    string
	// Optional logout audit row after a successful end.
	LogoutAuthMethod    string
	LogoutCorrelationID *uuid.UUID
	LogoutMetadata      []byte
}

// TimeoutOperatorSessionInput expires a session when expires_at is in the past.
type TimeoutOperatorSessionInput struct {
	OrganizationID uuid.UUID
	MachineID      uuid.UUID // must match the session row
	SessionID      uuid.UUID
}

// RecordAuthEventInput appends an auth audit event. OperatorSessionID nil is allowed for
// machine-scoped login_failure rows (no session row created).
type RecordAuthEventInput struct {
	OrganizationID    uuid.UUID
	OperatorSessionID *uuid.UUID
	MachineID         uuid.UUID
	EventType         string
	AuthMethod        string
	OccurredAt        *time.Time
	CorrelationID     *uuid.UUID
	Metadata          []byte
}

// RecordActionAttributionInput records polymorphic resource attribution.
type RecordActionAttributionInput struct {
	OrganizationID    uuid.UUID
	OperatorSessionID *uuid.UUID
	MachineID         uuid.UUID
	ActionOriginType  string
	ResourceType      string
	ResourceID        string
	OccurredAt        *time.Time
	Metadata          []byte
	CorrelationID     *uuid.UUID
}
