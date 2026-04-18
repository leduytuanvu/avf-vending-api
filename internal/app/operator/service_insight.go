package operator

import (
	"context"
	"strings"

	domainoperator "github.com/avf/avf-vending-api/internal/domain/operator"
	"github.com/google/uuid"
)

func timelineFetchLimit(limit int32) int32 {
	n := clampListLimit(limit)
	if n > 200 {
		return 200
	}
	return n
}

// ListAuthEventsForMachine returns auth audit rows for a machine (newest first).
func (s *Service) ListAuthEventsForMachine(ctx context.Context, organizationID, machineID uuid.UUID, limit int32) ([]domainoperator.AuthEvent, error) {
	machine, err := s.machines.GetByID(ctx, machineID)
	if err != nil {
		return nil, err
	}
	if machine.OrganizationID != organizationID {
		return nil, domainoperator.ErrOrganizationMismatch
	}
	return s.sessions.ListAuthEventsByMachineID(ctx, machineID, clampListLimit(limit))
}

// ListActionAttributionsForMachine returns operator-linked domain actions for a machine (newest first).
func (s *Service) ListActionAttributionsForMachine(ctx context.Context, organizationID, machineID uuid.UUID, limit int32) ([]domainoperator.ActionAttribution, error) {
	machine, err := s.machines.GetByID(ctx, machineID)
	if err != nil {
		return nil, err
	}
	if machine.OrganizationID != organizationID {
		return nil, domainoperator.ErrOrganizationMismatch
	}
	return s.sessions.ListActionAttributionsByMachineID(ctx, machineID, clampListLimit(limit))
}

// ListActionAttributionsForTechnician returns attributions for sessions owned by a technician within an organization.
func (s *Service) ListActionAttributionsForTechnician(ctx context.Context, organizationID, technicianID uuid.UUID, limit int32) ([]domainoperator.ActionAttribution, error) {
	tech, err := s.technicians.GetByID(ctx, technicianID)
	if err != nil {
		return nil, err
	}
	if tech.OrganizationID != organizationID {
		return nil, domainoperator.ErrOrganizationMismatch
	}
	return s.sessions.ListActionAttributionsForTechnician(ctx, organizationID, technicianID, clampListLimit(limit))
}

// ListActionAttributionsForUserPrincipal returns attributions for USER actor sessions in an organization.
func (s *Service) ListActionAttributionsForUserPrincipal(ctx context.Context, organizationID uuid.UUID, userPrincipal string, limit int32) ([]domainoperator.ActionAttribution, error) {
	principal := strings.TrimSpace(userPrincipal)
	if principal == "" {
		return nil, domainoperator.ErrInvalidActor
	}
	return s.sessions.ListActionAttributionsForUserPrincipal(ctx, organizationID, principal, clampListLimit(limit))
}

// BuildMachineOperatorTimeline merges recent auth events, action attributions, and session lifecycle markers.
func (s *Service) BuildMachineOperatorTimeline(ctx context.Context, organizationID, machineID uuid.UUID, limit int32) ([]TimelineItem, error) {
	machine, err := s.machines.GetByID(ctx, machineID)
	if err != nil {
		return nil, err
	}
	if machine.OrganizationID != organizationID {
		return nil, domainoperator.ErrOrganizationMismatch
	}
	fetch := timelineFetchLimit(limit)
	auth, err := s.sessions.ListAuthEventsByMachineID(ctx, machineID, fetch)
	if err != nil {
		return nil, err
	}
	attr, err := s.sessions.ListActionAttributionsByMachineID(ctx, machineID, fetch)
	if err != nil {
		return nil, err
	}
	sessions, err := s.sessions.ListSessionsByMachineID(ctx, machineID, fetch)
	if err != nil {
		return nil, err
	}
	return mergeMachineTimeline(auth, attr, sessions, clampListLimit(limit)), nil
}
