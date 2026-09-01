package postgres_test

import (
	"bytes"
	"context"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	appaudit "github.com/avf/avf-vending-api/internal/app/audit"
	appauth "github.com/avf/avf-vending-api/internal/app/auth"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func testAuthServiceWithEnterpriseAudit(t *testing.T, pool *pgxpool.Pool, audit *appaudit.Service, wireMutation bool) *appauth.Service {
	t.Helper()
	queries := db.New(pool)
	issuer, err := plauth.NewSessionIssuerFromHTTPAuth(config.HTTPAuthConfig{
		JWTSecret:        bytes.Repeat([]byte("x"), 32),
		JWTLeeway:        30 * time.Second,
		ExpectedIssuer:   "test-iss",
		ExpectedAudience: "test-aud",
		AccessTokenTTL:   time.Minute,
		RefreshTokenTTL:  time.Hour,
	})
	require.NoError(t, err)
	deps := appauth.Deps{Queries: queries, Issuer: issuer, Pool: pool, EnterpriseAudit: audit}
	if wireMutation {
		deps.OnAdminMutation = api.WireAuthAdminMutationAudit(audit)
	}
	svc, err := appauth.NewService(deps)
	require.NoError(t, err)
	return svc
}

func countAuditByRequestAndAction(t *testing.T, pool *pgxpool.Pool, requestID, action string) int64 {
	t.Helper()
	ctx := context.Background()
	var n int64
	err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE request_id = $1 AND action = $2`, requestID, action).Scan(&n)
	require.NoError(t, err)
	return n
}

func countAuditByActorAndAction(t *testing.T, pool *pgxpool.Pool, actor uuid.UUID, action string) int64 {
	t.Helper()
	ctx := context.Background()
	var n int64
	sub := actor.String()
	err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE actor_id = $1 AND action = $2`, sub, action).Scan(&n)
	require.NoError(t, err)
	return n
}

func TestEnterpriseAudit_LoginSuccessAndFailure(t *testing.T) {
	pool := testPool(t)
	deploymentKey := id.NewUUIDV7()

	audit := appaudit.NewService(pool)
	svc := testAuthServiceWithEnterpriseAudit(t, pool, audit, false)

	uid := id.NewUUIDV7()
	email := insertAuthAccount(t, pool, uid, deploymentKey, "audit-login-"+uid.String()[:8]+"@test.example.com", "password12345", []string{plauth.RoleOrgAdmin}, "active")

	ctx := compliance.WithTransportMeta(context.Background(), compliance.TransportMeta{
		RequestID: "rid-login-audit",
		TraceID:   "corr-login-audit",
		IP:        "203.0.113.9",
	})

	_, err := svc.Login(ctx, appauth.LoginRequest{Email: email, Password: "wrong-password"})
	require.ErrorIs(t, err, appauth.ErrInvalidCredentials)
	require.Equal(t, int64(1), countAuditByRequestAndAction(t, pool, "rid-login-audit", compliance.ActionAuthLoginFailed))
	var failOutcome string
	require.NoError(t, pool.QueryRow(ctx, `SELECT outcome FROM audit_events WHERE request_id = $1 AND action = $2`, "rid-login-audit", compliance.ActionAuthLoginFailed).Scan(&failOutcome))
	require.Equal(t, compliance.OutcomeFailure, failOutcome)

	_, err = svc.Login(ctx, appauth.LoginRequest{Email: email, Password: "password12345"})
	require.NoError(t, err)
	require.Equal(t, int64(1), countAuditByActorAndAction(t, pool, uid, compliance.ActionAuthLoginSuccess))
}

func TestEnterpriseAudit_DisabledLoginIncludesReason(t *testing.T) {
	pool := testPool(t)
	deploymentKey := id.NewUUIDV7()

	audit := appaudit.NewService(pool)
	svc := testAuthServiceWithEnterpriseAudit(t, pool, audit, false)

	id := id.NewUUIDV7()
	email := insertAuthAccount(t, pool, id, deploymentKey, "dis-reason-"+id.String()[:8]+"@test.example.com", "password12345", []string{plauth.RoleOrgAdmin}, "disabled")

	ctx := compliance.WithTransportMeta(context.Background(), compliance.TransportMeta{
		RequestID: "rid-disabled-login",
	})
	_, err := svc.Login(ctx, appauth.LoginRequest{Email: email, Password: "password12345"})
	require.ErrorIs(t, err, appauth.ErrInvalidCredentials)

	var meta []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT metadata FROM audit_events WHERE request_id = $1 AND action = $2 ORDER BY created_at DESC LIMIT 1`, "rid-disabled-login", compliance.ActionAuthLoginFailed).Scan(&meta))
	require.Contains(t, string(meta), "account_disabled")
}

func TestEnterpriseAudit_AdminCreateUser_emitsAuthUserCreated(t *testing.T) {
	pool := testPool(t)
	deploymentKey := id.NewUUIDV7()

	audit := appaudit.NewService(pool)
	svc := testAuthServiceWithEnterpriseAudit(t, pool, audit, true)

	actor := id.NewUUIDV7()
	insertAuthAccount(t, pool, actor, deploymentKey, "actor-create-audit-"+actor.String()[:8]+"@test.example.com", "password12345", []string{plauth.RoleOrgAdmin}, "active")

	_, err := svc.AdminCreateUser(context.Background(), actor, deploymentKey, appauth.AdminCreateUserRequest{
		Username: newTestUsername(),
		Email:    "new-audit-" + uuid.NewString()[:8] + "@test.example.com",
		Password: "password12345",
		Roles:    []string{"viewer"},
		Status:   "active",
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), countAuditByActorAndAction(t, pool, actor, compliance.ActionAuthUserCreated))
}

func TestEnterpriseAudit_AdminPatchRoles_emitsRoleChanged(t *testing.T) {
	pool := testPool(t)
	deploymentKey := id.NewUUIDV7()

	audit := appaudit.NewService(pool)
	svc := testAuthServiceWithEnterpriseAudit(t, pool, audit, true)

	actor := id.NewUUIDV7()
	target := id.NewUUIDV7()
	insertAuthAccount(t, pool, actor, deploymentKey, "audit-actor-"+actor.String()[:8]+"@test.example.com", "password12345", []string{plauth.RoleOrgAdmin}, "active")
	insertAuthAccount(t, pool, target, deploymentKey, "audit-tgt-"+target.String()[:8]+"@test.example.com", "password12345", []string{"viewer"}, "active")

	r := []string{"catalog_manager"}
	_, err := svc.AdminPatchUser(context.Background(), actor, deploymentKey, target, appauth.AdminPatchUserRequest{Roles: &r})
	require.NoError(t, err)
	require.Equal(t, int64(1), countAuditByActorAndAction(t, pool, actor, compliance.ActionRoleChanged))
}

func TestEnterpriseAudit_ListPaginationAndFilters(t *testing.T) {
	pool := testPool(t)

	audit := appaudit.NewService(pool)
	ctx := context.Background()

	a1 := compliance.ActionProductCreated
	a2 := compliance.ActionProductUpdated
	actor := "actor-filter-z"
	r1 := "prod-1"
	r2 := "prod-2"

	require.NoError(t, audit.Record(ctx, compliance.EnterpriseAuditRecord{
		ActorType:    compliance.ActorUser,
		ActorID:      &actor,
		Action:       a1,
		ResourceType: "product",
		ResourceID:   &r1,
		Metadata:     []byte(`{}`),
		Outcome:      compliance.OutcomeSuccess,
	}))
	require.NoError(t, audit.Record(ctx, compliance.EnterpriseAuditRecord{
		ActorType:    compliance.ActorMachine,
		ActorID:      &actor,
		Action:       a2,
		ResourceType: "product",
		ResourceID:   &r2,
		Metadata:     []byte(`{}`),
		Outcome:      compliance.OutcomeFailure,
	}))

	filtered, err := audit.ListEvents(ctx, appaudit.EventListParams{
		Action:       a1,
		ActorID:      actor,
		ResourceType: "product",
		ResourceID:   r1,
		Limit:        10,
		Offset:       0,
	})
	require.NoError(t, err)
	require.Len(t, filtered.Items, 1)
	require.Equal(t, int64(1), filtered.Meta.Total)
	require.Equal(t, a1, filtered.Items[0].Action)

	all, err := audit.ListEvents(ctx, appaudit.EventListParams{Limit: 100, Offset: 0})
	require.NoError(t, err)
	require.GreaterOrEqual(t, all.Meta.Total, int64(2))
	require.GreaterOrEqual(t, len(all.Items), 2)

	paged, err := audit.ListEvents(ctx, appaudit.EventListParams{Limit: 1, Offset: 0, Action: a1})
	require.NoError(t, err)
	require.Len(t, paged.Items, 1)
	require.Equal(t, int64(1), paged.Meta.Total)
}

func TestEnterpriseAudit_ListFiltersByResourceID(t *testing.T) {
	pool := testPool(t)

	audit := appaudit.NewService(pool)
	ctx := context.Background()
	m1 := "m-isolation-a"
	m2 := "m-isolation-b"

	require.NoError(t, audit.Record(ctx, compliance.EnterpriseAuditRecord{
		ActorType:    compliance.ActorSystem,
		Action:       compliance.ActionInventoryAdjusted,
		ResourceType: "machine",
		ResourceID:   &m1,
		Metadata:     []byte(`{}`),
		Outcome:      compliance.OutcomeSuccess,
	}))
	require.NoError(t, audit.Record(ctx, compliance.EnterpriseAuditRecord{
		ActorType:    compliance.ActorSystem,
		Action:       compliance.ActionInventoryAdjusted,
		ResourceType: "machine",
		ResourceID:   &m2,
		Metadata:     []byte(`{}`),
		Outcome:      compliance.OutcomeSuccess,
	}))

	listM1, err := audit.ListEvents(ctx, appaudit.EventListParams{ResourceID: m1, ResourceType: "machine", Limit: 50, Offset: 0})
	require.NoError(t, err)
	require.Len(t, listM1.Items, 1)
	require.Equal(t, m1, *listM1.Items[0].ResourceID)
}

func TestEnterpriseAudit_MachineFilterAndGetByID(t *testing.T) {
	pool := testPool(t)

	audit := appaudit.NewService(pool)
	ctx := context.Background()
	siteID := id.NewUUIDV7()
	machineID := id.NewUUIDV7()
	hw := id.NewUUIDV7()
	_, err := pool.Exec(ctx, `
INSERT INTO machine_hardware_profiles (id, name, spec) VALUES ($1, 'audit-hw', '{}'::jsonb)`,
		hw)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO sites (id, name, code, status) VALUES ($1, 'audit-site', '', 'active')`,
		siteID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, site_id, hardware_profile_id, serial_number, name, status, command_sequence, credential_version)
VALUES ($1, $2, $3, $4, 'm', 'online', 0, 0)`,
		machineID, siteID, hw, "audit-sn-"+machineID.String()[:12])
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM machines WHERE id = $1`, machineID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM sites WHERE id = $1`, siteID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM machine_hardware_profiles WHERE id = $1`, hw)
	})

	resID := "res-machine-audit"
	require.NoError(t, audit.Record(ctx, compliance.EnterpriseAuditRecord{
		ActorType:    compliance.ActorMachine,
		Action:       compliance.ActionMachineUpdated,
		ResourceType: "machine",
		ResourceID:   &resID,
		MachineID:    &machineID,
		SiteID:       &siteID,
		Metadata:     []byte(`{}`),
	}))
	filtered, err := audit.ListEvents(ctx, appaudit.EventListParams{
		MachineID: machineID.String(),
		Limit:     10,
		Offset:    0,
	})
	require.NoError(t, err)
	require.Len(t, filtered.Items, 1)
	require.Equal(t, machineID.String(), *filtered.Items[0].MachineID)

	eventUUID := uuid.MustParse(filtered.Items[0].ID)
	got, err := audit.GetEvent(ctx, eventUUID)
	require.NoError(t, err)
	require.Equal(t, machineID.String(), *got.MachineID)

	_, err = audit.GetEvent(ctx, id.NewUUIDV7())
	require.ErrorIs(t, err, pgx.ErrNoRows)
}

func TestEnterpriseAudit_RedactsSensitiveMetadataKeys(t *testing.T) {
	pool := testPool(t)
	svc := appaudit.NewService(pool)
	ctx := context.Background()
	meta := []byte(`{"refresh_token":"secret-value","ok":true}`)
	require.NoError(t, svc.Record(ctx, compliance.EnterpriseAuditRecord{
		ActorType:    compliance.ActorUser,
		Action:       "test.redaction",
		ResourceType: "fixture",
		Metadata:     meta,
	}))
	rows, err := svc.ListEvents(ctx, appaudit.EventListParams{Limit: 1})
	require.NoError(t, err)
	require.Len(t, rows.Items, 1)
	require.Contains(t, string(rows.Items[0].Metadata), "[REDACTED]")
	require.NotContains(t, string(rows.Items[0].Metadata), "secret-value")
}
