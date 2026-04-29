package auth

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/google/uuid"
)

const (
	// TokenUseInteractiveAccess is the normal interactive admin API session JWT.
	TokenUseInteractiveAccess = "access"
	// TokenUseMFAPending is issued after password verification until TOTP completes (or MFA enrollment step).
	TokenUseMFAPending = "mfa_pending"
)

// SessionIssuer signs short-lived HS256 access JWTs for interactive API sessions.
// Refresh tokens are opaque and stored hashed; see NewRefreshToken / HashRefreshToken.
type SessionIssuer struct {
	secret            []byte
	issuer            string
	audience          string
	accessTokenTTL    time.Duration
	refreshTTL        time.Duration
	mfaPendingTTL     time.Duration
	machineIssuer     string
	machineAudience   string
	machineAccessTTL  time.Duration
	machineRefreshTTL time.Duration
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
	mfaTTL := cfg.MFAPendingTTL
	if mfaTTL <= 0 {
		mfaTTL = 5 * time.Minute
	}
	return &SessionIssuer{
		secret:            sec,
		issuer:            cfg.ExpectedIssuer,
		audience:          cfg.ExpectedAudience,
		accessTokenTTL:    accessTTL,
		refreshTTL:        refreshTTL,
		mfaPendingTTL:     mfaTTL,
		machineIssuer:     cfg.ExpectedIssuer,
		machineAudience:   AudienceMachineGRPC,
		machineAccessTTL:  accessTTL,
		machineRefreshTTL: refreshTTL,
	}, nil
}

// ConfigureMachineTokens applies dedicated machine-token policy without changing
// admin session token policy.
func (s *SessionIssuer) ConfigureMachineTokens(cfg config.MachineJWTConfig) {
	if s == nil {
		return
	}
	if issuer := strings.TrimSpace(cfg.ExpectedIssuer); issuer != "" {
		s.machineIssuer = issuer
	}
	if audience := strings.TrimSpace(cfg.ExpectedAudience); audience != "" {
		s.machineAudience = audience
	}
	if cfg.AccessTokenTTL > 0 {
		s.machineAccessTTL = cfg.AccessTokenTTL
	}
	if cfg.RefreshTokenTTL > 0 {
		s.machineRefreshTTL = cfg.RefreshTokenTTL
	}
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

// MachineRefreshTokenTTL returns the configured opaque machine refresh token lifetime.
func (s *SessionIssuer) MachineRefreshTokenTTL() time.Duration {
	if s == nil {
		return 0
	}
	if s.machineRefreshTTL > 0 {
		return s.machineRefreshTTL
	}
	return s.refreshTTL
}

// MFAPendingTTL returns configured MFA challenge JWT lifetime.
func (s *SessionIssuer) MFAPendingTTL() time.Duration {
	if s == nil {
		return 0
	}
	return s.mfaPendingTTL
}

type sessionAccessClaims struct {
	Sub               string   `json:"sub"`
	Roles             []string `json:"roles"`
	OrgID             string   `json:"org_id,omitempty"`
	OrganizationID    string   `json:"organization_id,omitempty"`
	SiteID            string   `json:"site_id,omitempty"`
	MachineIDs        []string `json:"machine_ids,omitempty"`
	MachineID         string   `json:"machine_id,omitempty"`
	SessionID         string   `json:"session_id,omitempty"`
	ActorType         string   `json:"actor_type,omitempty"`
	AccountStatus     string   `json:"account_status,omitempty"`
	Typ               string   `json:"typ,omitempty"`
	TokenVersion      int64    `json:"token_version,omitempty"`
	CredentialVersion int64    `json:"credential_version,omitempty"`
	Scopes            []string `json:"scopes,omitempty"`
	Iss               string   `json:"iss,omitempty"`
	Aud               string   `json:"aud,omitempty"`
	Iat               int64    `json:"iat"`
	Exp               int64    `json:"exp"`
	TokenUse          string   `json:"token_use,omitempty"`
	Jti               string   `json:"jti,omitempty"`
	Nbf               int64    `json:"nbf,omitempty"`
	MFAEnrollment     bool     `json:"mfa_enrollment,omitempty"`
}

// MFAPendingClaims is returned after validating a MFA challenge JWT.
type MFAPendingClaims struct {
	AccountID      uuid.UUID
	OrganizationID uuid.UUID
	Roles          []string
	AccountStatus  string
	MFAEnrollment  bool
}

// IssueAccessJWT returns a signed HS256 JWT string for the given account subject and tenant claims.
// accountStatus should match platform_auth_accounts.status (e.g. active, disabled); empty defaults to active in the JWT.
func (s *SessionIssuer) IssueAccessJWT(accountID uuid.UUID, organizationID uuid.UUID, roles []string, accountStatus string) (token string, expiresAt time.Time, err error) {
	if s == nil {
		return "", time.Time{}, fmt.Errorf("auth: nil SessionIssuer")
	}
	if accountID == uuid.Nil {
		return "", time.Time{}, fmt.Errorf("auth: nil account id")
	}
	now := time.Now().UTC()
	expiresAt = now.Add(s.accessTokenTTL)
	st := strings.TrimSpace(accountStatus)
	if st == "" {
		st = "active"
	}
	claims := sessionAccessClaims{
		Sub:           accountID.String(),
		Roles:         roles,
		OrgID:         organizationID.String(),
		AccountStatus: st,
		Iss:           s.issuer,
		Aud:           s.audience,
		Iat:           now.Unix(),
		Exp:           expiresAt.Unix(),
		Nbf:           now.Unix(),
		TokenUse:      TokenUseInteractiveAccess,
		Jti:           uuid.NewString(),
	}
	raw, err := SignHS256JWT(s.secret, claims)
	if err != nil {
		return "", time.Time{}, err
	}
	return raw, expiresAt, nil
}

// IssueMFAPendingJWT issues a short-lived HS256 JWT after successful password verification when MFA must be satisfied or enrolled.
func (s *SessionIssuer) IssueMFAPendingJWT(accountID uuid.UUID, organizationID uuid.UUID, roles []string, accountStatus string, mfaEnrollment bool) (token string, expiresAt time.Time, err error) {
	if s == nil {
		return "", time.Time{}, fmt.Errorf("auth: nil SessionIssuer")
	}
	if accountID == uuid.Nil {
		return "", time.Time{}, fmt.Errorf("auth: nil account id")
	}
	now := time.Now().UTC()
	expiresAt = now.Add(s.mfaPendingTTL)
	st := strings.TrimSpace(accountStatus)
	if st == "" {
		st = "active"
	}
	claims := sessionAccessClaims{
		Sub:           accountID.String(),
		Roles:         roles,
		OrgID:         organizationID.String(),
		AccountStatus: st,
		Iss:           s.issuer,
		Aud:           s.audience,
		Iat:           now.Unix(),
		Exp:           expiresAt.Unix(),
		Nbf:           now.Unix(),
		TokenUse:      TokenUseMFAPending,
		Jti:           uuid.NewString(),
		MFAEnrollment: mfaEnrollment,
	}
	raw, err := SignHS256JWT(s.secret, claims)
	if err != nil {
		return "", time.Time{}, err
	}
	return raw, expiresAt, nil
}

// ParseMFAPendingJWT validates signature/expiry and returns MFA pending claims (interactive HS256 path).
func (s *SessionIssuer) ParseMFAPendingJWT(raw string, leeway time.Duration) (MFAPendingClaims, error) {
	if s == nil {
		return MFAPendingClaims{}, fmt.Errorf("auth: nil SessionIssuer")
	}
	if leeway <= 0 {
		leeway = DefaultClockLeeway
	}
	payload, err := verifyHS256JWT(s.secret, raw)
	if err != nil {
		return MFAPendingClaims{}, ErrUnauthenticated
	}
	var c sessionAccessClaims
	if err := json.Unmarshal(payload, &c); err != nil {
		return MFAPendingClaims{}, ErrUnauthenticated
	}
	if strings.ToLower(strings.TrimSpace(c.TokenUse)) != TokenUseMFAPending {
		return MFAPendingClaims{}, ErrUnauthenticated
	}
	if c.Exp > 0 {
		expAt := time.Unix(c.Exp, 0).UTC()
		if time.Now().UTC().After(expAt.Add(leeway)) {
			return MFAPendingClaims{}, ErrUnauthenticated
		}
	}
	aid, err := uuid.Parse(strings.TrimSpace(c.Sub))
	if err != nil || aid == uuid.Nil {
		return MFAPendingClaims{}, ErrUnauthenticated
	}
	orgID, err := uuid.Parse(strings.TrimSpace(c.OrgID))
	if err != nil || orgID == uuid.Nil {
		return MFAPendingClaims{}, ErrUnauthenticated
	}
	return MFAPendingClaims{
		AccountID:      aid,
		OrganizationID: orgID,
		Roles:          append([]string(nil), c.Roles...),
		AccountStatus:  strings.TrimSpace(c.AccountStatus),
		MFAEnrollment:  c.MFAEnrollment,
	}, nil
}

// IssueMachineAccessJWT issues a runtime JWT scoped to exactly one machine (kiosk / device bridge).
// credentialVersion must match machines.credential_version at issue time (rotation / revocation).
// sessionID binds the access token to machine_sessions when non-nil.
func (s *SessionIssuer) IssueMachineAccessJWT(machineID, organizationID, siteID uuid.UUID, credentialVersion int64, sessionID uuid.UUID) (token string, expiresAt time.Time, err error) {
	if s == nil {
		return "", time.Time{}, fmt.Errorf("auth: nil SessionIssuer")
	}
	if machineID == uuid.Nil || organizationID == uuid.Nil {
		return "", time.Time{}, fmt.Errorf("auth: machine and organization ids are required")
	}
	now := time.Now().UTC()
	machineAccessTTL := s.machineAccessTTL
	if machineAccessTTL <= 0 {
		machineAccessTTL = s.accessTokenTTL
	}
	expiresAt = now.Add(machineAccessTTL)
	machineIssuer := strings.TrimSpace(s.machineIssuer)
	if machineIssuer == "" {
		machineIssuer = strings.TrimSpace(s.issuer)
	}
	machineAudience := strings.TrimSpace(s.machineAudience)
	if machineAudience == "" {
		machineAudience = AudienceMachineGRPC
	}
	scopes := append([]string{}, DefaultMachineAccessScopes...)
	claims := sessionAccessClaims{
		Sub:               fmt.Sprintf("machine:%s", machineID.String()),
		Roles:             []string{RoleMachine},
		OrgID:             organizationID.String(),
		OrganizationID:    organizationID.String(),
		ActorType:         ActorTypeService,
		MachineIDs:        []string{machineID.String()},
		MachineID:         machineID.String(),
		Typ:               JWTClaimTypeMachine,
		TokenVersion:      credentialVersion,
		CredentialVersion: credentialVersion,
		Scopes:            scopes,
		Iss:               machineIssuer,
		Aud:               machineAudience,
		Iat:               now.Unix(),
		Exp:               expiresAt.Unix(),
		Nbf:               now.Unix(),
		TokenUse:          TokenUseMachineAccess,
		Jti:               uuid.NewString(),
	}
	if siteID != uuid.Nil {
		claims.SiteID = siteID.String()
	}
	if sessionID != uuid.Nil {
		claims.SessionID = sessionID.String()
	}
	raw, err := SignHS256JWT(s.secret, claims)
	if err != nil {
		return "", time.Time{}, err
	}
	return raw, expiresAt, nil
}
