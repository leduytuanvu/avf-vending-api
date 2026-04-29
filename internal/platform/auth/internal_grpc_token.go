package auth

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AudienceInternalGRPC is the required JWT "aud" for internal split-ready gRPC (operator-issued service tokens only).
const AudienceInternalGRPC = "avf-internal-grpc"

// JWTClaimTypeInternalService is the required JWT "typ" for internal gRPC bearer tokens.
const JWTClaimTypeInternalService = "service"

type internalServiceAccessClaims struct {
	Sub   string   `json:"sub"`
	Roles []string `json:"roles"`
	OrgID string   `json:"org_id,omitempty"`
	Typ   string   `json:"typ"`
	Aud   string   `json:"aud"`
	Iss   string   `json:"iss,omitempty"`
	Iat   int64    `json:"iat"`
	Exp   int64    `json:"exp"`
}

// IssueInternalServiceAccessJWT signs a short-lived HS256 JWT for internal gRPC callers (reconcilers, workers in-process today; separate processes later).
// organizationID may be uuid.Nil for platform-wide service subjects.
func IssueInternalServiceAccessJWT(secret []byte, subject string, organizationID uuid.UUID, ttl time.Duration, issuer string) (token string, expiresAt time.Time, err error) {
	if len(secret) == 0 {
		return "", time.Time{}, fmt.Errorf("auth: nil secret")
	}
	sub := strings.TrimSpace(subject)
	if sub == "" {
		return "", time.Time{}, fmt.Errorf("auth: internal service subject required")
	}
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	now := time.Now().UTC()
	expiresAt = now.Add(ttl)
	roles := []string{RoleService}
	claims := internalServiceAccessClaims{
		Sub:   sub,
		Roles: roles,
		Typ:   JWTClaimTypeInternalService,
		Aud:   AudienceInternalGRPC,
		Iss:   strings.TrimSpace(issuer),
		Iat:   now.Unix(),
		Exp:   expiresAt.Unix(),
	}
	if organizationID != uuid.Nil {
		claims.OrgID = organizationID.String()
	}
	raw, err := SignHS256JWT(secret, claims)
	if err != nil {
		return "", time.Time{}, err
	}
	return raw, expiresAt, nil
}

// ValidateInternalServiceAccessJWT verifies HS256 and enforces aud=avf-internal-grpc, typ=service, and role service.
func ValidateInternalServiceAccessJWT(raw string, secrets [][]byte, leeway time.Duration) (Principal, error) {
	if leeway <= 0 {
		leeway = DefaultClockLeeway
	}
	var lastErr error
	for _, sec := range secrets {
		if len(sec) == 0 {
			continue
		}
		payload, err := verifyHS256JWT(sec, raw)
		if err != nil {
			lastErr = err
			continue
		}
		p, err := PrincipalFromJWTPayloadJSON(payload, leeway)
		if err != nil {
			lastErr = err
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(p.JWTAudience), AudienceInternalGRPC) {
			return Principal{}, ErrUnauthenticated
		}
		if !strings.EqualFold(strings.TrimSpace(p.JWTType), JWTClaimTypeInternalService) {
			return Principal{}, ErrUnauthenticated
		}
		if !p.HasRole(RoleService) {
			return Principal{}, ErrUnauthenticated
		}
		return p, nil
	}
	if lastErr == nil {
		lastErr = ErrMisconfigured
	}
	if lastErr == ErrMisconfigured {
		return Principal{}, ErrMisconfigured
	}
	return Principal{}, ErrUnauthenticated
}
