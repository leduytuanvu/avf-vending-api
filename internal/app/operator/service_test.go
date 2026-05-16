package operator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/domain/fleet"
	domainoperator "github.com/avf/avf-vending-api/internal/domain/operator"
	"github.com/google/uuid"
)

type memMachineRepo struct {
	m fleet.Machine
	e error
}

func (m memMachineRepo) GetByID(ctx context.Context, id uuid.UUID) (fleet.Machine, error) {
	if m.e != nil {
		return fleet.Machine{}, m.e
	}
	return m.m, nil
}

type memTechRepo struct {
	t fleet.Technician
	e error
}

func (m memTechRepo) GetByID(ctx context.Context, id uuid.UUID) (fleet.Technician, error) {
	if m.e != nil {
		return fleet.Technician{}, m.e
	}
	return m.t, nil
}

type noopOpRepo struct{}

func (noopOpRepo) StartOperatorSession(ctx context.Context, in domainoperator.StartOperatorSessionParams) (domainoperator.Session, error) {
	return domainoperator.Session{}, nil
}
func (noopOpRepo) GetOperatorSessionByID(ctx context.Context, id uuid.UUID) (domainoperator.Session, error) {
	return domainoperator.Session{}, domainoperator.ErrSessionNotFound
}
func (noopOpRepo) GetActiveSessionByMachineID(ctx context.Context, machineID uuid.UUID) (domainoperator.Session, error) {
	return domainoperator.Session{}, domainoperator.ErrNoActiveSession
}
func (noopOpRepo) EndOperatorSession(ctx context.Context, in domainoperator.EndOperatorSessionParams) (domainoperator.Session, error) {
	return domainoperator.Session{}, nil
}
func (noopOpRepo) TouchOperatorSessionActivity(ctx context.Context, sessionID uuid.UUID) (domainoperator.Session, error) {
	return domainoperator.Session{}, nil
}
func (noopOpRepo) TimeoutOperatorSessionIfExpired(ctx context.Context, sessionID uuid.UUID) (domainoperator.Session, error) {
	return domainoperator.Session{}, nil
}
func (noopOpRepo) InsertAuthEvent(ctx context.Context, in domainoperator.InsertAuthEventParams) (domainoperator.AuthEvent, error) {
	return domainoperator.AuthEvent{}, nil
}
func (noopOpRepo) InsertActionAttribution(ctx context.Context, in domainoperator.InsertActionAttributionParams) (domainoperator.ActionAttribution, error) {
	return domainoperator.ActionAttribution{}, nil
}
func (noopOpRepo) ListSessionsByMachineID(ctx context.Context, machineID uuid.UUID, limit int32) ([]domainoperator.Session, error) {
	return nil, nil
}
func (noopOpRepo) ListSessionsByTechnicianID(ctx context.Context, technicianID uuid.UUID, limit int32) ([]domainoperator.Session, error) {
	return nil, nil
}
func (noopOpRepo) ListSessionsByUserPrincipal(ctx context.Context, in domainoperator.ListSessionsParams) ([]domainoperator.Session, error) {
	return nil, nil
}

func (noopOpRepo) ListAuthEventsByMachineID(ctx context.Context, machineID uuid.UUID, limit int32) ([]domainoperator.AuthEvent, error) {
	return nil, nil
}

func (noopOpRepo) ListActionAttributionsByMachineID(ctx context.Context, machineID uuid.UUID, limit int32) ([]domainoperator.ActionAttribution, error) {
	return nil, nil
}

func (noopOpRepo) ListActionAttributionsByMachineAndResource(ctx context.Context, machineID uuid.UUID, resourceType, resourceID string, limit int32) ([]domainoperator.ActionAttribution, error) {
	return nil, nil
}

func (noopOpRepo) ListActionAttributionsForTechnician(ctx context.Context, companyID, technicianID uuid.UUID, limit int32) ([]domainoperator.ActionAttribution, error) {
	return nil, nil
}

func (noopOpRepo) ListActionAttributionsForUserPrincipal(ctx context.Context, companyID uuid.UUID, userPrincipal string, limit int32) ([]domainoperator.ActionAttribution, error) {
	return nil, nil
}

func TestStartOperatorSession_orgMismatchMachine(t *testing.T) {
	t.Skip("single-company mode has no machine scope mismatch")
}

func TestStartOperatorSession_orgMismatchTechnician(t *testing.T) {
	t.Skip("single-company mode has no technician scope mismatch")
}

func strPtr(s string) *string { return &s }

func TestEndOperatorSession_invalidFinalStatus(t *testing.T) {
	svc := NewService(noopOpRepo{}, memMachineRepo{m: fleet.Machine{}}, memTechRepo{}, nil)
	_, err := svc.EndOperatorSession(context.Background(), EndOperatorSessionInput{
		MachineID:   uuid.New(),
		SessionID:   uuid.New(),
		FinalStatus: "ACTIVE",
	})
	if err != domainoperator.ErrInvalidSessionEndStatus {
		t.Fatalf("expected ErrInvalidSessionEndStatus, got %v", err)
	}
}

type touchInactiveRepo struct {
	noopOpRepo
	s domainoperator.Session
}

func (r touchInactiveRepo) GetOperatorSessionByID(ctx context.Context, id uuid.UUID) (domainoperator.Session, error) {
	return r.s, nil
}

func TestTouchOperatorSessionActivity_requiresActive(t *testing.T) {
	mid := uuid.New()
	sid := uuid.New()
	sess := domainoperator.Session{
		ID:        sid,
		MachineID: mid,
		Status:    domainoperator.SessionStatusEnded,
	}
	svc := NewService(touchInactiveRepo{noopOpRepo{}, sess}, memMachineRepo{m: fleet.Machine{}}, memTechRepo{}, nil)
	_, err := svc.TouchOperatorSessionActivity(context.Background(), uuid.Nil, mid, sid)
	if err != domainoperator.ErrSessionNotActive {
		t.Fatalf("expected ErrSessionNotActive, got %v", err)
	}
}

type memAssignFalse struct{}

func (memAssignFalse) HasActiveAssignment(ctx context.Context, technicianID, machineID uuid.UUID) (bool, error) {
	_, _ = technicianID, machineID
	return false, nil
}

func TestStartTechnicianSession_notAssignedToMachine(t *testing.T) {
	mid := uuid.New()
	tid := uuid.New()
	svc := NewService(noopOpRepo{}, memMachineRepo{m: fleet.Machine{ID: mid}}, memTechRepo{t: fleet.Technician{ID: tid}}, memAssignFalse{})
	_, err := svc.StartOperatorSession(context.Background(), StartOperatorSessionInput{
		MachineID:    mid,
		ActorType:    domainoperator.ActorTypeTechnician,
		TechnicianID: &tid,
	})
	if !errors.Is(err, domainoperator.ErrTechnicianNotAssignedToMachine) {
		t.Fatalf("expected ErrTechnicianNotAssignedToMachine, got %v", err)
	}
}

func TestStartTechnicianSession_requiresAssignmentChecker(t *testing.T) {
	mid := uuid.New()
	tid := uuid.New()
	svc := NewService(noopOpRepo{}, memMachineRepo{m: fleet.Machine{ID: mid}}, memTechRepo{t: fleet.Technician{ID: tid}}, nil)
	_, err := svc.StartOperatorSession(context.Background(), StartOperatorSessionInput{
		MachineID:    mid,
		ActorType:    domainoperator.ActorTypeTechnician,
		TechnicianID: &tid,
	})
	if err != domainoperator.ErrTechnicianAssignmentCheckerRequired {
		t.Fatalf("expected ErrTechnicianAssignmentCheckerRequired, got %v", err)
	}
}

func TestStartSession_rejectsPastExpiry(t *testing.T) {
	mid := uuid.New()
	past := time.Now().UTC().Add(-time.Hour)
	svc := NewService(noopOpRepo{}, memMachineRepo{m: fleet.Machine{ID: mid}}, memTechRepo{}, nil)
	_, err := svc.StartOperatorSession(context.Background(), StartOperatorSessionInput{
		MachineID:     mid,
		ActorType:     domainoperator.ActorTypeUser,
		UserPrincipal: strPtr("sub-1"),
		ExpiresAt:     &past,
	})
	if err != domainoperator.ErrInvalidSessionExpiry {
		t.Fatalf("expected ErrInvalidSessionExpiry, got %v", err)
	}
}

func TestStartSession_adminTakeoverUnauthorized(t *testing.T) {
	mid := uuid.New()
	principal := "u-1"
	svc := NewService(noopOpRepo{}, memMachineRepo{m: fleet.Machine{ID: mid}}, memTechRepo{}, nil)
	_, err := svc.StartOperatorSession(context.Background(), StartOperatorSessionInput{
		MachineID:               mid,
		ActorType:               domainoperator.ActorTypeUser,
		UserPrincipal:           &principal,
		ForceAdminTakeover:      true,
		AdminTakeoverAuthorized: false,
	})
	if !errors.Is(err, domainoperator.ErrAdminTakeoverUnauthorized) {
		t.Fatalf("expected ErrAdminTakeoverUnauthorized, got %v", err)
	}
}

type sessionRepoStub struct {
	noopOpRepo
	sess domainoperator.Session
}

func (r sessionRepoStub) GetOperatorSessionByID(ctx context.Context, id uuid.UUID) (domainoperator.Session, error) {
	return r.sess, nil
}

func TestRecordActionAttribution_rejectsSessionMachineMismatch(t *testing.T) {
	machineA := uuid.New()
	machineB := uuid.New()
	sid := uuid.New()
	sess := domainoperator.Session{
		ID:        sid,
		MachineID: machineA,
		Status:    domainoperator.SessionStatusActive,
	}
	svc := NewService(sessionRepoStub{sess: sess}, memMachineRepo{m: fleet.Machine{ID: machineB}}, memTechRepo{}, nil)
	_, err := svc.RecordActionAttribution(context.Background(), RecordActionAttributionInput{
		MachineID:         machineB,
		OperatorSessionID: &sid,
		ActionOriginType:  domainoperator.ActionOriginOperatorSession,
		ResourceType:      "ledger",
		ResourceID:        uuid.NewString(),
	})
	if !errors.Is(err, domainoperator.ErrCompanyMismatch) {
		t.Fatalf("expected ErrCompanyMismatch for session/machine mismatch, got %v", err)
	}
}

func TestEndSession_rejectsWrongMachine(t *testing.T) {
	machineA := uuid.New()
	machineB := uuid.New()
	sid := uuid.New()
	sess := domainoperator.Session{
		ID:        sid,
		MachineID: machineA,
		Status:    domainoperator.SessionStatusActive,
	}
	svc := NewService(sessionRepoStub{sess: sess}, memMachineRepo{m: fleet.Machine{ID: machineA}}, memTechRepo{}, nil)
	_, err := svc.EndOperatorSession(context.Background(), EndOperatorSessionInput{
		MachineID:   machineB,
		SessionID:   sid,
		FinalStatus: domainoperator.SessionStatusEnded,
	})
	if err != domainoperator.ErrSessionMachineMismatch {
		t.Fatalf("expected ErrSessionMachineMismatch, got %v", err)
	}
}
