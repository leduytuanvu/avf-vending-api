package operator

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/domain/fleet"
	domainoperator "github.com/avf/avf-vending-api/internal/domain/operator"
	"github.com/google/uuid"
)

// Service coordinates operator session lifecycle and attribution helpers.
//
// Machine vs operator identity: a fleet Machine is the long-lived asset and network identity at a
// site. An operator session is a short-lived, human-scoped context on that machine (who is at the
// device right now) used for attribution and audit. Sessions are not authentication tokens for
// the API; they bind actions taken on the machine to a technician or user principal until the
// session ends, expires, or is revoked.
//
// Ops: HTTP internal/httpserver/operator_http.go (error.code); DB machine_operator_sessions — ops/RUNBOOK.md.
type Service struct {
	sessions    domainoperator.Repository
	machines    fleet.MachineRepository
	technicians fleet.TechnicianRepository
	assignments fleet.TechnicianMachineAssignmentChecker
}

// NewService constructs an operator application service.
// assignments may be nil (tests only); production should pass a non-nil checker for technician flows.
func NewService(sessions domainoperator.Repository, machines fleet.MachineRepository, technicians fleet.TechnicianRepository, assignments fleet.TechnicianMachineAssignmentChecker) *Service {
	if sessions == nil {
		panic("operator.NewService: nil sessions repository")
	}
	if machines == nil {
		panic("operator.NewService: nil machines repository")
	}
	if technicians == nil {
		panic("operator.NewService: nil technicians repository")
	}
	return &Service{sessions: sessions, machines: machines, technicians: technicians, assignments: assignments}
}

// NewServiceFromDeps is a convenience wrapper over NewService.
func NewServiceFromDeps(d Deps) *Service {
	return NewService(d.Sessions, d.Machines, d.Technicians, d.Assignments)
}

// MachineByID loads a machine row (for HTTP access checks).
func (s *Service) MachineByID(ctx context.Context, machineID uuid.UUID) (fleet.Machine, error) {
	return s.machines.GetByID(ctx, machineID)
}

// GetSessionIfMatchesMachine returns a session when it exists and is tied to machineID.
func (s *Service) GetSessionIfMatchesMachine(ctx context.Context, sessionID, machineID uuid.UUID) (domainoperator.Session, error) {
	sess, err := s.sessions.GetOperatorSessionByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, domainoperator.ErrSessionNotFound) {
			return domainoperator.Session{}, domainoperator.ErrSessionNotFound
		}
		return domainoperator.Session{}, err
	}
	if sess.MachineID != machineID {
		return domainoperator.Session{}, domainoperator.ErrSessionMachineMismatch
	}
	return sess, nil
}

// ActiveSessionForMachine returns the active session for a machine or nil if none.
func (s *Service) ActiveSessionForMachine(ctx context.Context, machineID uuid.UUID) (*domainoperator.Session, error) {
	sess, err := s.sessions.GetActiveSessionByMachineID(ctx, machineID)
	if err != nil {
		if errors.Is(err, domainoperator.ErrNoActiveSession) {
			return nil, nil
		}
		return nil, err
	}
	return ptrSession(sess), nil
}

func clampListLimit(limit int32) int32 {
	switch {
	case limit <= 0:
		return 50
	case limit > 500:
		return 500
	default:
		return limit
	}
}

// StartOperatorSession validates actor and organization context, then creates one ACTIVE session.
// Caller-supplied actor fields must match the authenticated principal at the transport edge; HTTP
// handlers derive actor_type/technician_id/user_principal from JWT only.
func (s *Service) StartOperatorSession(ctx context.Context, in StartOperatorSessionInput) (domainoperator.Session, error) {
	userPrincipal := in.UserPrincipal
	if userPrincipal != nil {
		t := strings.TrimSpace(*userPrincipal)
		userPrincipal = &t
	}
	if err := domainoperator.ValidateActorConsistency(in.ActorType, in.TechnicianID, userPrincipal); err != nil {
		return domainoperator.Session{}, err
	}
	if err := domainoperator.ValidateSessionExpiryBounds(in.ExpiresAt, time.Now().UTC(), domainoperator.MaxOperatorSessionTTL); err != nil {
		return domainoperator.Session{}, err
	}
	machine, err := s.machines.GetByID(ctx, in.MachineID)
	if err != nil {
		return domainoperator.Session{}, err
	}
	if machine.OrganizationID != in.OrganizationID {
		return domainoperator.Session{}, domainoperator.ErrOrganizationMismatch
	}
	if in.ActorType == domainoperator.ActorTypeTechnician && in.TechnicianID != nil {
		tech, err := s.technicians.GetByID(ctx, *in.TechnicianID)
		if err != nil {
			return domainoperator.Session{}, err
		}
		if tech.OrganizationID != in.OrganizationID {
			return domainoperator.Session{}, domainoperator.ErrOrganizationMismatch
		}
		if s.assignments == nil {
			return domainoperator.Session{}, domainoperator.ErrTechnicianAssignmentCheckerRequired
		}
		ok, err := s.assignments.HasActiveAssignment(ctx, *in.TechnicianID, in.MachineID)
		if err != nil {
			return domainoperator.Session{}, err
		}
		if !ok {
			return domainoperator.Session{}, domainoperator.ErrTechnicianNotAssignedToMachine
		}
	}

	var initial *domainoperator.InitialSessionAuth
	if strings.TrimSpace(in.InitialAuthMethod) != "" {
		if err := domainoperator.ValidateAuthEventSemantics(domainoperator.AuthEventLoginSuccess, in.InitialAuthMethod); err != nil {
			return domainoperator.Session{}, err
		}
		initial = &domainoperator.InitialSessionAuth{
			EventType:     domainoperator.AuthEventLoginSuccess,
			AuthMethod:    strings.TrimSpace(in.InitialAuthMethod),
			CorrelationID: in.CorrelationID,
			Metadata:      in.InitialAuthMetadata,
		}
	}

	if in.ForceAdminTakeover && !in.AdminTakeoverAuthorized {
		return domainoperator.Session{}, domainoperator.ErrAdminTakeoverUnauthorized
	}

	return s.sessions.StartOperatorSession(ctx, domainoperator.StartOperatorSessionParams{
		OrganizationID:          in.OrganizationID,
		MachineID:               in.MachineID,
		ActorType:               in.ActorType,
		TechnicianID:            in.TechnicianID,
		UserPrincipal:           userPrincipal,
		ExpiresAt:               in.ExpiresAt,
		ClientMetadata:          in.ClientMetadata,
		InitialAuth:             initial,
		StaleIdleReclaimAfter:   in.StaleIdleReclaimAfter,
		ForceAdminTakeover:      in.ForceAdminTakeover,
		AdminTakeoverAuthorized: in.AdminTakeoverAuthorized,
	})
}

// EndOperatorSession ends an ACTIVE session if it belongs to the organization and machine.
func (s *Service) EndOperatorSession(ctx context.Context, in EndOperatorSessionInput) (domainoperator.Session, error) {
	switch in.FinalStatus {
	case domainoperator.SessionStatusEnded, domainoperator.SessionStatusRevoked:
	default:
		return domainoperator.Session{}, domainoperator.ErrInvalidSessionEndStatus
	}
	if in.MachineID == uuid.Nil {
		return domainoperator.Session{}, domainoperator.ErrMachineContextRequired
	}
	if strings.TrimSpace(in.LogoutAuthMethod) != "" {
		if err := domainoperator.ValidateAuthEventSemantics(domainoperator.AuthEventLogout, in.LogoutAuthMethod); err != nil {
			return domainoperator.Session{}, err
		}
	}
	sess, err := s.sessions.GetOperatorSessionByID(ctx, in.SessionID)
	if err != nil {
		if errors.Is(err, domainoperator.ErrSessionNotFound) {
			return domainoperator.Session{}, domainoperator.ErrSessionNotFound
		}
		return domainoperator.Session{}, err
	}
	if sess.MachineID != in.MachineID {
		return domainoperator.Session{}, domainoperator.ErrSessionMachineMismatch
	}
	if sess.OrganizationID != in.OrganizationID {
		return domainoperator.Session{}, domainoperator.ErrOrganizationMismatch
	}
	if sess.Status != domainoperator.SessionStatusActive {
		return domainoperator.Session{}, domainoperator.ErrSessionNotActive
	}
	var endedReason *string
	if strings.TrimSpace(in.EndedReason) != "" {
		r := strings.TrimSpace(in.EndedReason)
		endedReason = &r
	}
	endParams := domainoperator.EndOperatorSessionParams{
		SessionID:   in.SessionID,
		Status:      in.FinalStatus,
		EndedAt:     time.Now().UTC(),
		EndedReason: endedReason,
	}
	if strings.TrimSpace(in.LogoutAuthMethod) != "" {
		endParams.Logout = &domainoperator.InsertAuthEventParams{
			EventType:     domainoperator.AuthEventLogout,
			AuthMethod:    strings.TrimSpace(in.LogoutAuthMethod),
			OccurredAt:    nil,
			CorrelationID: in.LogoutCorrelationID,
			Metadata:      in.LogoutMetadata,
		}
	}
	ended, err := s.sessions.EndOperatorSession(ctx, endParams)
	if err != nil {
		return domainoperator.Session{}, err
	}
	return ended, nil
}

// TimeoutOperatorSession marks EXPIRED when expires_at is due; organization and machine must match.
func (s *Service) TimeoutOperatorSession(ctx context.Context, in TimeoutOperatorSessionInput) (domainoperator.Session, error) {
	if in.MachineID == uuid.Nil {
		return domainoperator.Session{}, domainoperator.ErrMachineContextRequired
	}
	sess, err := s.sessions.GetOperatorSessionByID(ctx, in.SessionID)
	if err != nil {
		if errors.Is(err, domainoperator.ErrSessionNotFound) {
			return domainoperator.Session{}, domainoperator.ErrSessionNotFound
		}
		return domainoperator.Session{}, err
	}
	if sess.MachineID != in.MachineID {
		return domainoperator.Session{}, domainoperator.ErrSessionMachineMismatch
	}
	if sess.OrganizationID != in.OrganizationID {
		return domainoperator.Session{}, domainoperator.ErrOrganizationMismatch
	}
	row, err := s.sessions.TimeoutOperatorSessionIfExpired(ctx, in.SessionID)
	if err != nil {
		if errors.Is(err, domainoperator.ErrTimeoutNotApplicable) {
			return domainoperator.Session{}, domainoperator.ErrTimeoutNotApplicable
		}
		return domainoperator.Session{}, err
	}
	return row, nil
}

// HeartbeatOperatorSession bumps last activity and expires the session when past expires_at.
func (s *Service) HeartbeatOperatorSession(ctx context.Context, organizationID, machineID, sessionID uuid.UUID) (domainoperator.Session, error) {
	sess, err := s.TouchOperatorSessionActivity(ctx, organizationID, machineID, sessionID)
	if err != nil {
		return domainoperator.Session{}, err
	}
	timed, err := s.sessions.TimeoutOperatorSessionIfExpired(ctx, sessionID)
	if err == nil {
		return timed, nil
	}
	if errors.Is(err, domainoperator.ErrTimeoutNotApplicable) {
		return sess, nil
	}
	return domainoperator.Session{}, err
}

// ResolveCurrentOperatorForMachine returns the active session for a machine, scoped to an organization.
func (s *Service) ResolveCurrentOperatorForMachine(ctx context.Context, organizationID, machineID uuid.UUID) (domainoperator.CurrentOperatorResolution, error) {
	machine, err := s.machines.GetByID(ctx, machineID)
	if err != nil {
		return domainoperator.CurrentOperatorResolution{}, err
	}
	if machine.OrganizationID != organizationID {
		return domainoperator.CurrentOperatorResolution{}, domainoperator.ErrOrganizationMismatch
	}
	sess, err := s.sessions.GetActiveSessionByMachineID(ctx, machineID)
	if err != nil {
		if errors.Is(err, domainoperator.ErrNoActiveSession) {
			return domainoperator.CurrentOperatorResolution{
				MachineID:             machineID,
				OrganizationID:        organizationID,
				ActiveSession:         nil,
				TechnicianDisplayName: nil,
			}, nil
		}
		return domainoperator.CurrentOperatorResolution{}, err
	}
	out := domainoperator.CurrentOperatorResolution{
		MachineID:      machineID,
		OrganizationID: organizationID,
		ActiveSession:  ptrSession(sess),
	}
	if sess.ActorType == domainoperator.ActorTypeTechnician && sess.TechnicianID != nil {
		tech, err := s.technicians.GetByID(ctx, *sess.TechnicianID)
		if err == nil {
			dn := tech.DisplayName
			out.TechnicianDisplayName = &dn
		}
	}
	return out, nil
}

func ptrSession(s domainoperator.Session) *domainoperator.Session {
	cp := s
	return &cp
}

// TouchOperatorSessionActivity bumps updated_at for an ACTIVE session in the caller organization and machine.
func (s *Service) TouchOperatorSessionActivity(ctx context.Context, organizationID, machineID, sessionID uuid.UUID) (domainoperator.Session, error) {
	if machineID == uuid.Nil {
		return domainoperator.Session{}, domainoperator.ErrMachineContextRequired
	}
	sess, err := s.sessions.GetOperatorSessionByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, domainoperator.ErrSessionNotFound) {
			return domainoperator.Session{}, domainoperator.ErrSessionNotFound
		}
		return domainoperator.Session{}, err
	}
	if sess.MachineID != machineID {
		return domainoperator.Session{}, domainoperator.ErrSessionMachineMismatch
	}
	if sess.OrganizationID != organizationID {
		return domainoperator.Session{}, domainoperator.ErrOrganizationMismatch
	}
	if sess.Status != domainoperator.SessionStatusActive {
		return domainoperator.Session{}, domainoperator.ErrSessionNotActive
	}
	return s.sessions.TouchOperatorSessionActivity(ctx, sessionID)
}

// ListSessionsByMachine lists historical sessions for a machine (newest first).
func (s *Service) ListSessionsByMachine(ctx context.Context, organizationID, machineID uuid.UUID, limit int32) ([]domainoperator.Session, error) {
	machine, err := s.machines.GetByID(ctx, machineID)
	if err != nil {
		return nil, err
	}
	if machine.OrganizationID != organizationID {
		return nil, domainoperator.ErrOrganizationMismatch
	}
	return s.sessions.ListSessionsByMachineID(ctx, machineID, clampListLimit(limit))
}

// ListSessionsByTechnician lists sessions attributed to a technician identity.
func (s *Service) ListSessionsByTechnician(ctx context.Context, organizationID, technicianID uuid.UUID, limit int32) ([]domainoperator.Session, error) {
	tech, err := s.technicians.GetByID(ctx, technicianID)
	if err != nil {
		return nil, err
	}
	if tech.OrganizationID != organizationID {
		return nil, domainoperator.ErrOrganizationMismatch
	}
	return s.sessions.ListSessionsByTechnicianID(ctx, technicianID, clampListLimit(limit))
}

// ListSessionsByUser lists USER actor sessions for a principal within an organization.
func (s *Service) ListSessionsByUser(ctx context.Context, organizationID uuid.UUID, userPrincipal string, limit int32) ([]domainoperator.Session, error) {
	principal := strings.TrimSpace(userPrincipal)
	if principal == "" {
		return nil, domainoperator.ErrInvalidActor
	}
	return s.sessions.ListSessionsByUserPrincipal(ctx, domainoperator.ListSessionsParams{
		OrganizationID: organizationID,
		UserPrincipal:  principal,
		Limit:          clampListLimit(limit),
	})
}

// RecordAuthEvent validates and appends an auth audit row.
func (s *Service) RecordAuthEvent(ctx context.Context, in RecordAuthEventInput) (domainoperator.AuthEvent, error) {
	if err := domainoperator.ValidateAuthEventSemantics(in.EventType, in.AuthMethod); err != nil {
		return domainoperator.AuthEvent{}, err
	}
	if in.OperatorSessionID != nil {
		sess, err := s.sessions.GetOperatorSessionByID(ctx, *in.OperatorSessionID)
		if err != nil {
			if errors.Is(err, domainoperator.ErrSessionNotFound) {
				return domainoperator.AuthEvent{}, domainoperator.ErrSessionNotFound
			}
			return domainoperator.AuthEvent{}, err
		}
		if sess.OrganizationID != in.OrganizationID || sess.MachineID != in.MachineID {
			return domainoperator.AuthEvent{}, domainoperator.ErrOrganizationMismatch
		}
	} else {
		machine, err := s.machines.GetByID(ctx, in.MachineID)
		if err != nil {
			return domainoperator.AuthEvent{}, err
		}
		if machine.OrganizationID != in.OrganizationID {
			return domainoperator.AuthEvent{}, domainoperator.ErrOrganizationMismatch
		}
	}
	return s.sessions.InsertAuthEvent(ctx, domainoperator.InsertAuthEventParams{
		OperatorSessionID: in.OperatorSessionID,
		MachineID:         in.MachineID,
		EventType:         in.EventType,
		AuthMethod:        in.AuthMethod,
		OccurredAt:        in.OccurredAt,
		CorrelationID:     in.CorrelationID,
		Metadata:          in.Metadata,
	})
}

// RecordActionAttribution validates and inserts an attribution row.
// When OperatorSessionID is set, the loaded session must match in.MachineID (same org). Never
// relax that check: attributions are the forensic link between operator_session and domain rows.
func (s *Service) RecordActionAttribution(ctx context.Context, in RecordActionAttributionInput) (domainoperator.ActionAttribution, error) {
	if err := domainoperator.ValidateActionAttributionSemantics(in.ActionOriginType); err != nil {
		return domainoperator.ActionAttribution{}, err
	}
	machine, err := s.machines.GetByID(ctx, in.MachineID)
	if err != nil {
		return domainoperator.ActionAttribution{}, err
	}
	if machine.OrganizationID != in.OrganizationID {
		return domainoperator.ActionAttribution{}, domainoperator.ErrOrganizationMismatch
	}
	if in.OperatorSessionID != nil {
		sess, err := s.sessions.GetOperatorSessionByID(ctx, *in.OperatorSessionID)
		if err != nil {
			if errors.Is(err, domainoperator.ErrSessionNotFound) {
				return domainoperator.ActionAttribution{}, domainoperator.ErrSessionNotFound
			}
			return domainoperator.ActionAttribution{}, err
		}
		if sess.OrganizationID != in.OrganizationID || sess.MachineID != in.MachineID {
			return domainoperator.ActionAttribution{}, domainoperator.ErrOrganizationMismatch
		}
	}
	return s.sessions.InsertActionAttribution(ctx, domainoperator.InsertActionAttributionParams{
		OperatorSessionID: in.OperatorSessionID,
		MachineID:         in.MachineID,
		ActionOriginType:  in.ActionOriginType,
		ResourceType:      in.ResourceType,
		ResourceID:        in.ResourceID,
		OccurredAt:        in.OccurredAt,
		Metadata:          in.Metadata,
		CorrelationID:     in.CorrelationID,
	})
}
