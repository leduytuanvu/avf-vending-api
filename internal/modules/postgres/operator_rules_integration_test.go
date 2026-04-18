package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/operator"
	"github.com/avf/avf-vending-api/internal/domain/device"
	domainoperator "github.com/avf/avf-vending-api/internal/domain/operator"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestOperatorSession_OneActivePerMachine(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := postgres.NewOperatorRepository(pool)
	machines := postgres.NewMachineRepository(pool)
	tech := postgres.NewTechnicianRepository(pool)
	assign := postgres.NewTechnicianAssignmentRepository(pool)
	svc := operator.NewService(repo, machines, tech, assign)

	tid := testfixtures.DevTechnicianID
	sess, err := svc.StartOperatorSession(ctx, operator.StartOperatorSessionInput{
		OrganizationID:    testfixtures.DevOrganizationID,
		MachineID:         testfixtures.DevMachineID,
		ActorType:         domainoperator.ActorTypeTechnician,
		TechnicianID:      &tid,
		InitialAuthMethod: domainoperator.AuthMethodBadge,
	})
	require.NoError(t, err)

	sess2, err := svc.StartOperatorSession(ctx, operator.StartOperatorSessionInput{
		OrganizationID:    testfixtures.DevOrganizationID,
		MachineID:         testfixtures.DevMachineID,
		ActorType:         domainoperator.ActorTypeTechnician,
		TechnicianID:      &tid,
		InitialAuthMethod: domainoperator.AuthMethodBadge,
	})
	require.NoError(t, err)
	require.Equal(t, sess.ID, sess2.ID, "same technician login should resume the ACTIVE session")

	_, err = svc.EndOperatorSession(ctx, operator.EndOperatorSessionInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		SessionID:      sess.ID,
		FinalStatus:    domainoperator.SessionStatusEnded,
	})
	require.NoError(t, err)
}

func TestOperatorSession_EndTwiceSecondIsConflict(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := postgres.NewOperatorRepository(pool)
	machines := postgres.NewMachineRepository(pool)
	tech := postgres.NewTechnicianRepository(pool)
	assign := postgres.NewTechnicianAssignmentRepository(pool)
	svc := operator.NewService(repo, machines, tech, assign)

	tid := testfixtures.DevTechnicianID
	sess, err := svc.StartOperatorSession(ctx, operator.StartOperatorSessionInput{
		OrganizationID:    testfixtures.DevOrganizationID,
		MachineID:         testfixtures.DevMachineID,
		ActorType:         domainoperator.ActorTypeTechnician,
		TechnicianID:      &tid,
		InitialAuthMethod: domainoperator.AuthMethodBadge,
	})
	require.NoError(t, err)

	_, err = svc.EndOperatorSession(ctx, operator.EndOperatorSessionInput{
		OrganizationID:   testfixtures.DevOrganizationID,
		MachineID:        testfixtures.DevMachineID,
		SessionID:        sess.ID,
		FinalStatus:      domainoperator.SessionStatusEnded,
		LogoutAuthMethod: domainoperator.AuthMethodBadge,
	})
	require.NoError(t, err)

	_, err = svc.EndOperatorSession(ctx, operator.EndOperatorSessionInput{
		OrganizationID:   testfixtures.DevOrganizationID,
		MachineID:        testfixtures.DevMachineID,
		SessionID:        sess.ID,
		FinalStatus:      domainoperator.SessionStatusEnded,
		LogoutAuthMethod: domainoperator.AuthMethodBadge,
	})
	require.ErrorIs(t, err, domainoperator.ErrSessionNotActive)
}

func TestOperatorSession_HeartbeatRejectedAfterEnd(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := postgres.NewOperatorRepository(pool)
	machines := postgres.NewMachineRepository(pool)
	tech := postgres.NewTechnicianRepository(pool)
	assign := postgres.NewTechnicianAssignmentRepository(pool)
	svc := operator.NewService(repo, machines, tech, assign)

	tid := testfixtures.DevTechnicianID
	sess, err := svc.StartOperatorSession(ctx, operator.StartOperatorSessionInput{
		OrganizationID:    testfixtures.DevOrganizationID,
		MachineID:         testfixtures.DevMachineID,
		ActorType:         domainoperator.ActorTypeTechnician,
		TechnicianID:      &tid,
		InitialAuthMethod: domainoperator.AuthMethodBadge,
	})
	require.NoError(t, err)

	_, err = svc.EndOperatorSession(ctx, operator.EndOperatorSessionInput{
		OrganizationID:   testfixtures.DevOrganizationID,
		MachineID:        testfixtures.DevMachineID,
		SessionID:        sess.ID,
		FinalStatus:      domainoperator.SessionStatusEnded,
		LogoutAuthMethod: domainoperator.AuthMethodBadge,
	})
	require.NoError(t, err)

	_, err = svc.HeartbeatOperatorSession(ctx, testfixtures.DevOrganizationID, testfixtures.DevMachineID, sess.ID)
	require.ErrorIs(t, err, domainoperator.ErrSessionNotActive)
}

func TestOperatorSession_LoginCreatesAuthEvent_LogoutCreatesAuthEvent(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := postgres.NewOperatorRepository(pool)
	machines := postgres.NewMachineRepository(pool)
	tech := postgres.NewTechnicianRepository(pool)
	assign := postgres.NewTechnicianAssignmentRepository(pool)
	svc := operator.NewService(repo, machines, tech, assign)

	tid := testfixtures.DevTechnicianID
	sess, err := svc.StartOperatorSession(ctx, operator.StartOperatorSessionInput{
		OrganizationID:    testfixtures.DevOrganizationID,
		MachineID:         testfixtures.DevMachineID,
		ActorType:         domainoperator.ActorTypeTechnician,
		TechnicianID:      &tid,
		InitialAuthMethod: domainoperator.AuthMethodBadge,
	})
	require.NoError(t, err)

	var loginCnt int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM machine_operator_auth_events WHERE machine_id = $1 AND event_type = 'login_success'`,
		testfixtures.DevMachineID,
	).Scan(&loginCnt))
	require.GreaterOrEqual(t, loginCnt, 1)

	_, err = svc.EndOperatorSession(ctx, operator.EndOperatorSessionInput{
		OrganizationID:   testfixtures.DevOrganizationID,
		MachineID:        testfixtures.DevMachineID,
		SessionID:        sess.ID,
		FinalStatus:      domainoperator.SessionStatusEnded,
		LogoutAuthMethod: domainoperator.AuthMethodBadge,
	})
	require.NoError(t, err)

	var logoutCnt int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM machine_operator_auth_events WHERE machine_id = $1 AND event_type = 'logout'`,
		testfixtures.DevMachineID,
	).Scan(&logoutCnt))
	require.GreaterOrEqual(t, logoutCnt, 1)
}

func TestOperatorSession_TimeoutWhenExpired(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := postgres.NewOperatorRepository(pool)
	machines := postgres.NewMachineRepository(pool)
	tech := postgres.NewTechnicianRepository(pool)
	assign := postgres.NewTechnicianAssignmentRepository(pool)
	svc := operator.NewService(repo, machines, tech, assign)

	tid := testfixtures.DevTechnicianID
	past := time.Now().UTC().Add(-2 * time.Hour)
	sess, err := svc.StartOperatorSession(ctx, operator.StartOperatorSessionInput{
		OrganizationID:    testfixtures.DevOrganizationID,
		MachineID:         testfixtures.DevMachineID,
		ActorType:         domainoperator.ActorTypeTechnician,
		TechnicianID:      &tid,
		ExpiresAt:         &past,
		InitialAuthMethod: domainoperator.AuthMethodBadge,
	})
	require.NoError(t, err)

	timed, err := svc.TimeoutOperatorSession(ctx, operator.TimeoutOperatorSessionInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		SessionID:      sess.ID,
	})
	require.NoError(t, err)
	require.Equal(t, domainoperator.SessionStatusExpired, timed.Status)
}

func TestOperatorInsightLists_AttributionsForFlows(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	repo := postgres.NewOperatorRepository(pool)
	machines := postgres.NewMachineRepository(pool)
	tech := postgres.NewTechnicianRepository(pool)
	assign := postgres.NewTechnicianAssignmentRepository(pool)
	svc := operator.NewService(repo, machines, tech, assign)

	tid := testfixtures.DevTechnicianID
	sess, err := svc.StartOperatorSession(ctx, operator.StartOperatorSessionInput{
		OrganizationID:    testfixtures.DevOrganizationID,
		MachineID:         testfixtures.DevMachineID,
		ActorType:         domainoperator.ActorTypeTechnician,
		TechnicianID:      &tid,
		InitialAuthMethod: domainoperator.AuthMethodBadge,
	})
	require.NoError(t, err)
	sid := sess.ID

	_, err = store.CreateRefillSessionWithAttribution(ctx, postgres.CreateRefillSessionWithAttributionInput{
		OrganizationID:    testfixtures.DevOrganizationID,
		MachineID:         testfixtures.DevMachineID,
		StartedAt:         time.Now().UTC(),
		Metadata:          []byte(`{}`),
		OperatorSessionID: &sid,
	})
	require.NoError(t, err)

	_, err = store.CreateCashCollectionWithAttribution(ctx, postgres.CreateCashCollectionWithAttributionInput{
		OrganizationID:    testfixtures.DevOrganizationID,
		MachineID:         testfixtures.DevMachineID,
		CollectedAt:       time.Now().UTC(),
		AmountMinor:       100,
		Currency:          "USD",
		Metadata:          []byte(`{}`),
		OperatorSessionID: &sid,
	})
	require.NoError(t, err)

	_, err = store.RecordMachineConfigApplicationWithAttribution(ctx, postgres.RecordMachineConfigApplicationWithAttributionInput{
		OrganizationID:    testfixtures.DevOrganizationID,
		MachineID:         testfixtures.DevMachineID,
		AppliedAt:         time.Now().UTC(),
		ConfigRevision:    1,
		ConfigPayload:     []byte(`{}`),
		Metadata:          []byte(`{}`),
		OperatorSessionID: &sid,
	})
	require.NoError(t, err)

	_, err = store.AppendCommandUpdateShadow(ctx, device.AppendCommandInput{
		MachineID:         testfixtures.DevMachineID,
		CommandType:       "test_op",
		Payload:           []byte(`{}`),
		IdempotencyKey:    "idem-op-" + uuid.NewString(),
		DesiredState:      []byte(`{}`),
		OperatorSessionID: &sid,
	})
	require.NoError(t, err)

	_, err = store.CreateIncidentWithAttribution(ctx, postgres.CreateIncidentWithAttributionInput{
		OrganizationID:    testfixtures.DevOrganizationID,
		MachineID:         testfixtures.DevMachineID,
		Status:            "open",
		Title:             "test",
		OpenedAt:          time.Now().UTC(),
		Metadata:          []byte(`{}`),
		OperatorSessionID: &sid,
	})
	require.NoError(t, err)

	attr, err := svc.ListActionAttributionsForMachine(ctx, testfixtures.DevOrganizationID, testfixtures.DevMachineID, 100)
	require.NoError(t, err)

	found := map[string]bool{}
	for _, a := range attr {
		switch a.ResourceType {
		case "refill_sessions", "cash_collections", "machine_configs", "command_ledger", "incidents":
			found[a.ResourceType] = true
		}
	}
	require.True(t, found["refill_sessions"], "refill attribution")
	require.True(t, found["cash_collections"], "cash attribution")
	require.True(t, found["machine_configs"], "config attribution")
	require.True(t, found["command_ledger"], "command attribution")
	require.True(t, found["incidents"], "incident attribution")

	byTech, err := svc.ListActionAttributionsForTechnician(ctx, testfixtures.DevOrganizationID, tid, 200)
	require.NoError(t, err)
	require.NotEmpty(t, byTech)

	tl, err := svc.BuildMachineOperatorTimeline(ctx, testfixtures.DevOrganizationID, testfixtures.DevMachineID, 50)
	require.NoError(t, err)
	require.NotEmpty(t, tl)

	authEvents, err := svc.ListAuthEventsForMachine(ctx, testfixtures.DevOrganizationID, testfixtures.DevMachineID, 20)
	require.NoError(t, err)
	require.NotEmpty(t, authEvents)

	_, err = svc.EndOperatorSession(ctx, operator.EndOperatorSessionInput{
		OrganizationID:   testfixtures.DevOrganizationID,
		MachineID:        testfixtures.DevMachineID,
		SessionID:        sid,
		FinalStatus:      domainoperator.SessionStatusEnded,
		LogoutAuthMethod: domainoperator.AuthMethodBadge,
	})
	require.NoError(t, err)
}
