package auth

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestValidateMachineAccessJWT_RoundTrip(t *testing.T) {
	t.Parallel()

	cfg := config.HTTPAuthConfig{
		Mode:            HTTPAuthModeHS256,
		JWTSecret:       bytes.Repeat([]byte("k"), 32),
		JWTLeeway:       30 * time.Second,
		AccessTokenTTL:  10 * time.Minute,
		RefreshTokenTTL: 720 * time.Hour,
	}
	issuer, err := NewSessionIssuerFromHTTPAuth(cfg)
	if err != nil {
		t.Fatal(err)
	}
	machineID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	siteID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	raw, _, err := issuer.IssueMachineAccessJWT(machineID, siteID, 7, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ValidateMachineAccessJWT(raw, MachineJWTSecretsFromHTTPAuth(cfg), cfg.JWTLeeway, AudienceMachineGRPC)
	if err != nil {
		t.Fatal(err)
	}
	if claims.MachineID != machineID {
		t.Fatalf("machine id=%v", claims.MachineID)
	}
	if claims.CredentialVersion != 7 {
		t.Fatalf("credential version=%d", claims.CredentialVersion)
	}
	if claims.TokenUse != TokenUseMachineAccess {
		t.Fatalf("token_use=%q", claims.TokenUse)
	}
}

func TestValidateMachineAccessJWT_RejectsUserAccessToken(t *testing.T) {
	t.Parallel()

	cfg := config.HTTPAuthConfig{
		Mode:            HTTPAuthModeHS256,
		JWTSecret:       bytes.Repeat([]byte("k"), 32),
		JWTLeeway:       30 * time.Second,
		AccessTokenTTL:  10 * time.Minute,
		RefreshTokenTTL: 720 * time.Hour,
	}
	issuer, err := NewSessionIssuerFromHTTPAuth(cfg)
	if err != nil {
		t.Fatal(err)
	}
	userTok, _, err := issuer.IssueAccessJWT(id.NewUUIDV7(), uuid.Nil, []string{RoleOrgAdmin}, "active")
	if err != nil {
		t.Fatal(err)
	}
	_, err = ValidateMachineAccessJWT(userTok, MachineJWTSecretsFromHTTPAuth(cfg), cfg.JWTLeeway, AudienceMachineGRPC)
	if err == nil {
		t.Fatal("expected error for user JWT")
	}
}

func TestValidateMachineAccessJWTWithConfig_EdDSA(t *testing.T) {
	t.Parallel()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	machineID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	raw := signMachineTestJWT(t, jwt.SigningMethodEdDSA, priv, "", machineID)

	claims, err := ValidateMachineAccessJWTWithConfig(context.Background(), raw, config.MachineJWTConfig{
		Mode:                   HTTPAuthModeEd25519PEM,
		Ed25519PublicKeyPEM:    pubPEM,
		JWTLeeway:              30 * time.Second,
		ExpectedAudience:       AudienceMachineGRPC,
		JWKSCacheTTL:           time.Minute,
		JWKSSkipStartupWarm:    true,
		AdditionalHS256Secrets: nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	if claims.MachineID != machineID {
		t.Fatalf("machine id=%v", claims.MachineID)
	}
}

func TestValidateMachineAccessJWTWithConfig_JWKSRejectsInvalidKID(t *testing.T) {
	t.Parallel()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	jwks := `{"keys":[{"kty":"OKP","crv":"Ed25519","kid":"good","use":"sig","x":"` + base64.RawURLEncoding.EncodeToString(pub) + `"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(jwks))
	}))
	t.Cleanup(srv.Close)

	raw := signMachineTestJWT(t, jwt.SigningMethodEdDSA, priv, "bad", uuid.MustParse("33333333-3333-3333-3333-333333333333"))
	_, err = ValidateMachineAccessJWTWithConfig(context.Background(), raw, config.MachineJWTConfig{
		Mode:             HTTPAuthModeJWTJWKS,
		JWKSURL:          srv.URL,
		JWKSCacheTTL:     time.Minute,
		JWTLeeway:        30 * time.Second,
		ExpectedAudience: AudienceMachineGRPC,
	})
	if err == nil {
		t.Fatal("expected invalid kid to be rejected")
	}
}

func TestIssueMachineAccessJWT_IncludesCredentialVersionAndSessionID(t *testing.T) {
	t.Parallel()
	cfg := config.HTTPAuthConfig{
		Mode:            HTTPAuthModeHS256,
		JWTSecret:       bytes.Repeat([]byte("k"), 32),
		JWTLeeway:       30 * time.Second,
		AccessTokenTTL:  10 * time.Minute,
		RefreshTokenTTL: 720 * time.Hour,
	}
	issuer, err := NewSessionIssuerFromHTTPAuth(cfg)
	if err != nil {
		t.Fatal(err)
	}
	machineID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	siteID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	sessID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	raw, _, err := issuer.IssueMachineAccessJWT(machineID, siteID, 42, sessID)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ValidateMachineAccessJWT(raw, MachineJWTSecretsFromHTTPAuth(cfg), cfg.JWTLeeway, AudienceMachineGRPC)
	if err != nil {
		t.Fatal(err)
	}
	if claims.CredentialVersion != 42 {
		t.Fatalf("credential_version=%d", claims.CredentialVersion)
	}
	if claims.SessionID != sessID {
		t.Fatalf("session_id=%v want %v", claims.SessionID, sessID)
	}
}

func TestIssueMachineAccessJWT_LegacyNoSessionID(t *testing.T) {
	t.Parallel()
	cfg := config.HTTPAuthConfig{
		Mode:            HTTPAuthModeHS256,
		JWTSecret:       bytes.Repeat([]byte("k"), 32),
		JWTLeeway:       30 * time.Second,
		AccessTokenTTL:  10 * time.Minute,
		RefreshTokenTTL: 720 * time.Hour,
	}
	issuer, err := NewSessionIssuerFromHTTPAuth(cfg)
	if err != nil {
		t.Fatal(err)
	}
	machineID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	siteID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	raw, _, err := issuer.IssueMachineAccessJWT(machineID, siteID, 3, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ValidateMachineAccessJWT(raw, MachineJWTSecretsFromHTTPAuth(cfg), cfg.JWTLeeway, AudienceMachineGRPC)
	if err != nil {
		t.Fatal(err)
	}
	if claims.SessionID != uuid.Nil {
		t.Fatalf("expected empty session, got %v", claims.SessionID)
	}
	if claims.CredentialVersion != 3 {
		t.Fatalf("credential_version=%d", claims.CredentialVersion)
	}
}

func signMachineTestJWT(t *testing.T, method jwt.SigningMethod, key any, kid string, machineID uuid.UUID) string {
	t.Helper()
	now := time.Now().UTC()
	tok := jwt.NewWithClaims(method, jwt.MapClaims{
		"sub":           "machine:" + machineID.String(),
		"typ":           JWTClaimTypeMachine,
		"aud":           AudienceMachineGRPC,
		"roles":         []string{RoleMachine},
		"machine_id":    machineID.String(),
		"token_version": float64(7),
		"scopes":        DefaultMachineAccessScopes,
		"iat":           now.Unix(),
		"nbf":           now.Unix(),
		"exp":           now.Add(10 * time.Minute).Unix(),
		"token_use":     TokenUseMachineAccess,
	})
	if kid != "" {
		tok.Header["kid"] = kid
	}
	raw, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
