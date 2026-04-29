package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/avf/avf-vending-api/internal/config"
)

func httpAccessTokenAudiences(cfg config.HTTPAuthConfig) []string {
	var out []string
	if a := strings.TrimSpace(cfg.ExpectedAudience); a != "" {
		out = append(out, a)
	}
	return out
}

// Well-known HTTP_AUTH_MODE values (case-insensitive in config).
const (
	HTTPAuthModeHS256      = "hs256"
	HTTPAuthModeRS256PEM   = "rs256_pem"
	HTTPAuthModeRS256JWKS  = "rs256_jwks"
	HTTPAuthModeEd25519PEM = "ed25519_pem"
	HTTPAuthModeJWTJWKS    = "jwt_jwks" // RS256 + EdDSA (Ed25519) keys from one JWKS document
)

// NewAccessTokenValidator builds the validator described by cfg (mode, keys, JWKS URL, iss/aud).
func NewAccessTokenValidator(cfg config.HTTPAuthConfig) (AccessTokenValidator, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	switch mode {
	case "", HTTPAuthModeHS256:
		return newHS256Validator(cfg.JWTSecret, cfg.JWTSecretPrevious, cfg.JWTLeeway), nil
	case HTTPAuthModeRS256PEM:
		if len(cfg.RSAPublicKeyPEM) == 0 {
			return nil, fmt.Errorf("auth: HTTP_AUTH_MODE=rs256_pem requires HTTP_AUTH_JWT_RSA_PUBLIC_KEY or HTTP_AUTH_JWT_RSA_PUBLIC_KEY_FILE")
		}
		return newRS256PEMValidator(cfg.RSAPublicKeyPEM, cfg.ExpectedIssuer, httpAccessTokenAudiences(cfg), cfg.JWTLeeway)
	case HTTPAuthModeRS256JWKS:
		if strings.TrimSpace(cfg.JWKSURL) == "" {
			return nil, fmt.Errorf("auth: HTTP_AUTH_MODE=rs256_jwks requires HTTP_AUTH_JWT_JWKS_URL")
		}
		return newRS256JWKSCachedValidator(cfg.JWKSURL, cfg.JWKSCacheTTL, cfg.ExpectedIssuer, httpAccessTokenAudiences(cfg), cfg.JWTLeeway), nil
	case HTTPAuthModeEd25519PEM:
		if len(cfg.Ed25519PublicKeyPEM) == 0 {
			return nil, fmt.Errorf("auth: HTTP_AUTH_MODE=ed25519_pem requires HTTP_AUTH_JWT_ED25519_PUBLIC_KEY or HTTP_AUTH_JWT_ED25519_PUBLIC_KEY_FILE")
		}
		return newEd25519PEMValidator(cfg.Ed25519PublicKeyPEM, cfg.ExpectedIssuer, httpAccessTokenAudiences(cfg), cfg.JWTLeeway)
	case HTTPAuthModeJWTJWKS:
		if strings.TrimSpace(cfg.JWKSURL) == "" {
			return nil, fmt.Errorf("auth: HTTP_AUTH_MODE=jwt_jwks requires HTTP_AUTH_JWT_JWKS_URL")
		}
		return newJWTJWKSCachedValidator(cfg.JWKSURL, cfg.JWKSCacheTTL, cfg.ExpectedIssuer, httpAccessTokenAudiences(cfg), cfg.JWTLeeway), nil
	default:
		return nil, fmt.Errorf("auth: unknown HTTP_AUTH_MODE %q", cfg.Mode)
	}
}

// MaybeWarmJWKS calls WarmJWKS on nested JWKS-backed validators (startup connectivity check).
func MaybeWarmJWKS(ctx context.Context, v AccessTokenValidator) error {
	if v == nil {
		return nil
	}
	var firstErr error
	var walk func(AccessTokenValidator)
	seen := map[AccessTokenValidator]struct{}{}
	walk = func(cur AccessTokenValidator) {
		if cur == nil {
			return
		}
		if _, ok := seen[cur]; ok {
			return
		}
		seen[cur] = struct{}{}
		switch t := cur.(type) {
		case *RevocationValidator:
			walk(t.Inner())
			return
		case chainedValidator:
			walk(t.primary)
			walk(t.secondary)
			return
		}
		if w, ok := cur.(interface{ WarmJWKS(context.Context) error }); ok {
			if err := w.WarmJWKS(ctx); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	walk(v)
	return firstErr
}
