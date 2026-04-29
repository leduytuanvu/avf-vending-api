package postgres_test

import (
	"bytes"
	"context"
	"strings"
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

func insertAuditOrganization(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	slug := "audit-" + strings.ReplaceAll(id.String(), "-", "")
	_, err := pool.Exec(ctx, `
INSERT INTO organizations (id, name, slug, status)
VALUES ($1, $2, $3, 'active')
`, id, "Enterprise audit test org", slug)
	require.NoError(t, err)
}

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

func countAuditEvents(t *testing.T, pool *pgxpool.Pool, org uuid.UUID, action string) int64 {
	t.Helper()
	ctx := context.Background()
	var n int64
	var err error
	if action == "" {
		err = pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE organization_id = $1`, org).Scan(&n)
	} else {
		err = pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE organization_id = $1 AND action = $2`, org, action).Scan(&n)
	}
	require.NoError(t, err)
	return n
}

func TestEnterpriseAudit_LoginSuccessAndFailure(t *testing.T) {
	pool := testPool(t)
	org := uuid.New()
	insertAuditOrganization(t, pool, org)

	audit := appaudit.NewService(pool)
	svc := testAuthServiceWithEnterpriseAudit(t, pool, audit, false)

	uid := uuid.New()
	email := "audit-login-" + uid.String()[:8] + "@test.example.com"
	insertAuthAccount(t, pool, uid, org, email, "password12345", []string{plauth.RoleOrgAdmin}, "active")

	ctx := compliance.WithTransportMeta(context.Background(), compliance.TransportMeta{
		RequestID: "rid-login-audit",
		TraceID:   "corr-login-audit",
		IP:        "203.0.113.9",
	})

	_, err := svc.Login(ctx, appauth.LoginRequest{OrganizationID: org, Email: email, Password: "wrong-password"})
	require.ErrorIs(t, err, appauth.ErrInvalidCredentials)
	require.Equal(t, int64(1), countAuditEvents(t, pool, org, compliance.ActionAuthLoginFailed))
	var failOutcome string
	require.NoError(t, pool.QueryRow(ctx, `SELECT outcome FROM audit_events WHERE organization_id = $1 AND action = $2`, org, compliance.ActionAuthLoginFailed).Scan(&failOutcome))
	require.Equal(t, compliance.OutcomeFailure, failOutcome)

	_, err = svc.Login(ctx, appauth.LoginRequest{OrganizationID: org, Email: email, Password: "password12345"})
	require.NoError(t, err)
	require.Equal(t, int64(1), countAuditEvents(t, pool, org, compliance.ActionAuthLoginSuccess))
}

func TestEnterpriseAudit_DisabledLoginIncludesReason(t *testing.T) {
	pool := testPool(t)
	org := uuid.New()
	insertAuditOrganization(t, pool, org)

	audit := appaudit.NewService(pool)
	svc := testAuthServiceWithEnterpriseAudit(t, pool, audit, false)

	id := uuid.New()
	email := "dis-reason-" + id.String()[:8] + "@test.example.com"
	insertAuthAccount(t, pool, id, org, email, "password12345", []string{plauth.RoleOrgAdmin}, "disabled")

	ctx := context.Background()
	_, err := svc.Login(ctx, appauth.LoginRequest{OrganizationID: org, Email: email, Password: "password12345"})
	require.ErrorIs(t, err, appauth.ErrInvalidCredentials)

	var meta []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT metadata FROM audit_events WHERE organization_id = $1 AND action = $2 ORDER BY created_at DESC LIMIT 1`, org, compliance.ActionAuthLoginFailed).Scan(&meta))
	require.Contains(t, string(meta), "account_disabled")
}

func TestEnterpriseAudit_AdminCreateUser_emitsAuthUserCreated(t *testing.T) {
	pool := testPool(t)
	org := uuid.New()
	insertAuditOrganization(t, pool, org)

	audit := appaudit.NewService(pool)
	svc := testAuthServiceWithEnterpriseAudit(t, pool, audit, true)

	actor := uuid.New()
	insertAuthAccount(t, pool, actor, org, "actor-create-audit-"+actor.String()[:8]+"@test.example.com", "password12345", []string{plauth.RoleOrgAdmin}, "active")

	_, err := svc.AdminCreateUser(context.Background(), actor, org, appauth.AdminCreateUserRequest{
		Email:    "new-audit-" + uuid.NewString()[:8] + "@test.example.com",
		Password: "password12345",
		Roles:    []string{"viewer"},
		Status:   "active",
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), countAuditEvents(t, pool, org, compliance.ActionAuthUserCreated))
}

func TestEnterpriseAudit_AdminPatchRoles_emitsRoleChanged(t *testing.T) {
	pool := testPool(t)
	org := uuid.New()
	insertAuditOrganization(t, pool, org)

	audit := appaudit.NewService(pool)
	svc := testAuthServiceWithEnterpriseAudit(t, pool, audit, true)

	actor := uuid.New()
	target := uuid.New()
	insertAuthAccount(t, pool, actor, org, "audit-actor-"+actor.String()[:8]+"@test.example.com", "password12345", []string{plauth.RoleOrgAdmin}, "active")
	insertAuthAccount(t, pool, target, org, "audit-tgt-"+target.String()[:8]+"@test.example.com", "password12345", []string{"viewer"}, "active")

	r := []string{"catalog_manager"}
	_, err := svc.AdminPatchUser(context.Background(), actor, org, target, appauth.AdminPatchUserRequest{Roles: &r})
	require.NoError(t, err)
	require.Equal(t, int64(1), countAuditEvents(t, pool, org, compliance.ActionRoleChanged))
}

func TestEnterpriseAudit_ListPaginationAndFilters(t *testing.T) {
	pool := testPool(t)
	org := uuid.New()
	insertAuditOrganization(t, pool, org)

	audit := appaudit.NewService(pool)
	ctx := context.Background()

	a1 := compliance.ActionProductCreated
	a2 := compliance.ActionProductUpdated
	actor := "actor-filter-z"
	r1 := "prod-1"
	r2 := "prod-2"

	require.NoError(t, audit.Record(ctx, compliance.EnterpriseAuditRecord{
		OrganizationID: org,
		ActorType:      compliance.ActorUser,
		ActorID:        &actor,
		Action:         a1,
		ResourceType:   "product",
		ResourceID:     &r1,
		Metadata:       []byte(`{}`),
		Outcome:        compliance.OutcomeSuccess,
	}))
	require.NoError(t, audit.Record(ctx, compliance.EnterpriseAuditRecord{
		OrganizationID: org,
		ActorType:      compliance.ActorMachine,
		ActorID:        &actor,
		Action:         a2,
		ResourceType:   "product",
		ResourceID:     &r2,
		Metadata:       []byte(`{}`),
		Outcome:        compliance.OutcomeFailure,
	}))

	filtered, err := audit.ListEvents(ctx, appaudit.EventListParams{
		OrganizationID: org,
		Action:         a1,
		ActorID:        actor,
		ResourceType:   "product",
		ResourceID:     r1,
		Limit:          10,
		Offset:         0,
	})
	require.NoError(t, err)
	require.Len(t, filtered.Items, 1)
	require.Equal(t, int64(1), filtered.Meta.Total)
	require.Equal(t, a1, filtered.Items[0].Action)

	all, err := audit.ListEvents(ctx, appaudit.EventListParams{OrganizationID: org, Limit: 10, Offset: 0})
	require.NoError(t, err)
	require.Len(t, all.Items, 2)
	require.Equal(t, int64(2), all.Meta.Total)

	paged, err := audit.ListEvents(ctx, appaudit.EventListParams{OrganizationID: org, Limit: 1, Offset: 1})
	require.NoError(t, err)
	require.Len(t, paged.Items, 1)
	require.Equal(t, int64(2), paged.Meta.Total)
}

func TestEnterpriseAudit_TenantIsolation(t *testing.T) {
	pool := testPool(t)
	orgA := uuid.New()
	orgB := uuid.New()
	insertAuditOrganization(t, pool, orgA)
	insertAuditOrganization(t, pool, orgB)

	audit := appaudit.NewService(pool)
	ctx := context.Background()
	m1 := "m-isolation-a"
	m2 := "m-isolation-b"

	require.NoError(t, audit.Record(ctx, compliance.EnterpriseAuditRecord{
		OrganizationID: orgA,
		ActorType:      compliance.ActorSystem,
		Action:         compliance.ActionInventoryAdjusted,
		ResourceType:   "machine",
		ResourceID:     &m1,
		Metadata:       []byte(`{}`),
		Outcome:        compliance.OutcomeSuccess,
	}))
	require.NoError(t, audit.Record(ctx, compliance.EnterpriseAuditRecord{
		OrganizationID: orgB,
		ActorType:      compliance.ActorSystem,
		Action:         compliance.ActionInventoryAdjusted,
		ResourceType:   "machine",
		ResourceID:     &m2,
		Metadata:       []byte(`{}`),
		Outcome:        compliance.OutcomeSuccess,
	}))

	listA, err := audit.ListEvents(ctx, appaudit.EventListParams{OrganizationID: orgA, Limit: 50, Offset: 0})
	require.NoError(t, err)
	require.Len(t, listA.Items, 1)
	require.Equal(t, orgA.String(), listA.Items[0].OrganizationID)
}

func TestEnterpriseAudit_MachineFilterAndGetByID(t *testing.T) {
	pool := testPool(t)
	org := uuid.New()
	insertAuditOrganization(t, pool, org)

	audit := appaudit.NewService(pool)
	ctx := context.Background()
	machineID := uuid.New()
	siteID := uuid.New()
	rid := "res-machine-scope"
	require.NoError(t, audit.Record(ctx, compliance.EnterpriseAuditRecord{
		OrganizationID: org,
		ActorType:      compliance.ActorMachine,
		Action:         compliance.ActionMachineUpdated,
		ResourceType:   "machine",
		ResourceID:     &rid,
		MachineID:      &machineID,
		SiteID:         &siteID,
		Metadata:       []byte(`{}`),
	}))
	filtered, err := audit.ListEvents(ctx, appaudit.EventListParams{
		OrganizationID: org,
		MachineID:      machineID.String(),
		Limit:          10,
		Offset:         0,
	})
	require.NoError(t, err)
	require.Len(t, filtered.Items, 1)
	require.Equal(t, machineID.String(), *filtered.Items[0].MachineID)

	eventUUID := uuid.MustParse(filtered.Items[0].ID)
	got, err := audit.GetEventForOrg(ctx, org, eventUUID)
	require.NoError(t, err)
	require.Equal(t, machineID.String(), *got.MachineID)

	_, err = audit.GetEventForOrg(ctx, org, uuid.New())
	require.ErrorIs(t, err, pgx.ErrNoRows)

	orgOther := uuid.New()
	insertAuditOrganization(t, pool, orgOther)
	_, err = audit.GetEventForOrg(ctx, orgOther, eventUUID)
	require.ErrorIs(t, err, pgx.ErrNoRows)
}

func TestEnterpriseAudit_RedactsSensitiveMetadataKeys(t *testing.T) {
	pool := testPool(t)
	org := uuid.New()
	insertAuditOrganization(t, pool, org)
	svc := appaudit.NewService(pool)
	ctx := context.Background()
	meta := []byte(`{"refresh_token":"secret-value","ok":true}`)
	require.NoError(t, svc.Record(ctx, compliance.EnterpriseAuditRecord{
		OrganizationID: org,
		ActorType:      compliance.ActorUser,
		Action:         "test.redaction",
		ResourceType:   "fixture",
		Metadata:       meta,
	}))
	rows, err := svc.ListEvents(ctx, appaudit.EventListParams{OrganizationID: org, Limit: 1})
	require.NoError(t, err)
	require.Len(t, rows.Items, 1)
	require.Contains(t, string(rows.Items[0].Metadata), "[REDACTED]")
	require.NotContains(t, string(rows.Items[0].Metadata), "secret-value")
}
