package postgres_test

import (
	"bytes"
	"context"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"strings"
	"testing"
	"time"

	appauth "github.com/avf/avf-vending-api/internal/app/auth"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func testAuthServiceWithPool(t *testing.T, pool *pgxpool.Pool) *appauth.Service {
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
	svc, err := appauth.NewService(appauth.Deps{Queries: queries, Issuer: issuer, Pool: pool})
	require.NoError(t, err)
	return svc
}

func testAuthUsername(accountID uuid.UUID) string {
	return "u" + strings.ReplaceAll(accountID.String()[:8], "-", "")
}

func insertAuthAccount(t *testing.T, pool *pgxpool.Pool, id uuid.UUID, _ uuid.UUID, email string, password string, roles []string, status string) string {
	t.Helper()
	if !strings.HasPrefix(email, "auth-regression-") {
		local, domain, ok := strings.Cut(email, "@")
		if !ok {
			local = email
			domain = "test.example.com"
		}
		email = "auth-regression-" + local + "@" + domain
	}
	username := testAuthUsername(id)
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	ctx := context.Background()
	_, err = pool.Exec(ctx, `
INSERT INTO platform_auth_accounts (id, username, email, password_hash, roles, status)
VALUES ($1,$2,$3,$4,$5,$6)
`, id, username, email, string(hash), roles, status)
	require.NoError(t, err)
	return email
}

func newTestUsername() string {
	return "usr" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
}

func authTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := testPool(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `DELETE FROM platform_auth_accounts WHERE email LIKE '%@test.example.com'`)
	require.NoError(t, err)
	return pool
}

func TestAuthAdmin_CreateUserAndDuplicateEmail(t *testing.T) {
	pool := authTestPool(t)
	svc := testAuthServiceWithPool(t, pool)
	ctx := context.Background()
	org := testfixtures.DevCompanyID

	actor := id.NewUUIDV7()
	insertAuthAccount(t, pool, actor, org, "actor-create-"+actor.String()[:8]+"@test.example.com", "password12345", []string{plauth.RoleOrgAdmin}, "active")

	email := "newuser-" + uuid.NewString()[:8] + "@test.example.com"
	username := newTestUsername()
	created, err := svc.AdminCreateUser(ctx, actor, org, appauth.AdminCreateUserRequest{
		Username: username,
		Email:    email,
		Password: "password12345",
		Roles:    []string{"viewer"},
		Status:   "active",
	})
	require.NoError(t, err)
	require.NotNil(t, created)

	_, err = svc.AdminCreateUser(ctx, actor, org, appauth.AdminCreateUserRequest{
		Username: username,
		Email:    email,
		Password: "password12345",
		Roles:    []string{"viewer"},
		Status:   "active",
	})
	require.ErrorIs(t, err, appauth.ErrConflictDuplicateEmail)
}

func TestAuthAdmin_InvalidRole(t *testing.T) {
	pool := authTestPool(t)
	svc := testAuthServiceWithPool(t, pool)
	ctx := context.Background()
	org := testfixtures.DevCompanyID

	actor := id.NewUUIDV7()
	insertAuthAccount(t, pool, actor, org, "actor-badrole-"+actor.String()[:8]+"@test.example.com", "password12345", []string{plauth.RoleOrgAdmin}, "active")

	_, err := svc.AdminCreateUser(ctx, actor, org, appauth.AdminCreateUserRequest{
		Username: newTestUsername(),
		Email:    "bad-" + uuid.NewString()[:8] + "@test.example.com",
		Password: "password12345",
		Roles:    []string{"not_a_real_role"},
		Status:   "active",
	})
	require.ErrorIs(t, err, appauth.ErrInvalidRole)
}

func TestAuthAdmin_MachineRoleRejected(t *testing.T) {
	pool := authTestPool(t)
	svc := testAuthServiceWithPool(t, pool)
	ctx := context.Background()
	org := testfixtures.DevCompanyID

	actor := id.NewUUIDV7()
	insertAuthAccount(t, pool, actor, org, "actor-machine-role-"+actor.String()[:8]+"@test.example.com", "password12345", []string{plauth.RoleOrgAdmin}, "active")

	_, err := svc.AdminCreateUser(ctx, actor, org, appauth.AdminCreateUserRequest{
		Username: newTestUsername(),
		Email:    "machine-role-" + uuid.NewString()[:8] + "@test.example.com",
		Password: "password12345",
		Roles:    []string{plauth.RoleMachine},
		Status:   "active",
	})
	require.ErrorIs(t, err, appauth.ErrInvalidRole)
}

func TestAuthAdmin_ActivateDeactivateAndLastOrgAdmin(t *testing.T) {
	pool := authTestPool(t)
	svc := testAuthServiceWithPool(t, pool)
	ctx := context.Background()
	org := testfixtures.DevCompanyID
	actor := id.NewUUIDV7()
	insertAuthAccount(t, pool, actor, org, "solo-admin-"+actor.String()[:8]+"@test.example.com", "password12345", []string{plauth.RoleOrgAdmin}, "active")

	created, err := svc.AdminCreateUser(ctx, actor, org, appauth.AdminCreateUserRequest{
		Username: newTestUsername(),
		Email:    "viewer-user-" + uuid.NewString()[:8] + "@test.example.com",
		Password: "password12345",
		Roles:    []string{"viewer"},
		Status:   "disabled",
	})
	require.NoError(t, err)
	memberID, err := uuid.Parse(created.AccountID)
	require.NoError(t, err)

	out, err := svc.AdminActivateUser(ctx, actor, org, memberID)
	require.NoError(t, err)
	require.Equal(t, "active", out.Status)

	_, err = svc.AdminDeactivateUser(ctx, actor, org, actor)
	require.ErrorIs(t, err, appauth.ErrForbiddenLastOrgAdmin)

	backup := id.NewUUIDV7()
	insertAuthAccount(t, pool, backup, org, "backup-admin-"+backup.String()[:8]+"@test.example.com", "password12345", []string{plauth.RoleOrgAdmin}, "active")

	_, err = svc.AdminDeactivateUser(ctx, actor, org, actor)
	require.NoError(t, err)

	me, err := svc.AdminGetUser(ctx, org, actor)
	require.NoError(t, err)
	require.Equal(t, "disabled", me.Status)
}

func TestAuthAdmin_ResetPasswordAndLoginAndDisabledNoLogin(t *testing.T) {
	pool := authTestPool(t)
	svc := testAuthServiceWithPool(t, pool)
	ctx := context.Background()
	org := testfixtures.DevCompanyID

	actor := id.NewUUIDV7()
	insertAuthAccount(t, pool, actor, org, "actor-login-"+actor.String()[:8]+"@test.example.com", "password12345", []string{plauth.RoleOrgAdmin}, "active")

	email := "login-subj-" + uuid.NewString()[:8] + "@test.example.com"
	loginUsername := newTestUsername()
	created, err := svc.AdminCreateUser(ctx, actor, org, appauth.AdminCreateUserRequest{
		Username: loginUsername,
		Email:    email,
		Password: "originalpwd12",
		Roles:    []string{"viewer"},
		Status:   "active",
	})
	require.NoError(t, err)
	uid, err := uuid.Parse(created.AccountID)
	require.NoError(t, err)

	_, err = svc.AdminResetPassword(ctx, actor, org, uid, appauth.ResetPasswordRequest{Password: "newpassword12"})
	require.NoError(t, err)

	login, err := svc.Login(ctx, appauth.LoginRequest{
		Username: loginUsername,
		Password: "newpassword12",
	})
	require.NoError(t, err)
	require.NotEmpty(t, login.Tokens.AccessToken)

	_, err = svc.AdminDeactivateUser(ctx, actor, org, uid)
	require.NoError(t, err)

	_, err = svc.Login(ctx, appauth.LoginRequest{
		Username: loginUsername,
		Password: "newpassword12",
	})
	require.ErrorIs(t, err, appauth.ErrInvalidCredentials)
}

func TestAuthAdmin_PatchRemovesLastOrgAdminForbidden(t *testing.T) {
	pool := authTestPool(t)
	svc := testAuthServiceWithPool(t, pool)
	ctx := context.Background()
	org := testfixtures.DevCompanyID
	actor := id.NewUUIDV7()
	insertAuthAccount(t, pool, actor, org, "patch-last-"+actor.String()[:8]+"@test.example.com", "password12345", []string{plauth.RoleOrgAdmin}, "active")

	roles := []string{"viewer"}
	_, err := svc.AdminPatchUser(ctx, actor, org, actor, appauth.AdminPatchUserRequest{Roles: &roles})
	require.ErrorIs(t, err, appauth.ErrForbiddenLastOrgAdmin)
}

func TestAuthAdmin_ChangePassword(t *testing.T) {
	pool := authTestPool(t)
	svc := testAuthServiceWithPool(t, pool)
	ctx := context.Background()
	org := testfixtures.DevCompanyID

	id := id.NewUUIDV7()
	email := insertAuthAccount(t, pool, id, org, "self-"+id.String()[:8]+"@test.example.com", "oldpassword12", []string{"viewer"}, "active")

	err := svc.ChangePassword(ctx, id, appauth.ChangePasswordRequest{
		CurrentPassword: "oldpassword12",
		NewPassword:     "newpassword12",
	})
	require.NoError(t, err)

	_, err = svc.Login(ctx, appauth.LoginRequest{
		Email:    email,
		Password: "newpassword12",
	})
	require.NoError(t, err)
}

func TestAuthAdmin_ReplaceUserRoles(t *testing.T) {
	pool := authTestPool(t)
	svc := testAuthServiceWithPool(t, pool)
	ctx := context.Background()
	org := testfixtures.DevCompanyID

	actor := id.NewUUIDV7()
	insertAuthAccount(t, pool, actor, org, "actor-roles-"+actor.String()[:8]+"@test.example.com", "password12345", []string{plauth.RoleOrgAdmin}, "active")

	target := id.NewUUIDV7()
	insertAuthAccount(t, pool, target, org, "target-roles-"+target.String()[:8]+"@test.example.com", "password12345", []string{"viewer"}, "active")

	out, err := svc.AdminReplaceUserRoles(ctx, actor, org, target, []string{"catalog_manager", "viewer"})
	require.NoError(t, err)
	require.Contains(t, out.Roles, "catalog_manager")
	require.Contains(t, out.Roles, "viewer")
}

func TestAuthAdmin_CrossCompanyGetUserDenied(t *testing.T) {
	pool := authTestPool(t)
	svc := testAuthServiceWithPool(t, pool)
	ctx := context.Background()
	orgA := testfixtures.DevCompanyID
	orgB := id.NewUUIDV7()

	target := id.NewUUIDV7()
	insertAuthAccount(t, pool, target, orgA, "company-a-"+target.String()[:8]+"@test.example.com", "password12345", []string{"viewer"}, "active")

	_, err := svc.AdminGetUser(ctx, orgB, target)
	require.ErrorIs(t, err, appauth.ErrAccountNotFound)
}

func TestAuthAdmin_DisabledUserCannotLogin(t *testing.T) {
	pool := authTestPool(t)
	svc := testAuthServiceWithPool(t, pool)
	ctx := context.Background()
	org := testfixtures.DevCompanyID

	id := id.NewUUIDV7()
	email := insertAuthAccount(t, pool, id, org, "disabled-"+id.String()[:8]+"@test.example.com", "password12345", []string{"viewer"}, "disabled")

	_, err := svc.Login(ctx, appauth.LoginRequest{
		Email:    email,
		Password: "password12345",
	})
	require.ErrorIs(t, err, appauth.ErrInvalidCredentials)
}

func TestAuthAdmin_RevokeSessionsInvalidatesRefreshToken(t *testing.T) {
	pool := authTestPool(t)
	svc := testAuthServiceWithPool(t, pool)
	ctx := context.Background()
	org := testfixtures.DevCompanyID

	actor := id.NewUUIDV7()
	insertAuthAccount(t, pool, actor, org, "actor-revoke-"+actor.String()[:8]+"@test.example.com", "password12345", []string{plauth.RoleOrgAdmin}, "active")

	target := id.NewUUIDV7()
	email := insertAuthAccount(t, pool, target, org, "target-revoke-"+target.String()[:8]+"@test.example.com", "password12345", []string{"viewer"}, "active")

	login, err := svc.Login(ctx, appauth.LoginRequest{Email: email, Password: "password12345"})
	require.NoError(t, err)
	require.NotEmpty(t, login.Tokens.RefreshToken)

	require.NoError(t, svc.AdminRevokeUserSessions(ctx, actor, org, target))

	_, err = svc.Refresh(ctx, appauth.RefreshRequest{RefreshToken: login.Tokens.RefreshToken})
	require.ErrorIs(t, err, appauth.ErrInvalidRefreshToken)
}

func TestAuthAdmin_PasswordResetTokenOneTime(t *testing.T) {
	pool := authTestPool(t)
	svc := testAuthServiceWithPool(t, pool)
	ctx := context.Background()
	org := testfixtures.DevCompanyID

	id := id.NewUUIDV7()
	email := insertAuthAccount(t, pool, id, org, "reset-"+id.String()[:8]+"@test.example.com", "password12345", []string{"viewer"}, "active")

	issued, err := svc.RequestPasswordReset(ctx, appauth.PasswordResetRequest{Email: email})
	require.NoError(t, err)
	require.True(t, issued.Accepted)
	require.NotEmpty(t, issued.ResetToken)

	require.NoError(t, svc.ConfirmPasswordReset(ctx, appauth.PasswordResetConfirmRequest{
		Token:       issued.ResetToken,
		NewPassword: "newpassword12",
	}))
	require.ErrorIs(t, svc.ConfirmPasswordReset(ctx, appauth.PasswordResetConfirmRequest{
		Token:       issued.ResetToken,
		NewPassword: "anotherpass12",
	}), appauth.ErrInvalidResetToken)

	_, err = svc.Login(ctx, appauth.LoginRequest{Email: email, Password: "newpassword12"})
	require.NoError(t, err)
}
