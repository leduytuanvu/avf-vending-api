package operator

import (
	"context"
	"testing"

	"github.com/avf/avf-vending-api/internal/domain/fleet"
	domainoperator "github.com/avf/avf-vending-api/internal/domain/operator"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"github.com/google/uuid"
)

type sessionRepo struct {
	noopOpRepo
	sess domainoperator.Session
	err  error
}

func (r sessionRepo) GetOperatorSessionByID(ctx context.Context, id uuid.UUID) (domainoperator.Session, error) {
	if r.err != nil {
		return domainoperator.Session{}, r.err
	}
	return r.sess, nil
}

func TestResolveMachineActionActor_optionalOmitsSessionUsesAPIOrigin(t *testing.T) {
	mid := id.NewUUIDV7()
	svc := NewService(noopOpRepo{}, memMachineRepo{m: fleet.Machine{ID: mid}}, memTechRepo{}, nil)
	got, err := svc.ResolveMachineActionActor(context.Background(), mid, nil, domainoperator.SessionOptionalByOrigin)
	if err != nil {
		t.Fatal(err)
	}
	if got.Origin != domainoperator.ActionOriginAPI || got.OperatorSessionID != nil {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveMachineActionActor_requiredWithoutSession(t *testing.T) {
	mid := id.NewUUIDV7()
	svc := NewService(noopOpRepo{}, memMachineRepo{m: fleet.Machine{ID: mid}}, memTechRepo{}, nil)
	_, err := svc.ResolveMachineActionActor(context.Background(), mid, nil, domainoperator.SessionRequired)
	if err != domainoperator.ErrOperatorSessionRequired {
		t.Fatalf("got %v", err)
	}
}

func TestResolveMachineActionActor_suppliedActiveSession(t *testing.T) {
	mid := id.NewUUIDV7()
	sid := id.NewUUIDV7()
	repo := sessionRepo{sess: domainoperator.Session{
		ID:        sid,
		MachineID: mid,
		Status:    domainoperator.SessionStatusActive,
	}}
	svc := NewService(repo, memMachineRepo{m: fleet.Machine{ID: mid}}, memTechRepo{}, nil)
	got, err := svc.ResolveMachineActionActor(context.Background(), mid, &sid, domainoperator.SessionOptionalByOrigin)
	if err != nil {
		t.Fatal(err)
	}
	if got.Origin != domainoperator.ActionOriginOperatorSession || got.OperatorSessionID == nil || *got.OperatorSessionID != sid {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveMachineActionActor_endedSessionRejected(t *testing.T) {
	mid := id.NewUUIDV7()
	sid := id.NewUUIDV7()
	repo := sessionRepo{sess: domainoperator.Session{
		ID:        sid,
		MachineID: mid,
		Status:    domainoperator.SessionStatusEnded,
	}}
	svc := NewService(repo, memMachineRepo{m: fleet.Machine{ID: mid}}, memTechRepo{}, nil)
	_, err := svc.ResolveMachineActionActor(context.Background(), mid, &sid, domainoperator.SessionOptionalByOrigin)
	if err != domainoperator.ErrSessionNotActive {
		t.Fatalf("got %v", err)
	}
}

func TestResolveMachineActionActor_machineMismatchRejected(t *testing.T) {
	mid := id.NewUUIDV7()
	other := id.NewUUIDV7()
	sid := id.NewUUIDV7()
	repo := sessionRepo{sess: domainoperator.Session{
		ID:        sid,
		MachineID: other,
		Status:    domainoperator.SessionStatusActive,
	}}
	svc := NewService(repo, memMachineRepo{m: fleet.Machine{ID: mid}}, memTechRepo{}, nil)
	_, err := svc.ResolveMachineActionActor(context.Background(), mid, &sid, domainoperator.SessionOptionalByOrigin)
	if err != domainoperator.ErrSessionMachineMismatch {
		t.Fatalf("got %v", err)
	}
}
