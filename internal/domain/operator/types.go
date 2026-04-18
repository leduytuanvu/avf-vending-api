package operator

import (
	"time"

	"github.com/google/uuid"
)

// Actor and session status values match DB CHECK constraints (migrations/00008_machine_operator_sessions.sql).
const (
	ActorTypeTechnician = "TECHNICIAN"
	ActorTypeUser       = "USER"
)

const (
	SessionStatusActive  = "ACTIVE"
	SessionStatusEnded   = "ENDED"
	SessionStatusExpired = "EXPIRED"
	SessionStatusRevoked = "REVOKED"
)

// Session end reasons persisted on machine_operator_sessions.ended_reason (free text; use these
// literals for stable APIs and audit queries).
const (
	EndedReasonStaleSessionReclaimed = "stale_session_reclaimed"
	EndedReasonAdminForcedTakeover   = "admin_forced_takeover"
)

// Auth methods match machine_operator_auth_events.auth_method CHECK.
const (
	AuthMethodPIN        = "pin"
	AuthMethodPassword   = "password"
	AuthMethodBadge      = "badge"
	AuthMethodOIDC       = "oidc"
	AuthMethodDeviceCert = "device_cert"
	AuthMethodUnknown    = "unknown"
)

// Auth event types match machine_operator_auth_events.event_type CHECK.
const (
	AuthEventLoginSuccess   = "login_success"
	AuthEventLoginFailure   = "login_failure"
	AuthEventLogout         = "logout"
	AuthEventSessionRefresh = "session_refresh"
	AuthEventLockout        = "lockout"
	AuthEventUnknown        = "unknown"
)

// Action origin types match machine_action_attributions.action_origin_type CHECK.
const (
	ActionOriginOperatorSession = "operator_session"
	ActionOriginSystem          = "system"
	ActionOriginScheduled       = "scheduled"
	ActionOriginAPI             = "api"
	ActionOriginRemoteSupport   = "remote_support"
)

// Session is a machine-bound operator login context (human, temporary).
type Session struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	MachineID      uuid.UUID
	ActorType      string
	TechnicianID   *uuid.UUID
	UserPrincipal  *string
	Status         string
	StartedAt      time.Time
	EndedAt        *time.Time
	ExpiresAt      *time.Time
	ClientMetadata []byte
	LastActivityAt time.Time
	EndedReason    *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// AuthEvent is an append-only audit row for operator authentication.
type AuthEvent struct {
	ID                int64
	OperatorSessionID *uuid.UUID
	MachineID         uuid.UUID
	EventType         string
	AuthMethod        string
	OccurredAt        time.Time
	CorrelationID     *uuid.UUID
	Metadata          []byte
}

// ActionAttribution links a domain resource to an operator session (or unattended origin).
type ActionAttribution struct {
	ID                int64
	OperatorSessionID *uuid.UUID
	MachineID         uuid.UUID
	ActionOriginType  string
	ResourceType      string
	ResourceID        string
	OccurredAt        time.Time
	Metadata          []byte
	CorrelationID     *uuid.UUID
}

// InitialSessionAuth is written in the same transaction as session creation when non-nil.
type InitialSessionAuth struct {
	EventType     string
	AuthMethod    string
	CorrelationID *uuid.UUID
	Metadata      []byte
}

// StartOperatorSessionParams is the transactional create payload.
type StartOperatorSessionParams struct {
	OrganizationID uuid.UUID
	MachineID      uuid.UUID
	ActorType      string
	TechnicianID   *uuid.UUID
	UserPrincipal  *string
	ExpiresAt      *time.Time
	ClientMetadata []byte
	InitialAuth    *InitialSessionAuth
	// StaleIdleReclaimAfter is the idle window after which another principal may claim the machine.
	// Zero defaults to DefaultStaleIdleReclaimForDifferentOperator.
	StaleIdleReclaimAfter time.Duration
	// ForceAdminTakeover ends a non-stale active session so this login can proceed (REVOKED +
	// EndedReasonAdminForcedTakeover). Must only be set when AdminTakeoverAuthorized is true.
	ForceAdminTakeover bool
	// AdminTakeoverAuthorized must be true whenever ForceAdminTakeover is true (trusted adapter).
	AdminTakeoverAuthorized bool
}

// EndOperatorSessionParams closes an ACTIVE session.
type EndOperatorSessionParams struct {
	SessionID   uuid.UUID
	Status      string
	EndedAt     time.Time
	EndedReason *string
	// Logout, when non-nil, is persisted in the same transaction as the session transition.
	// Repository sets OperatorSessionID and MachineID from the ended session row.
	Logout *InsertAuthEventParams
}

// InsertAuthEventParams appends an auth audit row.
type InsertAuthEventParams struct {
	OperatorSessionID *uuid.UUID
	MachineID         uuid.UUID
	EventType         string
	AuthMethod        string
	OccurredAt        *time.Time
	CorrelationID     *uuid.UUID
	Metadata          []byte
}

// InsertActionAttributionParams records resource-level attribution.
type InsertActionAttributionParams struct {
	OperatorSessionID *uuid.UUID
	MachineID         uuid.UUID
	ActionOriginType  string
	ResourceType      string
	ResourceID        string
	OccurredAt        *time.Time
	Metadata          []byte
	CorrelationID     *uuid.UUID
}

// ListSessionsParams scopes list-by-user queries.
type ListSessionsParams struct {
	OrganizationID uuid.UUID
	UserPrincipal  string
	Limit          int32
}

// CurrentOperatorResolution is the service-layer answer for who is operating this machine now.
type CurrentOperatorResolution struct {
	MachineID             uuid.UUID
	OrganizationID        uuid.UUID
	ActiveSession         *Session
	TechnicianDisplayName *string
}
