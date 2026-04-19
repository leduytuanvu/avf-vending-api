package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

// Deps wires persistence and token issuance for interactive sessions.
type Deps struct {
	Queries *db.Queries
	Issuer  *plauth.SessionIssuer
}

// Service implements POST /v1/auth/login, refresh, me, logout workflows.
type Service struct {
	q *db.Queries
	i *plauth.SessionIssuer
}

// NewService returns a session auth service.
func NewService(d Deps) (*Service, error) {
	if d.Queries == nil {
		return nil, fmt.Errorf("auth service: nil Queries")
	}
	if d.Issuer == nil {
		return nil, fmt.Errorf("auth service: nil Issuer")
	}
	return &Service{q: d.Queries, i: d.Issuer}, nil
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
	Roles          []string    `json:"roles"`
	Tokens         TokenPair   `json:"tokens"`
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
	acct, err := s.q.AuthGetAccountByOrgEmail(ctx, db.AuthGetAccountByOrgEmailParams{
		OrganizationID: req.OrganizationID,
		Lower:          email,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(acct.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	return s.issueLoginResponse(ctx, acct)
}

func (s *Service) issueLoginResponse(ctx context.Context, acct db.PlatformAuthAccount) (*LoginResponse, error) {
	at, accessExp, err := s.i.IssueAccessJWT(acct.ID, acct.OrganizationID, acct.Roles)
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
	}); err != nil {
		return nil, err
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
	if err := s.q.AuthRevokeRefreshToken(ctx, row.ID); err != nil {
		return nil, err
	}
	at, accessExp, err := s.i.IssueAccessJWT(acct.ID, acct.OrganizationID, acct.Roles)
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
	}); err != nil {
		return nil, err
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

// Me loads the authenticated account row for the access token subject (account id).
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
func (s *Service) Logout(ctx context.Context, accountID uuid.UUID, req LogoutRequest) error {
	if s == nil {
		return errors.New("auth service: nil")
	}
	if accountID == uuid.Nil {
		return ErrInvalidRequest
	}
	if req.RevokeAll {
		return s.q.AuthRevokeAllRefreshForAccount(ctx, accountID)
	}
	if strings.TrimSpace(req.RefreshToken) == "" {
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
	return s.q.AuthRevokeRefreshToken(ctx, row.ID)
}
