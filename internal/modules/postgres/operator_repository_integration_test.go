package postgres_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/operator"
	domainoperator "github.com/avf/avf-vending-api/internal/domain/operator"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func cleanupActiveOperatorSession(t *testing.T, pool *pgxpool.Pool, machineID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	repo := postgres.NewOperatorRepository(pool)
	sess, err := repo.GetActiveSessionByMachineID(ctx, machineID)
	if errors.Is(err, domainoperator.ErrNoActiveSession) {
		return
	}
	require.NoError(t, err)
	_, err = repo.EndOperatorSession(ctx, domainoperator.EndOperatorSessionParams{
		SessionID: sess.ID,
		Status:    domainoperator.SessionStatusEnded,
		EndedAt:   time.Now().UTC(),
	})
	require.NoError(t, err)
}

func TestOperatorRepository_EndWithLogoutIsAtomic(t *testing.T) {
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

	ended, err := repo.EndOperatorSession(ctx, domainoperator.EndOperatorSessionParams{
		SessionID: sess.ID,
		Status:    domainoperator.SessionStatusEnded,
		EndedAt:   time.Now().UTC(),
		Logout: &domainoperator.InsertAuthEventParams{
			EventType:  domainoperator.AuthEventLogout,
			AuthMethod: domainoperator.AuthMethodBadge,
		},
	})
	require.NoError(t, err)
	require.Equal(t, domainoperator.SessionStatusEnded, ended.Status)

	var logoutCnt int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM machine_operator_auth_events WHERE operator_session_id = $1 AND event_type = 'logout'`,
		sess.ID,
	).Scan(&logoutCnt))
	require.Equal(t, 1, logoutCnt)
}

func TestOperatorRepository_GetOperatorSessionByID_wrappedNotFound(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := postgres.NewOperatorRepository(pool)

	_, err := repo.GetOperatorSessionByID(ctx, uuid.New())
	require.ErrorIs(t, err, domainoperator.ErrSessionNotFound)
}

func TestOperatorRepository_GetActiveSession_wrappedNoActive(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := postgres.NewOperatorRepository(pool)

	_, err := repo.GetActiveSessionByMachineID(ctx, uuid.New())
	require.ErrorIs(t, err, domainoperator.ErrNoActiveSession)
}

func TestOperatorRepository_EndSecondTimeNotActive(t *testing.T) {
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

	_, err = repo.EndOperatorSession(ctx, domainoperator.EndOperatorSessionParams{
		SessionID: sess.ID,
		Status:    domainoperator.SessionStatusEnded,
		EndedAt:   time.Now().UTC(),
	})
	require.NoError(t, err)

	_, err = repo.EndOperatorSession(ctx, domainoperator.EndOperatorSessionParams{
		SessionID: sess.ID,
		Status:    domainoperator.SessionStatusEnded,
		EndedAt:   time.Now().UTC(),
	})
	require.ErrorIs(t, err, domainoperator.ErrSessionNotActive)
}

func TestOperatorRepository_ListActionAttributionsByMachineAndResource(t *testing.T) {
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
	sid := sess.ID

	rid := uuid.NewString()
	_, err = repo.InsertActionAttribution(ctx, domainoperator.InsertActionAttributionParams{
		OperatorSessionID: &sid,
		MachineID:         testfixtures.DevMachineID,
		ActionOriginType:  domainoperator.ActionOriginOperatorSession,
		ResourceType:      "test_resource",
		ResourceID:        rid,
		OccurredAt:        nil,
		Metadata:          []byte(`{}`),
	})
	require.NoError(t, err)
	_, err = repo.InsertActionAttribution(ctx, domainoperator.InsertActionAttributionParams{
		OperatorSessionID: &sid,
		MachineID:         testfixtures.DevMachineID,
		ActionOriginType:  domainoperator.ActionOriginOperatorSession,
		ResourceType:      "test_resource",
		ResourceID:        rid,
		OccurredAt:        nil,
		Metadata:          []byte(`{}`),
	})
	require.NoError(t, err)

	filtered, err := repo.ListActionAttributionsByMachineAndResource(ctx, testfixtures.DevMachineID, "test_resource", rid, 10)
	require.NoError(t, err)
	require.Len(t, filtered, 2)
	require.False(t, filtered[1].OccurredAt.After(filtered[0].OccurredAt), "expected occurred_at DESC")

	_, err = repo.EndOperatorSession(ctx, domainoperator.EndOperatorSessionParams{
		SessionID: sid,
		Status:    domainoperator.SessionStatusEnded,
		EndedAt:   time.Now().UTC(),
	})
	require.NoError(t, err)
}

func TestOperatorRepository_SecondActiveSessionRejected(t *testing.T) {
	pool := testPool(t)
	cleanupActiveOperatorSession(t, pool, testfixtures.DevMachineID)
	ctx := context.Background()
	repo := postgres.NewOperatorRepository(pool)
	machines := postgres.NewMachineRepository(pool)
	tech := postgres.NewTechnicianRepository(pool)
	assign := postgres.NewTechnicianAssignmentRepository(pool)
	svc := operator.NewService(repo, machines, tech, assign)

	tid := testfixtures.DevTechnicianID
	_, err := svc.StartOperatorSession(ctx, operator.StartOperatorSessionInput{
		OrganizationID:    testfixtures.DevOrganizationID,
		MachineID:         testfixtures.DevMachineID,
		ActorType:         domainoperator.ActorTypeTechnician,
		TechnicianID:      &tid,
		InitialAuthMethod: domainoperator.AuthMethodBadge,
	})
	require.NoError(t, err)

	_, err = svc.StartOperatorSession(ctx, operator.StartOperatorSessionInput{
		OrganizationID:    testfixtures.DevOrganizationID,
		MachineID:         testfixtures.DevMachineID,
		ActorType:         domainoperator.ActorTypeUser,
		UserPrincipal:     strPtr("second-login-" + uuid.NewString()),
		InitialAuthMethod: domainoperator.AuthMethodOIDC,
	})
	require.ErrorIs(t, err, domainoperator.ErrActiveSessionExists)
	cleanupActiveOperatorSession(t, pool, testfixtures.DevMachineID)
}

func TestOperatorRepository_ConcurrentStartAtMostOneActive(t *testing.T) {
	pool := testPool(t)
	cleanupActiveOperatorSession(t, pool, testfixtures.DevMachineID)
	ctx := context.Background()
	repo := postgres.NewOperatorRepository(pool)
	machines := postgres.NewMachineRepository(pool)
	tech := postgres.NewTechnicianRepository(pool)
	assign := postgres.NewTechnicianAssignmentRepository(pool)
	svc := operator.NewService(repo, machines, tech, assign)

	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := 0; i < 8; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			principal := "conc-user-" + uuid.NewString()
			_, errs[i] = svc.StartOperatorSession(ctx, operator.StartOperatorSessionInput{
				OrganizationID:    testfixtures.DevOrganizationID,
				MachineID:         testfixtures.DevMachineID,
				ActorType:         domainoperator.ActorTypeUser,
				UserPrincipal:     &principal,
				InitialAuthMethod: domainoperator.AuthMethodOIDC,
			})
		}()
	}
	wg.Wait()
	var successes, conflicts int
	for _, err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, domainoperator.ErrActiveSessionExists):
			conflicts++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	require.Equal(t, 1, successes, "exactly one concurrent login should win")
	require.Equal(t, 7, conflicts)
	cleanupActiveOperatorSession(t, pool, testfixtures.DevMachineID)
}

func TestOperatorRepository_TimeoutExpiredSessionMarksExpired(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := postgres.NewOperatorRepository(pool)
	machines := postgres.NewMachineRepository(pool)
	tech := postgres.NewTechnicianRepository(pool)
	assign := postgres.NewTechnicianAssignmentRepository(pool)
	svc := operator.NewService(repo, machines, tech, assign)

	future := time.Now().UTC().Add(2 * time.Hour)
	principal := "expiry-user-" + uuid.NewString()
	sess, err := svc.StartOperatorSession(ctx, operator.StartOperatorSessionInput{
		OrganizationID:    testfixtures.DevOrganizationID,
		MachineID:         testfixtures.DevMachineID,
		ActorType:         domainoperator.ActorTypeUser,
		UserPrincipal:     &principal,
		ExpiresAt:         &future,
		InitialAuthMethod: domainoperator.AuthMethodOIDC,
	})
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `UPDATE machine_operator_sessions SET expires_at = now() - interval '1 second' WHERE id = $1`, sess.ID)
	require.NoError(t, err)

	out, err := svc.TimeoutOperatorSession(ctx, operator.TimeoutOperatorSessionInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		SessionID:      sess.ID,
	})
	require.NoError(t, err)
	require.Equal(t, domainoperator.SessionStatusExpired, out.Status)

	_, err = repo.EndOperatorSession(ctx, domainoperator.EndOperatorSessionParams{
		SessionID: sess.ID,
		Status:    domainoperator.SessionStatusEnded,
		EndedAt:   time.Now().UTC(),
	})
	require.ErrorIs(t, err, domainoperator.ErrSessionNotActive)
}

func strPtr(s string) *string { return &s }

func TestOperatorRepository_TimeoutNotApplicable(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := postgres.NewOperatorRepository(pool)
	machines := postgres.NewMachineRepository(pool)
	tech := postgres.NewTechnicianRepository(pool)
	assign := postgres.NewTechnicianAssignmentRepository(pool)
	svc := operator.NewService(repo, machines, tech, assign)

	tid := testfixtures.DevTechnicianID
	future := time.Now().UTC().Add(24 * time.Hour)
	sess, err := svc.StartOperatorSession(ctx, operator.StartOperatorSessionInput{
		OrganizationID:    testfixtures.DevOrganizationID,
		MachineID:         testfixtures.DevMachineID,
		ActorType:         domainoperator.ActorTypeTechnician,
		TechnicianID:      &tid,
		ExpiresAt:         &future,
		InitialAuthMethod: domainoperator.AuthMethodBadge,
	})
	require.NoError(t, err)

	_, err = repo.TimeoutOperatorSessionIfExpired(ctx, sess.ID)
	require.ErrorIs(t, err, domainoperator.ErrTimeoutNotApplicable)
}

func TestOperatorRepository_StaleSessionReclaimedByDifferentUser(t *testing.T) {
	pool := testPool(t)
	cleanupActiveOperatorSession(t, pool, testfixtures.DevMachineID)
	ctx := context.Background()
	repo := postgres.NewOperatorRepository(pool)
	machines := postgres.NewMachineRepository(pool)
	tech := postgres.NewTechnicianRepository(pool)
	assign := postgres.NewTechnicianAssignmentRepository(pool)
	svc := operator.NewService(repo, machines, tech, assign)

	pa := "stale-a-" + uuid.NewString()
	pb := "stale-b-" + uuid.NewString()
	sessA, err := svc.StartOperatorSession(ctx, operator.StartOperatorSessionInput{
		OrganizationID:    testfixtures.DevOrganizationID,
		MachineID:         testfixtures.DevMachineID,
		ActorType:         domainoperator.ActorTypeUser,
		UserPrincipal:     &pa,
		InitialAuthMethod: domainoperator.AuthMethodOIDC,
	})
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `UPDATE machine_operator_sessions SET last_activity_at = now() - interval '20 minutes' WHERE id = $1`, sessA.ID)
	require.NoError(t, err)

	sessB, err := svc.StartOperatorSession(ctx, operator.StartOperatorSessionInput{
		OrganizationID:    testfixtures.DevOrganizationID,
		MachineID:         testfixtures.DevMachineID,
		ActorType:         domainoperator.ActorTypeUser,
		UserPrincipal:     &pb,
		InitialAuthMethod: domainoperator.AuthMethodOIDC,
	})
	require.NoError(t, err)
	require.NotEqual(t, sessA.ID, sessB.ID)

	var reason string
	require.NoError(t, pool.QueryRow(ctx, `SELECT ended_reason FROM machine_operator_sessions WHERE id = $1`, sessA.ID).Scan(&reason))
	require.Equal(t, domainoperator.EndedReasonStaleSessionReclaimed, reason)

	cleanupActiveOperatorSession(t, pool, testfixtures.DevMachineID)
}

func TestOperatorRepository_AdminTakeoverRevokesPriorSession(t *testing.T) {
	pool := testPool(t)
	cleanupActiveOperatorSession(t, pool, testfixtures.DevMachineID)
	ctx := context.Background()
	repo := postgres.NewOperatorRepository(pool)
	machines := postgres.NewMachineRepository(pool)
	tech := postgres.NewTechnicianRepository(pool)
	assign := postgres.NewTechnicianAssignmentRepository(pool)
	svc := operator.NewService(repo, machines, tech, assign)

	pa := "takeover-a-" + uuid.NewString()
	pb := "takeover-b-" + uuid.NewString()
	sessA, err := svc.StartOperatorSession(ctx, operator.StartOperatorSessionInput{
		OrganizationID:    testfixtures.DevOrganizationID,
		MachineID:         testfixtures.DevMachineID,
		ActorType:         domainoperator.ActorTypeUser,
		UserPrincipal:     &pa,
		InitialAuthMethod: domainoperator.AuthMethodOIDC,
	})
	require.NoError(t, err)

	sessB, err := svc.StartOperatorSession(ctx, operator.StartOperatorSessionInput{
		OrganizationID:          testfixtures.DevOrganizationID,
		MachineID:               testfixtures.DevMachineID,
		ActorType:               domainoperator.ActorTypeUser,
		UserPrincipal:           &pb,
		InitialAuthMethod:       domainoperator.AuthMethodOIDC,
		ForceAdminTakeover:      true,
		AdminTakeoverAuthorized: true,
	})
	require.NoError(t, err)
	require.NotEqual(t, sessA.ID, sessB.ID)

	var status, reason string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status, ended_reason FROM machine_operator_sessions WHERE id = $1`, sessA.ID).Scan(&status, &reason))
	require.Equal(t, domainoperator.SessionStatusRevoked, status)
	require.Equal(t, domainoperator.EndedReasonAdminForcedTakeover, reason)

	cleanupActiveOperatorSession(t, pool, testfixtures.DevMachineID)
}

func TestOperatorRepository_FreshSessionConflictDifferentUser(t *testing.T) {
	pool := testPool(t)
	cleanupActiveOperatorSession(t, pool, testfixtures.DevMachineID)
	ctx := context.Background()
	repo := postgres.NewOperatorRepository(pool)
	machines := postgres.NewMachineRepository(pool)
	tech := postgres.NewTechnicianRepository(pool)
	assign := postgres.NewTechnicianAssignmentRepository(pool)
	svc := operator.NewService(repo, machines, tech, assign)

	pa := "fresh-a-" + uuid.NewString()
	pb := "fresh-b-" + uuid.NewString()
	_, err := svc.StartOperatorSession(ctx, operator.StartOperatorSessionInput{
		OrganizationID:    testfixtures.DevOrganizationID,
		MachineID:         testfixtures.DevMachineID,
		ActorType:         domainoperator.ActorTypeUser,
		UserPrincipal:     &pa,
		InitialAuthMethod: domainoperator.AuthMethodOIDC,
	})
	require.NoError(t, err)

	_, err = svc.StartOperatorSession(ctx, operator.StartOperatorSessionInput{
		OrganizationID:    testfixtures.DevOrganizationID,
		MachineID:         testfixtures.DevMachineID,
		ActorType:         domainoperator.ActorTypeUser,
		UserPrincipal:     &pb,
		InitialAuthMethod: domainoperator.AuthMethodOIDC,
	})
	require.ErrorIs(t, err, domainoperator.ErrActiveSessionExists)

	cleanupActiveOperatorSession(t, pool, testfixtures.DevMachineID)
}

func TestOperatorRepository_SameUserResumeRecordsSessionRefresh(t *testing.T) {
	pool := testPool(t)
	cleanupActiveOperatorSession(t, pool, testfixtures.DevMachineID)
	ctx := context.Background()
	repo := postgres.NewOperatorRepository(pool)
	machines := postgres.NewMachineRepository(pool)
	tech := postgres.NewTechnicianRepository(pool)
	assign := postgres.NewTechnicianAssignmentRepository(pool)
	svc := operator.NewService(repo, machines, tech, assign)

	pa := "resume-a-" + uuid.NewString()
	sess1, err := svc.StartOperatorSession(ctx, operator.StartOperatorSessionInput{
		OrganizationID:    testfixtures.DevOrganizationID,
		MachineID:         testfixtures.DevMachineID,
		ActorType:         domainoperator.ActorTypeUser,
		UserPrincipal:     &pa,
		InitialAuthMethod: domainoperator.AuthMethodOIDC,
	})
	require.NoError(t, err)

	sess2, err := svc.StartOperatorSession(ctx, operator.StartOperatorSessionInput{
		OrganizationID:    testfixtures.DevOrganizationID,
		MachineID:         testfixtures.DevMachineID,
		ActorType:         domainoperator.ActorTypeUser,
		UserPrincipal:     &pa,
		InitialAuthMethod: domainoperator.AuthMethodOIDC,
	})
	require.NoError(t, err)
	require.Equal(t, sess1.ID, sess2.ID)

	var refreshCnt int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM machine_operator_auth_events WHERE operator_session_id = $1 AND event_type = 'session_refresh'`,
		sess1.ID,
	).Scan(&refreshCnt))
	require.GreaterOrEqual(t, refreshCnt, 1)

	cleanupActiveOperatorSession(t, pool, testfixtures.DevMachineID)
}

func TestOperatorRepository_EndOperatorSession_revokedForcedClose(t *testing.T) {
	pool := testPool(t)
	cleanupActiveOperatorSession(t, pool, testfixtures.DevMachineID)
	ctx := context.Background()
	repo := postgres.NewOperatorRepository(pool)
	machines := postgres.NewMachineRepository(pool)
	tech := postgres.NewTechnicianRepository(pool)
	assign := postgres.NewTechnicianAssignmentRepository(pool)
	svc := operator.NewService(repo, machines, tech, assign)

	pa := "revoke-a-" + uuid.NewString()
	sess, err := svc.StartOperatorSession(ctx, operator.StartOperatorSessionInput{
		OrganizationID:    testfixtures.DevOrganizationID,
		MachineID:         testfixtures.DevMachineID,
		ActorType:         domainoperator.ActorTypeUser,
		UserPrincipal:     &pa,
		InitialAuthMethod: domainoperator.AuthMethodOIDC,
	})
	require.NoError(t, err)

	reason := "admin_remote_revoke"
	ended, err := repo.EndOperatorSession(ctx, domainoperator.EndOperatorSessionParams{
		SessionID:   sess.ID,
		Status:      domainoperator.SessionStatusRevoked,
		EndedAt:     time.Now().UTC(),
		EndedReason: &reason,
	})
	require.NoError(t, err)
	require.Equal(t, domainoperator.SessionStatusRevoked, ended.Status)
	require.NotNil(t, ended.EndedReason)
	require.Equal(t, reason, *ended.EndedReason)
}
