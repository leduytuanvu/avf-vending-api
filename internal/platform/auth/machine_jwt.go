package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// AudienceMachineGRPC is the required JWT "aud" for machine runtime access tokens used on gRPC.
const AudienceMachineGRPC = "avf-machine-grpc"

// JWTClaimTypeMachine is the required JWT "typ" claim for machine runtime access tokens.
const JWTClaimTypeMachine = "machine"

const (
	TokenUseMachineAccess  = "machine_access"
	TokenUseMachineRefresh = "machine_refresh"
)

// DefaultMachineAccessScopes are embedded in machine access JWTs unless overridden later.
var DefaultMachineAccessScopes = []string{
	"machine:bootstrap",
	"machine:commerce",
	"machine:telemetry",
	"machine:operator",
}

// MachineAccessClaims is the validated machine runtime JWT view (gRPC + HTTP HS256 path).
type MachineAccessClaims struct {
	MachineID         uuid.UUID
	OrganizationID    uuid.UUID
	SiteID            uuid.UUID
	SessionID         uuid.UUID
	CredentialVersion int64
	Scopes            []string
	Subject           string
	ExpiresAt         time.Time
	Audience          string
	Type              string
	TokenUse          string
	JTI               string
}

// ValidateMachineAccessJWT verifies an HS256 JWT using any of the given secrets and returns
// machine-only claims. User/session tokens are rejected (wrong typ/aud/token_use/roles).
func ValidateMachineAccessJWT(raw string, secrets [][]byte, leeway time.Duration, wantAudience string) (MachineAccessClaims, error) {
	return validateMachineAccessJWT(raw, secrets, leeway, wantAudience, "", true)
}

func validateMachineAccessJWT(raw string, secrets [][]byte, leeway time.Duration, wantAudience, wantIssuer string, requireAudience bool) (MachineAccessClaims, error) {
	if wantAudience == "" {
		wantAudience = AudienceMachineGRPC
	}
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
		claims, err := ParseMachineAccessClaimsFromPayload(payload, leeway, wantAudience, wantIssuer, requireAudience)
		if err != nil {
			lastErr = err
			continue
		}
		return claims, nil
	}
	if lastErr == nil {
		lastErr = ErrMisconfigured
	}
	return MachineAccessClaims{}, lastErr
}

// ValidateMachineAccessJWTWithConfig verifies machine JWTs using MACHINE_JWT_* configuration.
// HS256 remains the default for local development, while RS256/EdDSA/JWKS modes are rotation-ready for enterprise deployments.
func ValidateMachineAccessJWTWithConfig(ctx context.Context, raw string, cfg config.MachineJWTConfig) (MachineAccessClaims, error) {
	wantAudience := strings.TrimSpace(cfg.ExpectedAudience)
	if wantAudience == "" {
		wantAudience = AudienceMachineGRPC
	}
	requireAudience := cfg.RequireAudience || strings.TrimSpace(cfg.ExpectedAudience) != ""
	leeway := cfg.JWTLeeway
	if leeway <= 0 {
		leeway = DefaultClockLeeway
	}
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" {
		mode = "hs256"
	}
	switch mode {
	case "hs256":
		return validateMachineAccessJWT(raw, MachineJWTSecretsFromConfig(cfg), leeway, wantAudience, cfg.ExpectedIssuer, requireAudience)
	case "rs256_pem":
		pub, err := parseRSAPublicKeyPEM(cfg.RSAPublicKeyPEM)
		if err != nil {
			return MachineAccessClaims{}, ErrMisconfigured
		}
		return validateMachineAccessJWTAsymmetric(ctx, raw, leeway, wantAudience, cfg.ExpectedIssuer, requireAudience, []string{jwt.SigningMethodRS256.Alg()}, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok || t.Method.Alg() != jwt.SigningMethodRS256.Alg() {
				return nil, ErrUnauthenticated
			}
			return pub, nil
		})
	case "ed25519_pem":
		pub, err := parseEd25519PublicKeyPEM(cfg.Ed25519PublicKeyPEM)
		if err != nil {
			return MachineAccessClaims{}, ErrMisconfigured
		}
		return validateMachineAccessJWTAsymmetric(ctx, raw, leeway, wantAudience, cfg.ExpectedIssuer, requireAudience, []string{jwt.SigningMethodEdDSA.Alg()}, func(t *jwt.Token) (any, error) {
			if t.Method.Alg() != jwt.SigningMethodEdDSA.Alg() {
				return nil, ErrUnauthenticated
			}
			return pub, nil
		})
	case "rs256_jwks":
		v := newRS256JWKSCachedValidator(cfg.JWKSURL, cfg.JWKSCacheTTL, cfg.ExpectedIssuer, []string{wantAudience}, leeway)
		return validateMachineAccessJWTAsymmetric(ctx, raw, leeway, wantAudience, cfg.ExpectedIssuer, requireAudience, []string{jwt.SigningMethodRS256.Alg()}, func(t *jwt.Token) (any, error) {
			return v.publicKeyForKid(ctx, jwtHeaderKid(t))
		})
	case "jwt_jwks":
		v := newJWTJWKSCachedValidator(cfg.JWKSURL, cfg.JWKSCacheTTL, cfg.ExpectedIssuer, []string{wantAudience}, leeway)
		return validateMachineAccessJWTAsymmetric(ctx, raw, leeway, wantAudience, cfg.ExpectedIssuer, requireAudience, []string{jwt.SigningMethodRS256.Alg(), jwt.SigningMethodEdDSA.Alg()}, func(t *jwt.Token) (any, error) {
			return v.keyForToken(ctx, t)
		})
	default:
		return MachineAccessClaims{}, ErrMisconfigured
	}
}

func validateMachineAccessJWTAsymmetric(ctx context.Context, raw string, leeway time.Duration, wantAudience, issuer string, requireAudience bool, methods []string, keyFunc jwt.Keyfunc) (MachineAccessClaims, error) {
	raw = strings.TrimSpace(raw)
	opts := []jwt.ParserOption{jwt.WithLeeway(leeway), jwt.WithValidMethods(methods)}
	if strings.TrimSpace(issuer) != "" {
		opts = append(opts, jwt.WithIssuer(strings.TrimSpace(issuer)))
	}
	parser := jwt.NewParser(opts...)
	token, err := parser.ParseWithClaims(raw, jwt.MapClaims{}, keyFunc)
	if err != nil || token == nil {
		return MachineAccessClaims{}, ErrUnauthenticated
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return MachineAccessClaims{}, ErrUnauthenticated
	}
	if requireAudience && !jwtMapClaimsAudienceAllowed(claims, []string{wantAudience}) {
		return MachineAccessClaims{}, ErrUnauthenticated
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return MachineAccessClaims{}, ErrUnauthenticated
	}
	return ParseMachineAccessClaimsFromPayload(payload, leeway, wantAudience, issuer, requireAudience)
}

func jwtHeaderKid(t *jwt.Token) string {
	kid, _ := t.Header["kid"].(string)
	return strings.TrimSpace(kid)
}

// ParseMachineAccessClaimsFromPayload validates issuer, typ, aud, token_use,
// role=machine, organization_id, and machine_id binding.
func ParseMachineAccessClaimsFromPayload(payloadJSON []byte, leeway time.Duration, wantAudience, wantIssuer string, requireAudience bool) (MachineAccessClaims, error) {
	if wantAudience == "" {
		wantAudience = AudienceMachineGRPC
	}
	var m struct {
		Sub               string   `json:"sub"`
		Typ               string   `json:"typ"`
		Iss               string   `json:"iss"`
		Aud               any      `json:"aud"`
		Roles             []string `json:"roles"`
		OrgID             string   `json:"org_id"`
		OrganizationID    string   `json:"organization_id"`
		SiteID            string   `json:"site_id"`
		SessionID         string   `json:"session_id"`
		MachineIDs        []string `json:"machine_ids"`
		MachineID         string   `json:"machine_id"`
		TokenVersion      int64    `json:"token_version"`
		CredentialVersion int64    `json:"credential_version"`
		Scopes            []string `json:"scopes"`
		Jti               string   `json:"jti"`
		TokenUse          string   `json:"token_use"`
		Exp               int64    `json:"exp"`
		Iat               int64    `json:"iat"`
		Nbf               int64    `json:"nbf"`
	}
	if err := json.Unmarshal(payloadJSON, &m); err != nil {
		return MachineAccessClaims{}, ErrUnauthenticated
	}
	now := time.Now().UTC()
	if strings.TrimSpace(wantIssuer) != "" && strings.TrimSpace(m.Iss) != strings.TrimSpace(wantIssuer) {
		return MachineAccessClaims{}, ErrUnauthenticated
	}
	if m.Exp <= 0 || m.Iat <= 0 {
		return MachineAccessClaims{}, ErrUnauthenticated
	}
	expAt := time.Unix(m.Exp, 0).UTC()
	if now.After(expAt.Add(leeway)) {
		return MachineAccessClaims{}, ErrUnauthenticated
	}
	iatAt := time.Unix(m.Iat, 0).UTC()
	if now.Add(leeway).Before(iatAt) {
		return MachineAccessClaims{}, ErrUnauthenticated
	}
	if m.Nbf > 0 {
		nbfAt := time.Unix(m.Nbf, 0).UTC()
		if now.Add(leeway).Before(nbfAt) {
			return MachineAccessClaims{}, ErrUnauthenticated
		}
	}
	if !strings.EqualFold(strings.TrimSpace(m.Typ), JWTClaimTypeMachine) {
		return MachineAccessClaims{}, ErrUnauthenticated
	}
	audStr := normalizeJWTAudienceClaim(m.Aud)
	if requireAudience && (audStr == "" || !strings.EqualFold(audStr, wantAudience)) {
		return MachineAccessClaims{}, ErrUnauthenticated
	}
	if !strings.EqualFold(strings.TrimSpace(m.TokenUse), TokenUseMachineAccess) {
		return MachineAccessClaims{}, ErrUnauthenticated
	}
	hasMachineRole := false
	for _, r := range m.Roles {
		if strings.EqualFold(strings.TrimSpace(r), RoleMachine) {
			hasMachineRole = true
			break
		}
	}
	if !hasMachineRole {
		return MachineAccessClaims{}, ErrUnauthenticated
	}
	machineID, err := parseMachineSubject(m.Sub, m.MachineID, m.MachineIDs)
	if err != nil {
		return MachineAccessClaims{}, ErrUnauthenticated
	}
	orgRaw := strings.TrimSpace(m.OrganizationID)
	if orgRaw == "" {
		orgRaw = strings.TrimSpace(m.OrgID)
	}
	orgID, err := uuid.Parse(orgRaw)
	if err != nil || orgID == uuid.Nil {
		return MachineAccessClaims{}, ErrUnauthenticated
	}
	var siteID uuid.UUID
	if sid := strings.TrimSpace(m.SiteID); sid != "" {
		siteID, _ = uuid.Parse(sid)
	}
	var sessID uuid.UUID
	if s := strings.TrimSpace(m.SessionID); s != "" {
		sessID, _ = uuid.Parse(s)
	}
	credVer := m.CredentialVersion
	if credVer == 0 {
		credVer = m.TokenVersion
	}
	return MachineAccessClaims{
		MachineID:         machineID,
		OrganizationID:    orgID,
		SiteID:            siteID,
		SessionID:         sessID,
		CredentialVersion: credVer,
		Scopes:            m.Scopes,
		Subject:           strings.TrimSpace(m.Sub),
		ExpiresAt:         time.Unix(m.Exp, 0).UTC(),
		Audience:          audStr,
		Type:              strings.TrimSpace(m.Typ),
		TokenUse:          strings.TrimSpace(m.TokenUse),
		JTI:               strings.TrimSpace(m.Jti),
	}, nil
}

func parseMachineSubject(sub, machineIDClaim string, machineIDs []string) (uuid.UUID, error) {
	sub = strings.TrimSpace(sub)
	if strings.HasPrefix(strings.ToLower(sub), "machine:") {
		id, err := uuid.Parse(strings.TrimSpace(sub[len("machine:"):]))
		if err != nil || id == uuid.Nil {
			return uuid.Nil, fmt.Errorf("bad sub")
		}
		return id, nil
	}
	if machineIDClaim != "" {
		id, err := uuid.Parse(strings.TrimSpace(machineIDClaim))
		if err != nil || id == uuid.Nil {
			return uuid.Nil, fmt.Errorf("bad machine_id")
		}
		return id, nil
	}
	for _, s := range machineIDs {
		id, err := uuid.Parse(strings.TrimSpace(s))
		if err == nil && id != uuid.Nil {
			return id, nil
		}
	}
	return uuid.Nil, fmt.Errorf("no machine id")
}

func normalizeJWTAudienceClaim(aud any) string {
	switch v := aud.(type) {
	case string:
		return strings.TrimSpace(v)
	case []any:
		if len(v) > 0 {
			if s, ok := v[0].(string); ok {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func MachineJWTSecretsFromConfig(cfg config.MachineJWTConfig) [][]byte {
	out := [][]byte{}
	if sec := TrimSecret(cfg.JWTSecret); len(sec) > 0 {
		out = append(out, sec)
	}
	if sec := TrimSecret(cfg.JWTSecretPrevious); len(sec) > 0 {
		out = append(out, sec)
	}
	for _, sec := range cfg.AdditionalHS256Secrets {
		if trimmed := TrimSecret(sec); len(trimmed) > 0 {
			out = append(out, trimmed)
		}
	}
	return dedupeSecretSlices(out)
}

// MachineJWTSecretsFromHTTPAuth returns HS256 signing secrets used for machine JWTs (same material as SessionIssuer).
func MachineJWTSecretsFromHTTPAuth(cfg config.HTTPAuthConfig) [][]byte {
	var out [][]byte
	if sec := TrimSecret(cfg.LoginJWTSecret); len(sec) > 0 {
		out = append(out, sec)
	}
	if sec := TrimSecret(cfg.JWTSecret); len(sec) > 0 {
		out = append(out, sec)
	}
	if sec := TrimSecret(cfg.JWTSecretPrevious); len(sec) > 0 {
		out = append(out, sec)
	}
	return dedupeSecretSlices(out)
}

func dedupeSecretSlices(in [][]byte) [][]byte {
	seen := make(map[string]struct{})
	var out [][]byte
	for _, b := range in {
		if len(b) == 0 {
			continue
		}
		k := string(b)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, b)
	}
	return out
}
