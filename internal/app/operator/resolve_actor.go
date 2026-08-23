package operator

import (
	"context"
	"strings"

	domainoperator "github.com/avf/avf-vending-api/internal/domain/operator"
	"github.com/google/uuid"
)

// MachineActionActor is the server-derived actor for a machine mutation.
// Origin is never taken from the request body.
type MachineActionActor struct {
	MachineID         uuid.UUID
	Origin            string
	OperatorSessionID *uuid.UUID
}

// ResolveMachineActionActor validates a supplied operator session against policy.
//
// A supplied session is always validated (ACTIVE, same machine). Invalid sessions are never
// silently ignored. An omitted session is allowed only when policy is not SessionRequired;
// origin is then ActionOriginAPI.
func (s *Service) ResolveMachineActionActor(ctx context.Context, machineID uuid.UUID, supplied *uuid.UUID, policy domainoperator.SessionPolicy) (MachineActionActor, error) {
	if machineID == uuid.Nil {
		return MachineActionActor{}, domainoperator.ErrMachineContextRequired
	}
	if supplied != nil && *supplied != uuid.Nil {
		sess, err := s.GetSessionIfMatchesMachine(ctx, *supplied, machineID)
		if err != nil {
			return MachineActionActor{}, err
		}
		if !strings.EqualFold(strings.TrimSpace(sess.Status), domainoperator.SessionStatusActive) {
			return MachineActionActor{}, domainoperator.ErrSessionNotActive
		}
		id := sess.ID
		return MachineActionActor{
			MachineID:         machineID,
			Origin:            domainoperator.ActionOriginOperatorSession,
			OperatorSessionID: &id,
		}, nil
	}
	switch policy {
	case domainoperator.SessionRequired:
		return MachineActionActor{}, domainoperator.ErrOperatorSessionRequired
	case domainoperator.SessionOptionalByOrigin, domainoperator.SessionNotApplicable:
		return MachineActionActor{
			MachineID: machineID,
			Origin:    domainoperator.ActionOriginAPI,
		}, nil
	default:
		return MachineActionActor{}, domainoperator.ErrOperatorSessionRequired
	}
}
