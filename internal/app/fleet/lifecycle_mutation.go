package fleet

import (
	"errors"
	"strings"
	"time"

	domainfleet "github.com/avf/avf-vending-api/internal/domain/fleet"
	"github.com/google/uuid"
)

// LifecycleMutationInput captures accountability fields for admin/technician lifecycle mutations.
type LifecycleMutationInput struct {
	Reason            string
	Notes             string
	OperatorSessionID *uuid.UUID
	CorrelationID     *uuid.UUID
	ActorAccountID    uuid.UUID
	RequestID         string
	Metadata          map[string]any
}

// LifecycleMutationResult is the standard enterprise lifecycle mutation response payload.
type LifecycleMutationResult struct {
	PreviousStatus          string
	NewStatus               string
	CredentialVersion       int64
	SessionsRevokedCount    int
	CredentialsRevokedCount int
	ActorAccountID          uuid.UUID
	OperatorSessionID       *uuid.UUID
	Reason                  string
	OccurredAt              time.Time
	CorrelationID           *uuid.UUID
}

var (
	// ErrLifecycleReasonRequired is returned when a mutating lifecycle action lacks reason.
	ErrLifecycleReasonRequired = errors.New("fleet: reason is required")
	// ErrOperatorSessionRequired is returned when technician-on-app action lacks active operator session.
	ErrOperatorSessionRequired = errors.New("fleet: operator_session_id is required for technician actions")
)

// Actions that require a non-empty reason string.
var lifecycleReasonRequired = map[string]struct{}{
	"suspend":            {},
	"archive":            {},
	"retire":             {},
	"mark-compromised":   {},
	"revoke-credentials": {},
	"revoke-sessions":    {},
	"rotate-credentials": {},
}

// ValidateLifecycleMutation enforces reason and optional operator session requirements.
func ValidateLifecycleMutation(action string, in LifecycleMutationInput, technicianOrigin bool) error {
	action = strings.ToLower(strings.TrimSpace(action))
	if _, ok := lifecycleReasonRequired[action]; ok {
		if strings.TrimSpace(in.Reason) == "" {
			return ErrLifecycleReasonRequired
		}
	}
	if technicianOrigin {
		if in.OperatorSessionID == nil || *in.OperatorSessionID == uuid.Nil {
			return ErrOperatorSessionRequired
		}
	}
	return nil
}

func lifecycleResult(prev, next string, m domainfleet.Machine, in LifecycleMutationInput) LifecycleMutationResult {
	return LifecycleMutationResult{
		PreviousStatus:    prev,
		NewStatus:         next,
		CredentialVersion: m.CredentialVersion,
		ActorAccountID:    in.ActorAccountID,
		OperatorSessionID: in.OperatorSessionID,
		Reason:            strings.TrimSpace(in.Reason),
		OccurredAt:        time.Now().UTC(),
		CorrelationID:     in.CorrelationID,
	}
}

// LifecycleMutationOutcome combines machine row and mutation metadata.
type LifecycleMutationOutcome struct {
	Machine domainfleet.Machine
	Result  LifecycleMutationResult
}
