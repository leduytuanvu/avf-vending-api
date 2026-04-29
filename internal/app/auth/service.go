package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/auth/revocation"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// Deps wires persistence and token issuance for interactive sessions.
type Deps struct {
	Queries *db.Queries
	Issuer  *plauth.SessionIssuer
	// Pool is optional; when set, password mutations run in a transaction with refresh-token revocation.
	Pool *pgxpool.Pool
	// OnAdminMutation is optional; invoked after successful admin auth user mutations for audit integration (prefer tx non-nil).
	OnAdminMutation func(context.Context, pgx.Tx, AuthAdminMutationEvent) error
	// EnterpriseAudit optional P1.4 audit_events sink (login/logout and related paths use fail-closed where enforced).
	EnterpriseAudit compliance.EnterpriseRecorder
	// AccessRevocation optional Redis-backed access-token JTI / subject revocation.
	AccessRevocation revocation.Store
	SessionCache     plauth.RefreshSessionCache
	LoginFailures    plauth.LoginFailureCounter
	// AdminSecurity configures MFA policy, lockout thresholds, password rules, and reset TTL (required; defaults applied in NewService).
	AdminSecurity config.AdminAuthSecurityConfig
	AppEnv        config.AppEnvironment
}

// AuthAdminMutationEvent describes an admin API auth user change for optional audit sinks.
type AuthAdminMutationEvent struct {
	Action          string
	OrganizationID  uuid.UUID
	ActorAccountID  uuid.UUID
	TargetAccountID uuid.UUID
	Details         map[string]any
}

// Service implements POST /v1/auth/login, refresh, me, logout workflows.
type Service struct {
	q                *db.Queries
	i                *plauth.SessionIssuer
	pool             *pgxpool.Pool
	onAdminMutation  func(context.Context, pgx.Tx, AuthAdminMutationEvent) error
	enterpriseAudit  compliance.EnterpriseRecorder
	accessRevocation revocation.Store
	sessionCache     plauth.RefreshSessionCache
	loginFailures    plauth.LoginFailureCounter
	adminSec         config.AdminAuthSecurityConfig
	appEnv           config.AppEnvironment
}

// NewService returns a session auth service.
func NewService(d Deps) (*Service, error) {
	if d.Queries == nil {
		return nil, fmt.Errorf("auth service: nil Queries")
	}
	if d.Issuer == nil {
		return nil, fmt.Errorf("auth service: nil Issuer")
	}
	sec := d.AdminSecurity
	if sec.LoginMaxFailedAttempts <= 0 {
		sec.LoginMaxFailedAttempts = 5
	}
	if sec.LoginLockoutTTL <= 0 {
		sec.LoginLockoutTTL = 15 * time.Minute
	}
	if sec.PasswordMinLength <= 0 {
		sec.PasswordMinLength = 10
	}
	if sec.PasswordResetTTL <= 0 {
		sec.PasswordResetTTL = 15 * time.Minute
	}
	env := d.AppEnv
	if env == "" {
		env = config.AppEnvDevelopment
	}
	return &Service{
		q:                d.Queries,
		i:                d.Issuer,
		pool:             d.Pool,
		onAdminMutation:  d.OnAdminMutation,
		enterpriseAudit:  d.EnterpriseAudit,
		accessRevocation: d.AccessRevocation,
		sessionCache:     d.SessionCache,
		loginFailures:    d.LoginFailures,
		adminSec:         sec,
		appEnv:           env,
	}, nil
}

// LoginRequest is the JSON body for POST /v1/auth/login.
type LoginRequest struct {
	OrganizationID uuid.UUID `json:"organizationId"`
	Email          string    `json:"email"`
	Password       string    `json:"password"`
}

// TokenPair is returned on login and refresh.
type TokenPair struct {
	AccessToken      string    `json:"accessToken"`
	AccessExpiresAt  time.Time `json:"accessExpiresAt"`
	RefreshToken     string    `json:"refreshToken"`
	RefreshExpiresAt time.Time `json:"refreshExpiresAt"`
	TokenType        string    `json:"tokenType"`
}

// LoginResponse is returned from POST /v1/auth/login.
type LoginResponse struct {
	AccountID      uuid.UUID `json:"accountId"`
	OrganizationID uuid.UUID `json:"organizationId"`
	Email          string    `json:"email"`
	Roles          []string  `json:"roles"`
	Tokens         TokenPair `json:"tokens"`

	MFARequired           bool       `json:"mfaRequired,omitempty"`
	MFAEnrollmentRequired bool       `json:"mfaEnrollmentRequired,omitempty"`
	MFAChallengeToken     string     `json:"mfaChallengeToken,omitempty"`
	MFAExpiresAt          *time.Time `json:"mfaExpiresAt,omitempty"`
}

// RefreshRequest is the JSON body for POST /v1/auth/refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// RefreshResponse is returned from POST /v1/auth/refresh.
type RefreshResponse struct {
	Tokens TokenPair `json:"tokens"`
}

// MeResponse is returned from GET /v1/auth/me.
type MeResponse struct {
	AccountID      uuid.UUID `json:"accountId"`
	OrganizationID uuid.UUID `json:"organizationId"`
	Email          string    `json:"email"`
	Roles          []string  `json:"roles"`
}

// LogoutRequest is the JSON body for POST /v1/auth/logout (optional fields).
type LogoutRequest struct {
	RefreshToken string `json:"refreshToken"`
	RevokeAll    bool   `json:"revokeAll"`
}

// Login authenticates a platform account and issues access + refresh tokens.
func (s *Service) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	if s == nil {
		return nil, errors.New("auth service: nil")
	}
	email := strings.TrimSpace(strings.ToLower(req.Email))
	if req.OrganizationID == uuid.Nil || email == "" || req.Password == "" {
		return nil, ErrInvalidRequest
	}
	acct, err := s.q.AuthLookupAccountByOrgEmailAnyStatus(ctx, db.AuthLookupAccountByOrgEmailAnyStatusParams{
		OrganizationID: req.OrganizationID,
		Lower:          email,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if s.loginFailures != nil {
				_, _, _ = s.loginFailures.IncrementFailure(ctx, req.OrganizationID, email, s.adminSec.LoginMaxFailedAttempts, s.adminSec.LoginLockoutTTL)
			}
			s.auditLoginFailure(ctx, req.OrganizationID, email, "")
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if acct.LockedUntil.Valid && time.Now().UTC().Before(acct.LockedUntil.Time) {
		s.auditLoginFailure(ctx, req.OrganizationID, email, "account_locked")
		return nil, ErrInvalidCredentials
	}
	if strings.EqualFold(strings.TrimSpace(acct.Status), "locked") {
		if acct.LockedUntil.Valid && !time.Now().UTC().Before(acct.LockedUntil.Time) {
			_ = s.q.AuthClearExpiredLock(ctx, acct.ID)
			acct.Status = "active"
		}
	}
	if strings.ToLower(strings.TrimSpace(acct.Status)) != "active" {
		s.auditLoginFailure(ctx, req.OrganizationID, email, "account_disabled")
		return nil, ErrInvalidCredentials
	}
	if s.loginFailures != nil {
		n, err := s.loginFailures.PeekFailureCount(ctx, req.OrganizationID, email)
		if err != nil {
			return nil, err
		}
		if int32(n) >= s.adminSec.LoginMaxFailedAttempts {
			s.auditLoginFailure(ctx, req.OrganizationID, email, "account_locked")
			return nil, ErrInvalidCredentials
		}
	}
	if err := bcrypt.CompareHashAndPassword([]byte(acct.PasswordHash), []byte(req.Password)); err != nil {
		if s.loginFailures != nil {
			_, _, _ = s.loginFailures.IncrementFailure(ctx, req.OrganizationID, email, s.adminSec.LoginMaxFailedAttempts, s.adminSec.LoginLockoutTTL)
		}
		_ = s.q.AuthRecordLoginFailure(ctx, db.AuthRecordLoginFailureParams{
			ID:               acct.ID,
			FailedLoginCount: s.adminSec.LoginMaxFailedAttempts,
			Column3:          int64(s.adminSec.LoginLockoutTTL / time.Second),
		})
		s.auditLoginFailure(ctx, req.OrganizationID, email, "")
		return nil, ErrInvalidCredentials
	}

	mfaActive, err := s.q.AuthAdminMFACountActiveForUser(ctx, acct.ID)
	if err != nil {
		return nil, err
	}
	requireEnrollment := s.appEnv == config.AppEnvProduction && s.adminSec.MFARequiredInProduction && mfaActive == 0
	if requireEnrollment || mfaActive > 0 {
		enrollmentJWT := requireEnrollment && mfaActive == 0
		challenge, exp, err := s.i.IssueMFAPendingJWT(acct.ID, acct.OrganizationID, acct.Roles, acct.Status, enrollmentJWT)
		if err != nil {
			return nil, err
		}
		return &LoginResponse{
			AccountID:             acct.ID,
			OrganizationID:        acct.OrganizationID,
			Email:                 acct.Email,
			Roles:                 acct.Roles,
			MFARequired:           true,
			MFAEnrollmentRequired: enrollmentJWT,
			MFAChallengeToken:     challenge,
			MFAExpiresAt:          &exp,
			Tokens:                TokenPair{},
		}, nil
	}

	_ = s.q.AuthRecordLoginSuccess(ctx, acct.ID)
	if s.loginFailures != nil {
		_ = s.loginFailures.ClearFailures(ctx, req.OrganizationID, email)
	}
	out, err := s.issueLoginResponse(ctx, acct)
	if err != nil {
		return nil, err
	}
	if err := s.auditLoginSuccess(ctx, acct); err != nil {
		_ = s.q.AuthRevokeAllRefreshForAccount(ctx, acct.ID)
		return nil, fmt.Errorf("audit: %w", err)
	}
	return out, nil
}

func (s *Service) issueLoginResponse(ctx context.Context, acct db.PlatformAuthAccount) (*LoginResponse, error) {
	at, accessExp, err := s.i.IssueAccessJWT(acct.ID, acct.OrganizationID, acct.Roles, acct.Status)
	if err != nil {
		return nil, err
	}
	rt, rtHash, err := plauth.NewRefreshToken()
	if err != nil {
		return nil, err
	}
	rtID := uuid.New()
	rtExp := time.Now().UTC().Add(s.i.RefreshTokenTTL())
	meta := compliance.TransportMetaFromContext(ctx)
	ip := strings.TrimSpace(meta.IP)
	ua := strings.TrimSpace(meta.UserAgent)
	if err := s.q.AuthInsertRefreshToken(ctx, db.AuthInsertRefreshTokenParams{
		ID:        rtID,
		AccountID: acct.ID,
		TokenHash: rtHash,
		ExpiresAt: rtExp,
		IpAddress: optionalMetaString(ip),
		UserAgent: optionalMetaString(ua),
	}); err != nil {
		return nil, err
	}
	sessID := uuid.New()
	_ = s.q.AuthAdminInsertAdminSession(ctx, db.AuthAdminInsertAdminSessionParams{
		ID:               sessID,
		OrganizationID:   acct.OrganizationID,
		UserID:           acct.ID,
		RefreshTokenID:   rtID,
		RefreshTokenHash: rtHash,
		IpAddress:        optionalMetaString(ip),
		UserAgent:        optionalMetaString(ua),
		ExpiresAt:        rtExp,
	})
	if s.sessionCache != nil {
		_ = s.sessionCache.PutRefreshSession(ctx, rtHash, acct.ID, rtExp)
	}
	return &LoginResponse{
		AccountID:      acct.ID,
		OrganizationID: acct.OrganizationID,
		Email:          acct.Email,
		Roles:          acct.Roles,
		Tokens: TokenPair{
			AccessToken:      at,
			AccessExpiresAt:  accessExp,
			RefreshToken:     rt,
			RefreshExpiresAt: rtExp,
			TokenType:        "Bearer",
		},
	}, nil
}

// Refresh rotates refresh token and issues a new access token.
func (s *Service) Refresh(ctx context.Context, req RefreshRequest) (*RefreshResponse, error) {
	if s == nil {
		return nil, errors.New("auth service: nil")
	}
	if strings.TrimSpace(req.RefreshToken) == "" {
		return nil, ErrInvalidRequest
	}
	hash := plauth.HashRefreshToken(req.RefreshToken)
	if s.sessionCache != nil && s.sessionCache.IsRefreshRevoked(ctx, hash) {
		return nil, ErrInvalidRefreshToken
	}
	row, err := s.q.AuthGetRefreshTokenByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidRefreshToken
		}
		return nil, err
	}
	acct, err := s.q.AuthGetAccountByID(ctx, row.AccountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidRefreshToken
		}
		return nil, err
	}
	if acct.LockedUntil.Valid && time.Now().UTC().Before(acct.LockedUntil.Time) {
		return nil, ErrInvalidRefreshToken
	}
	at, accessExp, err := s.i.IssueAccessJWT(acct.ID, acct.OrganizationID, acct.Roles, acct.Status)
	if err != nil {
		return nil, err
	}
	rt, rtHash, err := plauth.NewRefreshToken()
	if err != nil {
		return nil, err
	}
	rtID := uuid.New()
	rtExp := time.Now().UTC().Add(s.i.RefreshTokenTTL())
	if err := s.q.AuthInsertRefreshToken(ctx, db.AuthInsertRefreshTokenParams{
		ID:        rtID,
		AccountID: acct.ID,
		TokenHash: rtHash,
		ExpiresAt: rtExp,
		IpAddress: pgtype.Text{},
		UserAgent: pgtype.Text{},
	}); err != nil {
		return nil, err
	}
	_ = s.q.AuthAdminRotateSessionRefreshToken(ctx, db.AuthAdminRotateSessionRefreshTokenParams{
		UserID:           acct.ID,
		RefreshTokenID:   row.ID,
		RefreshTokenID_2: rtID,
		RefreshTokenHash: rtHash,
		ExpiresAt:        rtExp,
	})
	if err := s.q.AuthRevokeRefreshToken(ctx, row.ID); err != nil {
		_ = s.q.AuthRevokeRefreshToken(ctx, rtID)
		return nil, err
	}
	if s.sessionCache != nil {
		_ = s.sessionCache.InvalidateRefreshSession(ctx, hash)
		_ = s.sessionCache.PutRefreshSession(ctx, rtHash, acct.ID, rtExp)
	}
	if err := s.auditRefreshSuccess(ctx, acct); err != nil {
		_ = s.q.AuthRevokeRefreshToken(ctx, rtID)
		return nil, fmt.Errorf("audit: %w", err)
	}
	return &RefreshResponse{
		Tokens: TokenPair{
			AccessToken:      at,
			AccessExpiresAt:  accessExp,
			RefreshToken:     rt,
			RefreshExpiresAt: rtExp,
			TokenType:        "Bearer",
		},
	}, nil
}

func (s *Service) Me(ctx context.Context, accountID uuid.UUID) (*MeResponse, error) {
	if s == nil {
		return nil, errors.New("auth service: nil")
	}
	if accountID == uuid.Nil {
		return nil, ErrInvalidRequest
	}
	acct, err := s.q.AuthGetAccountByID(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	return &MeResponse{
		AccountID:      acct.ID,
		OrganizationID: acct.OrganizationID,
		Email:          acct.Email,
		Roles:          acct.Roles,
	}, nil
}

// Logout revokes refresh token(s) for the caller.
func (s *Service) Logout(ctx context.Context, accountID uuid.UUID, accessJTI string, accessExpiresAt time.Time, req LogoutRequest) error {
	if s == nil {
		return errors.New("auth service: nil")
	}
	if accountID == uuid.Nil {
		return ErrInvalidRequest
	}
	if req.RevokeAll {
		acct, err := s.q.AuthGetAccountByID(ctx, accountID)
		if err != nil {
			return err
		}
		if err := s.q.AuthRevokeAllRefreshForAccount(ctx, accountID); err != nil {
			return err
		}
		if err := s.q.AuthAdminRevokeAllAdminSessionsForUser(ctx, db.AuthAdminRevokeAllAdminSessionsForUserParams{
			OrganizationID: acct.OrganizationID,
			UserID:         accountID,
		}); err != nil {
			return err
		}
		if s.sessionCache != nil {
			_ = s.sessionCache.InvalidateAccountSessions(ctx, accountID)
		}
		if s.accessRevocation != nil {
			ttl := s.i.AccessTokenTTL()
			if ttl > 0 {
				_ = s.accessRevocation.RevokeSubject(ctx, accountID.String(), ttl)
			}
		}
		return s.auditLogout(ctx, accountID)
	}
	if strings.TrimSpace(req.RefreshToken) == "" {
		if s.accessRevocation != nil && strings.TrimSpace(accessJTI) != "" {
			ttl := time.Until(accessExpiresAt)
			if ttl > 0 {
				_ = s.accessRevocation.RevokeJTI(ctx, accessJTI, ttl)
			}
		}
		return nil
	}
	hash := plauth.HashRefreshToken(req.RefreshToken)
	row, err := s.q.AuthGetRefreshTokenByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if row.AccountID != accountID {
		return nil
	}
	if err := s.q.AuthRevokeRefreshToken(ctx, row.ID); err != nil {
		return err
	}
	if _, err := s.q.AuthAdminRevokeAdminSessionByRefreshTokenID(ctx, db.AuthAdminRevokeAdminSessionByRefreshTokenIDParams{
		RefreshTokenID: row.ID,
		UserID:         accountID,
	}); err != nil {
		return err
	}
	if s.sessionCache != nil {
		_ = s.sessionCache.InvalidateRefreshSession(ctx, hash)
	}
	if s.accessRevocation != nil && strings.TrimSpace(accessJTI) != "" {
		ttl := time.Until(accessExpiresAt)
		if ttl > 0 {
			_ = s.accessRevocation.RevokeJTI(ctx, accessJTI, ttl)
		}
	}
	return s.auditLogout(ctx, accountID)
}

func optionalMetaString(s string) pgtype.Text {
	s = strings.TrimSpace(s)
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}
