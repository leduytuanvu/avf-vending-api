package auth

import (
	"fmt"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/google/uuid"
)

// SessionIssuer signs short-lived HS256 access JWTs for interactive API sessions.
// Refresh tokens are opaque and stored hashed; see NewRefreshToken / HashRefreshToken.
type SessionIssuer struct {
	secret         []byte
	issuer         string
	audience       string
	accessTokenTTL time.Duration
	refreshTTL     time.Duration
}

// NewSessionIssuerFromHTTPAuth builds a session token issuer from HTTP auth configuration.
// Signing material prefers HTTP_AUTH_LOGIN_JWT_SECRET and falls back to HTTP_AUTH_JWT_SECRET.
func NewSessionIssuerFromHTTPAuth(cfg config.HTTPAuthConfig) (*SessionIssuer, error) {
	sec := TrimSecret(cfg.LoginJWTSecret)
	if len(sec) == 0 {
		sec = TrimSecret(cfg.JWTSecret)
	}
	if len(sec) == 0 {
		return nil, fmt.Errorf("auth: session issuer requires HTTP_AUTH_LOGIN_JWT_SECRET or HTTP_AUTH_JWT_SECRET")
	}
	accessTTL := cfg.AccessTokenTTL
	if accessTTL <= 0 {
		accessTTL = 15 * time.Minute
	}
	refreshTTL := cfg.RefreshTokenTTL
	if refreshTTL <= 0 {
		refreshTTL = 30 * 24 * time.Hour
	}
	return &SessionIssuer{
		secret:         sec,
		issuer:         cfg.ExpectedIssuer,
		audience:       cfg.ExpectedAudience,
		accessTokenTTL: accessTTL,
		refreshTTL:     refreshTTL,
	}, nil
}

// AccessTokenTTL returns configured access token lifetime.
func (s *SessionIssuer) AccessTokenTTL() time.Duration {
	if s == nil {
		return 0
	}
	return s.accessTokenTTL
}

// RefreshTokenTTL returns configured refresh token lifetime.
func (s *SessionIssuer) RefreshTokenTTL() time.Duration {
	if s == nil {
		return 0
	}
	return s.refreshTTL
}

type sessionAccessClaims struct {
	Sub      string   `json:"sub"`
	Roles    []string `json:"roles"`
	OrgID    string   `json:"org_id"`
	Iss      string   `json:"iss,omitempty"`
	Aud      string   `json:"aud,omitempty"`
	Iat      int64    `json:"iat"`
	Exp      int64    `json:"exp"`
	TokenUse string   `json:"token_use,omitempty"`
}

// IssueAccessJWT returns a signed HS256 JWT string for the given account subject and tenant claims.
func (s *SessionIssuer) IssueAccessJWT(accountID uuid.UUID, organizationID uuid.UUID, roles []string) (token string, expiresAt time.Time, err error) {
	if s == nil {
		return "", time.Time{}, fmt.Errorf("auth: nil SessionIssuer")
	}
	if accountID == uuid.Nil {
		return "", time.Time{}, fmt.Errorf("auth: nil account id")
	}
	now := time.Now().UTC()
	expiresAt = now.Add(s.accessTokenTTL)
	claims := sessionAccessClaims{
		Sub:      accountID.String(),
		Roles:    roles,
		OrgID:    organizationID.String(),
		Iss:      s.issuer,
		Aud:      s.audience,
		Iat:      now.Unix(),
		Exp:      expiresAt.Unix(),
		TokenUse: "access",
	}
	raw, err := SignHS256JWT(s.secret, claims)
	if err != nil {
		return "", time.Time{}, err
	}
	return raw, expiresAt, nil
}
