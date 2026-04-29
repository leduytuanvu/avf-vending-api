package correctness

import (
	"bytes"
	"context"
	"testing"
	"time"

	appauth "github.com/avf/avf-vending-api/internal/app/auth"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	platformredis "github.com/avf/avf-vending-api/internal/platform/redis"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestP01_AdminAuth_MFAInteractiveEnrollAndLoginChallenge(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	org := testfixtures.DevOrganizationID
	queries := db.New(pool)
	issuer, err := plauth.NewSessionIssuerFromHTTPAuth(config.HTTPAuthConfig{
		JWTSecret:        bytes.Repeat([]byte("p"), 32),
		JWTLeeway:        30 * time.Second,
		ExpectedIssuer:   "p01-iss",
		ExpectedAudience: "p01-aud",
		AccessTokenTTL:   time.Minute,
		RefreshTokenTTL:  time.Hour,
	})
	require.NoError(t, err)
	mfaKey := bytes.Repeat([]byte("m"), 32)
	svc, err := appauth.NewService(appauth.Deps{
		Queries:       queries,
		Issuer:        issuer,
		Pool:          pool,
		LoginFailures: platformredis.NewMemoryLoginFailureCounter(),
		AdminSecurity: config.AdminAuthSecurityConfig{
			LoginMaxFailedAttempts: 5,
			LoginLockoutTTL:        time.Minute,
			MFAEncryptionKey:       mfaKey,
			PasswordMinLength:      10,
			PasswordResetTTL:       time.Minute,
		},
		AppEnv: config.AppEnvDevelopment,
	})
	require.NoError(t, err)

	id := uuid.New()
	email := "p01-mfa-" + id.String()[:8] + "@test.example.com"
	hash, err := bcrypt.GenerateFromPassword([]byte("password12345"), bcrypt.MinCost)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO platform_auth_accounts (id, organization_id, email, password_hash, roles, status)
VALUES ($1,$2,$3,$4,$5,'active')`, id, org, email, string(hash), []string{plauth.RoleOrgAdmin})
	require.NoError(t, err)

	login1, err := svc.Login(ctx, appauth.LoginRequest{OrganizationID: org, Email: email, Password: "password12345"})
	require.NoError(t, err)
	require.NotEmpty(t, login1.Tokens.AccessToken)

	interact := plauth.Principal{
		Subject:        id.String(),
		OrganizationID: org,
		Roles:          []string{plauth.RoleOrgAdmin},
		TokenUse:       plauth.TokenUseInteractiveAccess,
	}
	enroll, err := svc.MFATOTPEnrollBegin(ctx, interact)
	require.NoError(t, err)
	require.NotEmpty(t, enroll.Secret)

	code, err := totp.GenerateCode(enroll.Secret, time.Now().UTC())
	require.NoError(t, err)
	loginPost, err := svc.MFATOTPVerify(ctx, interact, appauth.MFATOTPVerifyRequest{Code: code})
	require.NoError(t, err)
	require.NotEmpty(t, loginPost.Tokens.RefreshToken)

	challengeLogin, err := svc.Login(ctx, appauth.LoginRequest{OrganizationID: org, Email: email, Password: "password12345"})
	require.NoError(t, err)
	require.True(t, challengeLogin.MFARequired)
	require.NotEmpty(t, challengeLogin.MFAChallengeToken)
	claims, err := issuer.ParseMFAPendingJWT(challengeLogin.MFAChallengeToken, plauth.DefaultClockLeeway)
	require.NoError(t, err)
	chalPrincipal := plauth.Principal{
		Subject:        claims.AccountID.String(),
		OrganizationID: claims.OrganizationID,
		TokenUse:       plauth.TokenUseMFAPending,
		MFAEnrollment:  false,
		Roles:          interact.Roles,
	}
	_, err = svc.MFATOTPVerify(ctx, chalPrincipal, appauth.MFATOTPVerifyRequest{Code: "000000"})
	require.ErrorIs(t, err, appauth.ErrInvalidCredentials)
	good, err := totp.GenerateCode(enroll.Secret, time.Now().UTC())
	require.NoError(t, err)
	final, err := svc.MFATOTPVerify(ctx, chalPrincipal, appauth.MFATOTPVerifyRequest{Code: good})
	require.NoError(t, err)
	require.NotEmpty(t, final.Tokens.RefreshToken)

	sessions, err := svc.ListMySessions(ctx, id)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(sessions), 1)

	require.NoError(t, svc.RevokeMySession(ctx, id, uuid.MustParse(sessions[0].SessionID)))
	refreshed, err := svc.Refresh(ctx, appauth.RefreshRequest{RefreshToken: final.Tokens.RefreshToken})
	require.ErrorIs(t, err, appauth.ErrInvalidRefreshToken)
	_ = refreshed
}

func TestP01_AdminAuth_PasswordResetOneTime(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	org := testfixtures.DevOrganizationID
	queries := db.New(pool)
	issuer, err := plauth.NewSessionIssuerFromHTTPAuth(config.HTTPAuthConfig{
		JWTSecret:        bytes.Repeat([]byte("z"), 32),
		JWTLeeway:        30 * time.Second,
		ExpectedIssuer:   "p01b-iss",
		ExpectedAudience: "p01b-aud",
		AccessTokenTTL:   time.Minute,
		RefreshTokenTTL:  time.Hour,
	})
	require.NoError(t, err)
	svc, err := appauth.NewService(appauth.Deps{
		Queries: queries,
		Issuer:  issuer,
		Pool:    pool,
		AdminSecurity: config.AdminAuthSecurityConfig{
			PasswordMinLength: 10,
			PasswordResetTTL:  time.Minute,
			MFAEncryptionKey:  bytes.Repeat([]byte("k"), 32),
		},
		AppEnv: config.AppEnvDevelopment,
	})
	require.NoError(t, err)

	id := uuid.New()
	email := "p01-reset-" + id.String()[:8] + "@test.example.com"
	hash, err := bcrypt.GenerateFromPassword([]byte("password12345"), bcrypt.MinCost)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO platform_auth_accounts (id, organization_id, email, password_hash, roles, status)
VALUES ($1,$2,$3,$4,$5,'active')`, id, org, email, string(hash), []string{"viewer"})
	require.NoError(t, err)

	out, err := svc.RequestPasswordReset(ctx, appauth.PasswordResetRequest{OrganizationID: org, Email: email})
	require.NoError(t, err)
	require.True(t, out.Accepted)
	require.NotEmpty(t, out.ResetToken)
	require.NoError(t, svc.ConfirmPasswordReset(ctx, appauth.PasswordResetConfirmRequest{Token: out.ResetToken, NewPassword: "newpassword123"}))
	err = svc.ConfirmPasswordReset(ctx, appauth.PasswordResetConfirmRequest{Token: out.ResetToken, NewPassword: "otherpass123"})
	require.ErrorIs(t, err, appauth.ErrInvalidResetToken)
}

func TestP01_AdminAuth_LoginLockoutRedis(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	org := testfixtures.DevOrganizationID
	queries := db.New(pool)
	issuer, err := plauth.NewSessionIssuerFromHTTPAuth(config.HTTPAuthConfig{
		JWTSecret:        bytes.Repeat([]byte("l"), 32),
		JWTLeeway:        30 * time.Second,
		ExpectedIssuer:   "p01c-iss",
		ExpectedAudience: "p01c-aud",
		AccessTokenTTL:   time.Minute,
		RefreshTokenTTL:  time.Hour,
	})
	require.NoError(t, err)
	fail := platformredis.NewMemoryLoginFailureCounter()
	svc, err := appauth.NewService(appauth.Deps{
		Queries:       queries,
		Issuer:        issuer,
		Pool:          pool,
		LoginFailures: fail,
		AdminSecurity: config.AdminAuthSecurityConfig{
			LoginMaxFailedAttempts: 2,
			LoginLockoutTTL:        10 * time.Minute,
			PasswordMinLength:      10,
			PasswordResetTTL:       time.Minute,
			MFAEncryptionKey:       bytes.Repeat([]byte("x"), 32),
		},
		AppEnv: config.AppEnvDevelopment,
	})
	require.NoError(t, err)

	id := uuid.New()
	email := "p01-lock-" + id.String()[:8] + "@test.example.com"
	hash, err := bcrypt.GenerateFromPassword([]byte("rightpass123"), bcrypt.MinCost)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO platform_auth_accounts (id, organization_id, email, password_hash, roles, status)
VALUES ($1,$2,$3,$4,$5,'active')`, id, org, email, string(hash), []string{"viewer"})
	require.NoError(t, err)

	_, err = svc.Login(ctx, appauth.LoginRequest{OrganizationID: org, Email: email, Password: "wrong"})
	require.ErrorIs(t, err, appauth.ErrInvalidCredentials)
	_, err = svc.Login(ctx, appauth.LoginRequest{OrganizationID: org, Email: email, Password: "wrong"})
	require.ErrorIs(t, err, appauth.ErrInvalidCredentials)
	_, err = svc.Login(ctx, appauth.LoginRequest{OrganizationID: org, Email: email, Password: "rightpass123"})
	require.ErrorIs(t, err, appauth.ErrInvalidCredentials)
}
